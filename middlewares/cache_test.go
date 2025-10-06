package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/kaptinlin/requests"
	"github.com/stretchr/testify/assert"
)

// TestCacheMiddleware tests the basic functionality of cache middleware
func TestCacheMiddleware(t *testing.T) {
	// Create test server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"message": "test", "count": %d}`, callCount)
	}))
	defer server.Close()

	// Create memory cache and logger
	cache := NewMemoryCache()
	logger := requests.NewDefaultLogger(os.Stdout, requests.LevelDebug)

	// Create client with cache middleware
	client := requests.Create(&requests.Config{
		BaseURL: server.URL,
		Middlewares: []requests.Middleware{
			CacheMiddleware(cache, 5*time.Second, logger),
		},
	})

	// First request (cache miss)
	resp1, err := client.Get("/test").Send(context.Background())
	assert.NoError(t, err, "First request failed")
	defer resp1.Close() //nolint:errcheck

	assert.Equal(t, 1, callCount, "Expected server to be called once")

	// Second request (should hit cache)
	resp2, err := client.Get("/test").Send(context.Background())
	assert.NoError(t, err, "Second request failed")
	defer resp2.Close() //nolint:errcheck

	assert.Equal(t, 1, callCount, "Expected server to still be called once")

	// Wait for cache to expire
	time.Sleep(6 * time.Second)

	// Third request (cache should be expired)
	resp3, err := client.Get("/test").Send(context.Background())
	assert.NoError(t, err, "Third request failed")
	defer resp3.Close() //nolint:errcheck

	assert.Equal(t, 2, callCount, "Expected server to be called twice")
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

// TestMemoryCache tests basic memory cache operations
func TestMemoryCache(t *testing.T) {
	cache := NewMemoryCache()

	// Test set and get
	cache.Set("key1", []byte("value1"), 2*time.Second)
	value, ok := cache.Get("key1")
	assert.True(t, ok, "Failed to get cached value")
	assert.Equal(t, "value1", string(value), "Unexpected cached value")

	// Test expiration
	cache.Set("key2", []byte("value2"), 1*time.Second)
	time.Sleep(2 * time.Second)
	_, ok = cache.Get("key2")
	assert.False(t, ok, "Cache item should have expired")

	// Test deletion
	cache.Set("key3", []byte("value3"), 10*time.Second)
	cache.Delete("key3")
	_, ok = cache.Get("key3")
	assert.False(t, ok, "Cache item should have been deleted")
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
