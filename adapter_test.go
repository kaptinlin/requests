package requests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kaptinlin/orderedobject"
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

	httpClient := client.AsHTTPClient()
	require.Equal(t, 5*time.Second, httpClient.Timeout)

	resp, err := httpClient.Get(server.URL)
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

func TestAsHTTPClientAppliesDefaultsToExampleHost(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "api.example.com", r.Host)
		assert.Equal(t, "value", r.Header.Get("X-Default"))
		assert.Equal(t, "middleware", r.Header.Get("X-Middleware"))
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
		cookie, err := r.Cookie("session")
		require.NoError(t, err)
		assert.Equal(t, "abc", cookie.Value)
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	client := New(
		WithHTTPClient(server.Client()),
		WithHeader("X-Default", "value"),
		WithBearerAuth("token"),
		WithCookies(map[string]string{"session": "abc"}),
	)
	client.AddMiddleware(func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			req.Header.Set("X-Middleware", "middleware")
			return next(req)
		}
	})

	resp, err := client.AsHTTPClient().Get("https://api.example.com/resource")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAsTransportAppliesDefaultsToExampleHost(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "api.example.com", r.Host)
		assert.Equal(t, "value", r.Header.Get("X-Default"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(WithHTTPClient(server.Client()), WithHeader("X-Default", "value"))
	req, err := http.NewRequest(http.MethodGet, "https://api.example.com/resource", nil)
	require.NoError(t, err)

	resp, err := client.AsTransport().RoundTrip(req)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()

	assert.Empty(t, req.Header.Get("X-Default"))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAsTransportAttachesDefaultOrderedHeaders(t *testing.T) {
	headers := orderedobject.NewObject[[]string]().
		Set("X-First", []string{"1"}).
		Set(":authority", []string{"metadata-only"}).
		Set("X-Second", []string{"2"})

	client := New(
		WithOrderedHeaders(headers),
		WithTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "1", req.Header.Get("X-First"))
			assert.Equal(t, "2", req.Header.Get("X-Second"))
			assert.Empty(t, req.Header.Get(":authority"))

			ordered, ok := OrderedHeaders(req)
			require.True(t, ok)
			assert.Equal(t, []string{"X-First", ":authority", "X-Second"}, ordered.Keys())

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{},
				Body:       http.NoBody,
				Request:    req,
			}, nil
		})),
	)

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	resp, err := client.AsTransport().RoundTrip(req)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAsTransportDropsOrderedMetadataForOriginalHeaderOverrides(t *testing.T) {
	headers := orderedobject.NewObject[[]string]().
		Set("X-First", []string{"default"}).
		Set("X-Keep", []string{"default"})

	client := New(
		WithOrderedHeaders(headers),
		WithTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "request", req.Header.Get("X-First"))
			assert.Equal(t, "default", req.Header.Get("X-Keep"))

			ordered, ok := OrderedHeaders(req)
			require.True(t, ok)
			assert.Equal(t, []string{"X-Keep"}, ordered.Keys())

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{},
				Body:       http.NoBody,
				Request:    req,
			}, nil
		})),
	)

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)
	req.Header.Set("X-First", "request")

	resp, err := client.AsTransport().RoundTrip(req)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
