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
)

// TestCacheMiddleware tests the basic functionality of cache middleware
func TestCacheMiddleware(t *testing.T) {
	// Create test server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"message": "test", "count": %d}`, callCount)
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
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	defer resp1.Close()

	if callCount != 1 {
		t.Errorf("Expected server to be called once, got %d", callCount)
	}

	// Second request (should hit cache)
	resp2, err := client.Get("/test").Send(context.Background())
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	defer resp2.Close()

	if callCount != 1 {
		t.Errorf("Expected server to still be called once, got %d", callCount)
	}

	// Wait for cache to expire
	time.Sleep(6 * time.Second)

	// Third request (cache should be expired)
	resp3, err := client.Get("/test").Send(context.Background())
	if err != nil {
		t.Fatalf("Third request failed: %v", err)
	}
	defer resp3.Close()

	if callCount != 2 {
		t.Errorf("Expected server to be called twice, got %d", callCount)
	}
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
			if key != tt.expected {
				t.Errorf("Expected cache key %s, got %s", tt.expected, key)
			}
		})
	}
}

// TestMemoryCache tests basic memory cache operations
func TestMemoryCache(t *testing.T) {
	cache := NewMemoryCache()

	// Test set and get
	cache.Set("key1", []byte("value1"), 2*time.Second)
	if value, ok := cache.Get("key1"); !ok || string(value) != "value1" {
		t.Error("Failed to get cached value")
	}

	// Test expiration
	cache.Set("key2", []byte("value2"), 1*time.Second)
	time.Sleep(2 * time.Second)
	if _, ok := cache.Get("key2"); ok {
		t.Error("Cache item should have expired")
	}

	// Test deletion
	cache.Set("key3", []byte("value3"), 10*time.Second)
	cache.Delete("key3")
	if _, ok := cache.Get("key3"); ok {
		t.Error("Cache item should have been deleted")
	}
}

// TestNonGetRequests tests handling of non-GET requests
func TestNonGetRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
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
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Close()

	// Verify nothing was stored in cache
	if _, ok := cache.Get("/test"); ok {
		t.Error("POST request should not be cached")
	}
}
