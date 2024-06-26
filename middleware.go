package requests

import "net/http"

// MiddlewareHandlerFunc defines a function that takes an http.Request and returns an http.Response and an error.
type MiddlewareHandlerFunc func(req *http.Request) (*http.Response, error)

// Middleware defines a function that takes an http.Request and returns an http.Response and an error.
// It wraps around a next function call, which can be another middleware or the final transport layer call.
type Middleware func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc
