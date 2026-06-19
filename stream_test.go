package requests

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

func TestStream(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := range 3 {
			_, _ = fmt.Fprintf(w, "data: Message %d\n", i)
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()

	dataReceived := make([]string, 0, 3)

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").SendStream(t.Context())
	require.NoError(t, err)
	defer resp.Close() //nolint:errcheck // test cleanup closes response body

	for data, err := range resp.Lines() {
		require.NoError(t, err)
		dataReceived = append(dataReceived, string(data))
		assert.Contains(t, string(data), "data: Message")
	}
	assert.Equal(t, 3, len(dataReceived))
}

func TestStreamResponseRawAndAccessors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Stream", "yes")
		w.WriteHeader(http.StatusAccepted)
		_, _ = fmt.Fprint(w, "hello\n")
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/events").SendStream(t.Context())
	require.NoError(t, err)
	defer resp.Close() //nolint:errcheck // test cleanup closes response body

	assert.Equal(t, http.StatusAccepted, resp.StatusCode())
	assert.Equal(t, "202 Accepted", resp.Status())
	assert.Equal(t, "yes", resp.Header().Get("X-Stream"))
	assert.Equal(t, "/events", resp.URL().Path)
	assert.Equal(t, 1, resp.Attempts())
	assert.Positive(t, resp.Elapsed())
	assert.Equal(t, http.StatusAccepted, resp.Raw().StatusCode)

	body, err := io.ReadAll(resp.Body())
	require.NoError(t, err)
	assert.Equal(t, "hello\n", string(body))
}

func TestStreamResponseCloseReleasesTimeoutContext(t *testing.T) {
	t.Parallel()

	ctxDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(ctxDone)
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Timeout(time.Second).SendStream(t.Context())
	require.NoError(t, err)

	require.NoError(t, resp.Close())
	require.NoError(t, resp.Close())

	select {
	case <-ctxDone:
	case <-time.After(time.Second):
		t.Fatal("stream close did not release request context")
	}
}

func TestStreamResponseLinesReportsScannerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxStreamBufferSize+1))
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").SendStream(t.Context())
	require.NoError(t, err)
	defer resp.Close() //nolint:errcheck // test cleanup closes response body

	var gotErr error
	for _, err := range resp.Lines() {
		gotErr = err
	}
	require.Error(t, gotErr)
}
