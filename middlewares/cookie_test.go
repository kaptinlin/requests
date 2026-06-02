package middlewares

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kaptinlin/requests"
)

func TestCookieMiddleware(t *testing.T) {
	tests := []struct {
		name            string
		cookies         []*http.Cookie
		existingCookies []*http.Cookie
		wantCookies     []*http.Cookie
	}{
		{
			name: "Add new cookies",
			cookies: []*http.Cookie{
				{Name: "session", Value: "abc123"},
				{Name: "user", Value: "john"},
			},
			existingCookies: nil,
			wantCookies: []*http.Cookie{
				{Name: "session", Value: "abc123"},
				{Name: "user", Value: "john"},
			},
		},
		{
			name: "Add cookies with existing values",
			cookies: []*http.Cookie{
				{Name: "session", Value: "xyz789"},
				{Name: "theme", Value: "dark"},
			},
			existingCookies: []*http.Cookie{
				{Name: "user", Value: "alice"},
			},
			wantCookies: []*http.Cookie{
				{Name: "user", Value: "alice"},
				{Name: "session", Value: "xyz789"},
				{Name: "theme", Value: "dark"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test request
			req, _ := http.NewRequest("GET", "http://example.com", nil)

			// Add existing cookies
			for _, cookie := range tt.existingCookies {
				req.AddCookie(cookie)
			}

			// Create middleware
			middleware := CookieMiddleware(tt.cookies)

			// Create a mock next handler
			nextHandler := requests.MiddlewareHandlerFunc(func(req *http.Request) (*http.Response, error) {
				// Get all cookies from request
				cookies := req.Cookies()

				// Check if cookies were properly set
				assert.Equal(t, len(tt.wantCookies), len(cookies),
					"unexpected number of cookies")

				// Create a map for easier comparison
				gotCookies := make(map[string]*http.Cookie)
				for _, cookie := range cookies {
					gotCookies[cookie.Name] = cookie
				}

				// Compare each wanted cookie
				for _, wantCookie := range tt.wantCookies {
					gotCookie, exists := gotCookies[wantCookie.Name]
					assert.True(t, exists, "cookie %s not found", wantCookie.Name)
					if exists {
						assert.Equal(t, wantCookie.Value, gotCookie.Value,
							"cookie %s: unexpected value", wantCookie.Name)
					}
				}
				return &http.Response{}, nil
			})

			// Execute middleware
			handler := middleware(nextHandler)
			_, err := handler(req)
			assert.NoError(t, err, "unexpected error")
		})
	}
}
