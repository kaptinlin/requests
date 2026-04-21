package requests

import "net/http"

// MiddlewareHandlerFunc handles an HTTP request and returns an HTTP response.
type MiddlewareHandlerFunc func(req *http.Request) (*http.Response, error)

// Middleware wraps a MiddlewareHandlerFunc with additional behavior.
type Middleware func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc
