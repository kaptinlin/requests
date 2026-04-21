package middlewares

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-json-experiment/json"
	"github.com/kaptinlin/requests"
)

// Duration stores a duration value as an int64.
type Duration int64

// Cacher is the interface for the cache.
type Cacher interface {
	// Get returns the cached value for key and reports whether it was found.
	Get(key string) ([]byte, bool)
	// Set stores value under key until ttl expires.
	Set(key string, value []byte, ttl time.Duration)
	// Delete removes the cached value for key.
	Delete(key string)
}

// CacheMiddleware creates a middleware that caches GET responses.
func CacheMiddleware(cache Cacher, ttl time.Duration, logger requests.Logger) requests.Middleware {
	return func(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				return next(req)
			}
			cacheKey := generateCacheKey(req)
			cachedData, ok := cache.Get(cacheKey)
			if ok {
				logger.Debugf("Cache hit: url=%s key=%s", req.URL.String(), cacheKey)
				return buildResponseFromCache(cachedData)
			}
			resp, err := next(req)
			if err != nil {
				return nil, err
			}

			if resp.StatusCode == http.StatusOK {
				if data, err := cacheResponse(resp); err == nil {
					cache.Set(cacheKey, data, ttl)
					logger.Debugf("Cached response: url=%s key=%s", req.URL.String(), cacheKey)
				}
			}
			return resp, nil
		}
	}
}

func cacheResponse(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	cacheData := &CachedResponse{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
	}

	return json.Marshal(cacheData)
}

// generateCacheKey creates a cache key from the request URL path and query string.
func generateCacheKey(req *http.Request) string {
	key := req.URL.Path
	if req.URL.RawQuery != "" {
		key += "?" + req.URL.RawQuery
	}
	return key
}

// buildResponseFromCache reconstructs an HTTP response from cached data.
func buildResponseFromCache(data []byte) (*http.Response, error) {
	var cached CachedResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, err
	}

	return &http.Response{
		Status:     cached.Status,
		StatusCode: cached.StatusCode,
		Header:     cached.Headers,
		Body:       io.NopCloser(bytes.NewReader(cached.Body)),
	}, nil
}

// CachedResponse represents a serializable HTTP response stored in the cache.
type CachedResponse struct {
	Status     string      // Status is the HTTP status text.
	StatusCode int         // StatusCode is the HTTP status code.
	Headers    http.Header // Headers contains the cached response headers.
	Body       []byte      // Body contains the cached response body.
}

// MemoryCache is an in-memory cache implementation with TTL-based expiration.
type MemoryCache struct {
	data  map[string]*cacheItem
	mutex sync.RWMutex
	done  chan struct{}
}

type cacheItem struct {
	value      []byte
	expiration time.Time
}

// NewMemoryCache creates a new MemoryCache and starts a background goroutine to clean expired items.
func NewMemoryCache() *MemoryCache {
	cache := &MemoryCache{
		data: make(map[string]*cacheItem),
		done: make(chan struct{}),
	}
	go cache.cleanExpired()
	return cache
}

// Close stops the background cleanup goroutine.
func (c *MemoryCache) Close() {
	close(c.done)
}

// Get retrieves a cache item by key, returning the value and whether it was found.
func (c *MemoryCache) Get(key string) ([]byte, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if item, exists := c.data[key]; exists {
		if time.Now().Before(item.expiration) {
			return item.value, true
		}
		// Item is expired; let the cleanup goroutine handle deletion.
	}
	return nil, false
}

// Set stores a value in the cache with the specified TTL.
func (c *MemoryCache) Set(key string, value []byte, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.data[key] = &cacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}
}

// Delete removes a cache item by key.
func (c *MemoryCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.data, key)
}

// cleanExpired periodically removes expired items from the cache.
func (c *MemoryCache) cleanExpired() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mutex.Lock()
			now := time.Now()
			for key, item := range c.data {
				if now.After(item.expiration) {
					delete(c.data, key)
				}
			}
			c.mutex.Unlock()
		case <-c.done:
			return
		}
	}
}
