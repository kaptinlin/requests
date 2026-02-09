package middlewares

import (
	"net/http"

	"github.com/kaptinlin/requests"
)

// HeaderMiddleware creates a middleware that adds the specified headers to each request.
func HeaderMiddleware(headers http.Header) requests.Middleware {
	return func(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			for key, values := range headers {
				for _, value := range values {
					req.Header.Add(key, value)
				}
			}
			return next(req)
		}
	}
}
