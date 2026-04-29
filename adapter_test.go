package requests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

func TestAsHTTPClientAppliesClientDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "value", r.Header.Get("X-Default"))
		assert.Equal(t, "middleware", r.Header.Get("X-Middleware"))
		assert.Equal(t, "Basic dXNlcjpwYXNz", r.Header.Get("Authorization"))
		cookie, err := r.Cookie("session")
		require.NoError(t, err)
		assert.Equal(t, "abc", cookie.Value)
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	client := New(
		WithTimeout(5*time.Second),
		WithHeader("X-Default", "value"),
		WithBasicAuth("user", "pass"),
		WithCookies(map[string]string{"session": "abc"}),
	)
	client.AddMiddleware(func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			req.Header.Set("X-Middleware", "middleware")
			return next(req)
		}
	})

	stdClient := client.AsHTTPClient()
	require.Equal(t, 5*time.Second, stdClient.Timeout)

	resp, err := stdClient.Get(server.URL)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAsTransportDoesNotMutateOriginalRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "value", r.Header.Get("X-Default"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(WithHeader("X-Default", "value"))
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	resp, err := client.AsTransport().RoundTrip(req)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()

	assert.Empty(t, req.Header.Get("X-Default"))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
