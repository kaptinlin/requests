package middlewares

import (
	"net/http"

	"github.com/kaptinlin/requests"
)

// CookieMiddleware creates a middleware that adds the specified cookies to each request.
func CookieMiddleware(cookies []*http.Cookie) requests.Middleware {
	return func(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			for _, cookie := range cookies {
				req.AddCookie(cookie)
			}
			return next(req)
		}
	}
}
