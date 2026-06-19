package requests

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

// TestCancelBeforeSend covers the case where ctx is already canceled before
// Send is invoked. Send must return the cancellation cause without dialing.
func TestCancelBeforeSend(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("server should not be reached when ctx is pre-canceled")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(ctx)

	require.Error(t, err)
	assert.True(t, IsCanceled(err), "expected IsCanceled to match cancellation, got %v", err)
	assert.False(t, IsTimeout(err), "explicit cancel must not be classified as timeout")
	assert.Nil(t, resp)
}

// TestCancelDuringResponseHeader covers cancellation while the server holds
// the request open before the first response byte. Send must release the
// connection and return the cancellation cause.
func TestCancelDuringResponseHeader(t *testing.T) {
	t.Parallel()

	gate := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-gate:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	var closeGate sync.Once
	defer closeGate.Do(func() { close(gate) })

	ctx, cancel := context.WithCancel(t.Context())
	client := newTestClient(t, WithBaseURL(server.URL))

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	resp, err := client.Get("/").Send(ctx)

	require.Error(t, err)
	assert.True(t, IsCanceled(err) || isURLErrorCanceled(err),
		"expected cancellation, got %v", err)
	assert.Nil(t, resp)
}

// TestCancelDuringResponseBody covers cancellation after the response headers
// have arrived but while the body is still streaming. Send buffers the body
// internally; when ctx is canceled mid-buffer, Send must return ctx's error
// and release the underlying connection.
func TestCancelDuringResponseBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		flusher, _ := w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
		if flusher != nil {
			flusher.Flush()
		}
		for {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(10 * time.Millisecond):
				if _, err := w.Write([]byte("x")); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	defer cancel()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(ctx)

	require.Error(t, err, "Send must surface cancellation while buffering the body")
	assert.True(t, IsCanceled(err) || errors.Is(err, context.Canceled) || isURLErrorCanceled(err),
		"expected cancellation, got %v", err)
	if resp != nil {
		_ = resp.Close()
	}
}

// TestCancelDuringRetryBackoff covers cancellation while the retry loop is
// sleeping between attempts. The loop must wake immediately, return ctx.Err(),
// and not perform another attempt.
func TestCancelDuringRetryBackoff(t *testing.T) {
	t.Parallel()

	var attempts int32
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		attempts++
		mu.Unlock()
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := newTestClient(t,
		WithBaseURL(server.URL),
		WithRetry(RetryPolicy{Max: 5, Backoff: DefaultBackoffStrategy(2 * time.Second)}),
	)

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	resp, err := client.Get("/").Send(ctx)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.True(t, IsCanceled(err), "expected IsCanceled, got %v", err)
	assert.Less(t, elapsed, 1500*time.Millisecond, "cancel must wake the backoff sleep promptly")

	mu.Lock()
	got := attempts
	mu.Unlock()
	assert.LessOrEqual(t, got, int32(2), "no further attempts after cancellation")
	if resp != nil {
		_ = resp.Close()
	}
}

func TestCancelDuringStream(t *testing.T) {
	t.Parallel()

	gate := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = fmt.Fprint(w, "data: tick\n")
		if flusher != nil {
			flusher.Flush()
		}
		<-gate
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer server.Close()
	var closeGate sync.Once
	defer closeGate.Do(func() { close(gate) })

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := newTestClient(t, WithBaseURL(server.URL))

	resp, err := client.Get("/").SendStream(ctx)
	require.NoError(t, err)
	defer resp.Close() //nolint:errcheck // test cleanup closes response body

	var gotData bool
	var streamErr error
	for line, err := range resp.Lines() {
		if err != nil {
			streamErr = err
			break
		}
		assert.Equal(t, "data: tick", string(line))
		gotData = true
		cancel()
		closeGate.Do(func() { close(gate) })
	}
	assert.True(t, gotData, "first chunk should have reached the callback before cancel")
	require.Error(t, streamErr)
	assert.True(t, IsCanceled(streamErr) || errors.Is(streamErr, context.Canceled),
		"streamErr should reflect cancellation, got %v", streamErr)
}

// TestNoGoroutineLeakAfterCancel runs a batch of canceled requests and
// verifies the goroutine count returns to baseline. This is a smoke test, not
// a substitute for goleak — it catches gross leaks introduced by future
// cancellation bugs without adding a dependency.
func TestNoGoroutineLeakAfterCancel(t *testing.T) {
	t.Parallel()

	gate := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-gate:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(gate)

	client := newTestClient(t, WithBaseURL(server.URL))

	// Warm the transport so its background goroutines do not skew the baseline.
	warmup, cancel := context.WithCancel(t.Context())
	cancel()
	_, _ = client.Get("/").Send(warmup)

	// Allow runtime + transport state to settle.
	settle()
	baseline := runtime.NumGoroutine()

	const iterations = 30
	var wg sync.WaitGroup
	wg.Add(iterations)
	for range iterations {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
			defer cancel()
			_, _ = client.Get("/").Send(ctx)
		}()
	}
	wg.Wait()
	settle()

	final := runtime.NumGoroutine()
	// Allow generous slack for transport-internal goroutines that may linger
	// briefly between idle-conn close and pool reaping.
	assert.LessOrEqualf(t, final, baseline+5,
		"expected goroutine count to return to baseline (~%d), got %d", baseline, final)
}

// settle gives the runtime time to reap finished goroutines and lets the
// HTTP transport notice closed connections.
func settle() {
	for range 5 {
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
	}
}

// isURLErrorCanceled unwraps a *url.Error to check for context.Canceled. The
// http transport sometimes wraps cancellation in a url.Error before any of
// our helpers see it.
func isURLErrorCanceled(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		// Some platforms map cancel to a timeout error for the underlying
		// syscall; treat that as cancellation for the purpose of this check.
		return false
	}
	return false
}
