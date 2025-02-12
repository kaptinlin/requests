package middlewares

import (
	"net/http"

	"github.com/kaptinlin/requests"
)

var HeaderMiddleware = func(headers http.Header) requests.Middleware {
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
