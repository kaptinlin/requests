package middlewares

import (
	"context"
	"net/http"
	"testing"

	"github.com/kaptinlin/requests"
)

func TestHeaderMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		headers        http.Header
		existingHeader http.Header
		wantHeader     http.Header
	}{
		{
			name: "Add new headers",
			headers: http.Header{
				"User-Agent": []string{"Custom-Agent"},
				"Accept":     []string{"application/json"},
			},
			existingHeader: http.Header{},
			wantHeader: http.Header{
				"User-Agent": []string{"Custom-Agent"},
				"Accept":     []string{"application/json"},
			},
		},
		{
			name: "Add headers with existing values",
			headers: http.Header{
				"User-Agent": []string{"Custom-Agent-2"},
				"Accept":     []string{"text/plain"},
			},
			existingHeader: http.Header{
				"User-Agent":   []string{"Custom-Agent-1"},
				"Content-Type": []string{"application/json"},
			},
			wantHeader: http.Header{
				"User-Agent":   []string{"Custom-Agent-1", "Custom-Agent-2"},
				"Accept":       []string{"text/plain"},
				"Content-Type": []string{"application/json"},
			},
		},
		{
			name: "Add multiple values for same header",
			headers: http.Header{
				"Accept": []string{"application/json", "text/plain"},
			},
			existingHeader: http.Header{},
			wantHeader: http.Header{
				"Accept": []string{"application/json", "text/plain"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test request with existing headers
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			for k, v := range tt.existingHeader {
				for _, val := range v {
					req.Header.Add(k, val)
				}
			}

			// Create middleware
			middleware := HeaderMiddleware(tt.headers)

			// Create a mock next handler
			nextHandler := requests.MiddlewareHandlerFunc(func(req *http.Request) (*http.Response, error) {
				// Check if headers were properly set
				for key, wantValues := range tt.wantHeader {
					gotValues := req.Header[key]
					if len(gotValues) != len(wantValues) {
						t.Errorf("header %s: got %v values, want %v values", key, gotValues, wantValues)
						continue
					}
					for i, want := range wantValues {
						if i >= len(gotValues) || gotValues[i] != want {
							t.Errorf("header %s[%d]: got %v, want %v", key, i, gotValues, want)
						}
					}
				}
				return &http.Response{}, nil
			})

			client := requests.Create(&requests.Config{
				BaseURL:     "http://example.com",
				Middlewares: []requests.Middleware{middleware},
			})

			// Create an HTTP request object
			resp, err := client.Get("/").Send(context.Background())
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Close() //nolint: errcheck
			// Execute middleware
			handler := middleware(nextHandler)
			_, err = handler(req)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
