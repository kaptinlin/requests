package middlewares

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"

	"github.com/kaptinlin/requests"
)

func TestCacheMiddleware(t *testing.T) {
	t.Parallel()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"message": "test", "count": %d}`, callCount)
	}))
	defer server.Close()

	cache := NewMemoryCache()
	defer cache.Close()
	logger := requests.NewDefaultLogger(io.Discard, requests.LevelDebug)
	mw, err := NewCacheMiddleware(cache, time.Millisecond, logger)
	require.NoError(t, err)
	client, err := requests.New(
		requests.WithBaseURL(server.URL),
		requests.WithMiddleware(mw),
	)
	require.NoError(t, err)

	resp1, err := client.Get("/test").Send(context.Background())
	require.NoError(t, err)
	defer resp1.Close() //nolint:errcheck // test cleanup closes response body
	assert.Equal(t, 1, callCount)

	resp2, err := client.Get("/test").Send(context.Background())
	require.NoError(t, err)
	defer resp2.Close() //nolint:errcheck // test cleanup closes response body
	assert.Equal(t, 1, callCount)

	assert.Eventually(t, func() bool {
		resp3, err := client.Get("/test").Send(context.Background())
		if err != nil {
			return false
		}
		defer resp3.Close() //nolint:errcheck // test cleanup closes response body
		return callCount == 2
	}, time.Second, 10*time.Millisecond)
}

func TestNewCacheMiddlewareValidatesSetup(t *testing.T) {
	t.Parallel()

	cache := NewMemoryCache()
	defer cache.Close()
	logger := requests.NewDefaultLogger(io.Discard, requests.LevelDebug)

	mw, err := NewCacheMiddleware(cache, time.Second, logger)
	require.NoError(t, err)
	require.NotNil(t, mw)

	_, err = NewCacheMiddleware(nil, time.Second, logger)
	assert.ErrorIs(t, err, requests.ErrInvalidConfigValue)

	_, err = NewCacheMiddleware(cache, time.Second, nil)
	assert.ErrorIs(t, err, requests.ErrInvalidConfigValue)

	_, err = NewCacheMiddleware(cache, 0, logger)
	assert.ErrorIs(t, err, requests.ErrInvalidConfigValue)

	_, err = NewCacheMiddleware(cache, -time.Nanosecond, logger)
	assert.ErrorIs(t, err, requests.ErrInvalidConfigValue)
}

// TestCacheKeyGeneration tests cache key generation
func TestCacheKeyGeneration(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "origin and path",
			url:      "http://example.com/test",
			expected: "http://example.com/test",
		},
		{
			name:     "origin path and query",
			url:      "http://example.com/test?a=1&b=2",
			expected: "http://example.com/test?a=1&b=2",
		},
		{
			name:     "different scheme is distinct",
			url:      "https://example.com/test?a=1&b=2",
			expected: "https://example.com/test?a=1&b=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tt.url, nil)
			require.NoError(t, err)

			key := generateCacheKey(req)
			assert.Equal(t, tt.expected, key, "Unexpected cache key generated")
		})
	}
}

func TestCacheMiddlewareSeparatesOriginInCacheKey(t *testing.T) {
	callCount := 0
	cache := NewMemoryCache()
	defer cache.Close()
	logger := requests.NewDefaultLogger(io.Discard, requests.LevelDebug)
	mw, err := NewCacheMiddleware(cache, time.Minute, logger)
	require.NoError(t, err)

	handler := mw(func(req *http.Request) (*http.Response, error) {
		callCount++
		body := fmt.Sprintf("%s:%d", req.URL.Host, callCount)
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Body:       io.NopCloser(bytes.NewBufferString(body)),
			Request:    req,
		}, nil
	})

	send := func(rawURL string) string {
		t.Helper()

		req := httptest.NewRequest(http.MethodGet, rawURL, nil)
		resp, err := handler(req)
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Body.Close() })

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return string(body)
	}

	assert.Equal(t, "a.example.com:1", send("https://a.example.com/resource?id=1"))
	assert.Equal(t, "b.example.com:2", send("https://b.example.com/resource?id=1"))
	assert.Equal(t, "a.example.com:1", send("https://a.example.com/resource?id=1"))
	assert.Equal(t, 2, callCount)
}

func TestMemoryCache(t *testing.T) {
	t.Parallel()

	cache := NewMemoryCache()
	defer cache.Close()
	cache.Set("key1", []byte("value1"), time.Second)
	value, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", string(value))

	cache.Set("key2", []byte("value2"), -time.Nanosecond)
	_, ok = cache.Get("key2")
	assert.False(t, ok)

	cache.Set("key3", []byte("value3"), time.Second)
	cache.Delete("key3")
	_, ok = cache.Get("key3")
	assert.False(t, ok)
}

func TestMemoryCacheCloseIsIdempotent(t *testing.T) {
	t.Parallel()

	cache := NewMemoryCache()

	require.NotPanics(t, func() {
		cache.Close()
		cache.Close()
	})
}

func TestMemoryCacheOwnsStoredBytes(t *testing.T) {
	t.Parallel()

	cache := NewMemoryCache()
	defer cache.Close()

	original := []byte("value")
	cache.Set("key", original, time.Second)
	original[0] = 'X'

	value, ok := cache.Get("key")
	require.True(t, ok)
	assert.Equal(t, "value", string(value))

	value[0] = 'Y'

	again, ok := cache.Get("key")
	require.True(t, ok)
	assert.Equal(t, "value", string(again))
}

// TestNonGetRequests tests handling of non-GET requests
func TestNonGetRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "ok")
	}))
	defer server.Close()

	cache := NewMemoryCache()
	defer cache.Close()
	logger := requests.NewDefaultLogger(io.Discard, requests.LevelDebug)
	mw, err := NewCacheMiddleware(cache, 5*time.Second, logger)
	require.NoError(t, err)

	client, err := requests.New(
		requests.WithBaseURL(server.URL),
		requests.WithMiddleware(mw),
	)
	require.NoError(t, err)

	// Test POST request (should not be cached)
	resp, err := client.Post("/test").Send(context.Background())
	assert.NoError(t, err, "POST request failed")
	defer resp.Close() //nolint:errcheck // test cleanup closes response body

	// Verify nothing was stored in cache
	_, ok := cache.Get("/test")
	assert.False(t, ok, "POST request should not be cached")
}

func TestCacheResponseAndBuildResponseFromCache(t *testing.T) {
	resp := &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(bytes.NewBufferString("cached-body")),
	}

	data, err := cacheResponse(resp)
	assert.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "cached-body", string(body))

	rebuilt, err := buildResponseFromCache(data)
	assert.NoError(t, err)

	rebuiltBody, err := io.ReadAll(rebuilt.Body)
	assert.NoError(t, err)
	assert.Equal(t, "cached-body", string(rebuiltBody))
	assert.Equal(t, http.StatusOK, rebuilt.StatusCode)
	assert.Equal(t, "text/plain", rebuilt.Header.Get("Content-Type"))
}

func TestBuildResponseFromCacheInvalidData(t *testing.T) {
	_, err := buildResponseFromCache([]byte("{"))
	assert.Error(t, err)
}

func TestMemoryCacheCloseStopsCleaner(t *testing.T) {
	cache := &MemoryCache{
		data: make(map[string]*cacheItem),
		done: make(chan struct{}),
	}

	stopped := make(chan struct{})
	go func() {
		cache.cleanExpired()
		close(stopped)
	}()

	cache.Close()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("expected cleaner goroutine to stop after close")
	}
}
