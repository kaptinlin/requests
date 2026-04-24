package middlewares

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/kaptinlin/requests"
	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
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
	logger := requests.NewDefaultLogger(os.Stdout, requests.LevelDebug)
	client := requests.Create(&requests.Config{
		BaseURL: server.URL,
		Middlewares: []requests.Middleware{
			CacheMiddleware(cache, time.Millisecond, logger),
		},
	})

	resp1, err := client.Get("/test").Send(context.Background())
	require.NoError(t, err)
	defer resp1.Close() //nolint:errcheck
	assert.Equal(t, 1, callCount)

	resp2, err := client.Get("/test").Send(context.Background())
	require.NoError(t, err)
	defer resp2.Close() //nolint:errcheck
	assert.Equal(t, 1, callCount)

	assert.Eventually(t, func() bool {
		resp3, err := client.Get("/test").Send(context.Background())
		if err != nil {
			return false
		}
		defer resp3.Close() //nolint:errcheck
		return callCount == 2
	}, time.Second, 10*time.Millisecond)
}

// TestCacheKeyGeneration tests cache key generation
func TestCacheKeyGeneration(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		query    string
		expected string
	}{
		{
			name:     "Simple path",
			path:     "/test",
			query:    "",
			expected: "/test",
		},
		{
			name:     "Path with query",
			path:     "/test",
			query:    "a=1&b=2",
			expected: "/test?a=1&b=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com"+tt.path, nil)
			if tt.query != "" {
				req.URL.RawQuery = tt.query
			}

			key := generateCacheKey(req)
			assert.Equal(t, tt.expected, key, "Unexpected cache key generated")
		})
	}
}

func TestMemoryCache(t *testing.T) {
	t.Parallel()

	cache := NewMemoryCache()
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

// TestNonGetRequests tests handling of non-GET requests
func TestNonGetRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "ok")
	}))
	defer server.Close()

	cache := NewMemoryCache()
	logger := requests.NewDefaultLogger(os.Stdout, requests.LevelDebug)

	client := requests.Create(&requests.Config{
		BaseURL: server.URL,
		Middlewares: []requests.Middleware{
			CacheMiddleware(cache, 5*time.Second, logger),
		},
	})

	// Test POST request (should not be cached)
	resp, err := client.Post("/test").Send(context.Background())
	assert.NoError(t, err, "POST request failed")
	defer resp.Close() //nolint:errcheck

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
