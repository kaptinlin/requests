package middlewares

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/kaptinlin/requests"
)

type Duration int64

// Cacher is the interface for the cache
type Cacher interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration)
	Delete(key string)
}

// CacheMiddleware is the middleware for the cache
var CacheMiddleware = func(cache Cacher, ttl time.Duration, logger requests.Logger) requests.Middleware {
	return func(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			// If not GET request, skip cache
			if req.Method != http.MethodGet {
				return next(req)
			}
			// Generate cache key
			cacheKey := generateCacheKey(req)
			// Get cached data
			cachedData, ok := cache.Get(cacheKey)
			if ok {
				logger.Debugf("Cache hit", map[string]interface{}{
					"url": req.URL.String(),
					"key": cacheKey,
				})
				// Build response from cache
				return buildResponseFromCache(cachedData)
			}
			// Call next middleware
			resp, err := next(req)
			if err != nil {
				return nil, err
			}

			// Cache response if status code is 200
			if resp.StatusCode == http.StatusOK {
				if data, err := cacheResponse(resp); err == nil {
					// Cache response
					cache.Set(cacheKey, data, ttl)
					logger.Debugf("Cached response", map[string]interface{}{
						"url": req.URL.String(),
						"key": cacheKey,
					})
				}
			}
			// Return response
			return resp, nil
		}
	}
}

func cacheResponse(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Reset body
	resp.Body = io.NopCloser(bytes.NewReader(body))

	// Cache data
	cacheData := &CachedResponse{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       body,
	}

	return json.Marshal(cacheData)
}

// Generate cache key
func generateCacheKey(req *http.Request) string {
	// Generate cache key from request
	key := req.URL.Path
	if req.URL.RawQuery != "" {
		key += "?" + req.URL.RawQuery
	}
	return key
}

// Build response from cache
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

// CachedResponse
type CachedResponse struct {
	Status     string
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// MemoryCache
type MemoryCache struct {
	data  map[string]*cacheItem
	mutex sync.RWMutex
}

type cacheItem struct {
	value      []byte
	expiration time.Time
}

// NewMemoryCache
func NewMemoryCache() *MemoryCache {
	cache := &MemoryCache{
		data: make(map[string]*cacheItem),
	}
	// Clean expired items
	go cache.cleanExpired()
	return cache
}

// Get cache item
func (c *MemoryCache) Get(key string) ([]byte, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if item, exists := c.data[key]; exists {
		if time.Now().Before(item.expiration) {
			return item.value, true
		}
		// Expired, delete
		delete(c.data, key)
	}
	return nil, false
}

// Set cache item
func (c *MemoryCache) Set(key string, value []byte, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.data[key] = &cacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}
}

// Delete cache item
func (c *MemoryCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.data, key)
}

// Clean expired items
func (c *MemoryCache) cleanExpired() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		c.mutex.Lock()
		now := time.Now()
		for key, item := range c.data {
			if now.After(item.expiration) {
				delete(c.data, key)
			}
		}
		c.mutex.Unlock()
	}
}
