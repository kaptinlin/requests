package requests

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

func TestNew_NoOptions(t *testing.T) {
	c := New()
	require.NotNil(t, c)
	assert.NotNil(t, c.HTTPClient)
	assert.NotNil(t, c.JSONEncoder)
	assert.NotNil(t, c.JSONDecoder)
	assert.Empty(t, c.BaseURL)
}

func TestNew_WithBaseURL(t *testing.T) {
	c := New(WithBaseURL("https://api.example.com"))
	assert.Equal(t, "https://api.example.com", c.BaseURL)
}

func TestNew_WithTimeout(t *testing.T) {
	c := New(WithTimeout(5 * time.Second))
	assert.Equal(t, 5*time.Second, c.HTTPClient.Timeout)
}

func TestNew_WithHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "value" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(WithBaseURL(server.URL), WithHeader("X-Custom", "value"))
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithHeaders(t *testing.T) {
	h := &http.Header{}
	h.Set("X-One", "1")
	h.Set("X-Two", "2")
	c := New(WithHeaders(h))
	assert.Equal(t, "1", c.Headers.Get("X-One"))
	assert.Equal(t, "2", c.Headers.Get("X-Two"))
}

func TestNew_WithContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(WithBaseURL(server.URL), WithContentType("application/json"))
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithAccept(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/xml" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(WithBaseURL(server.URL), WithAccept("application/xml"))
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithUserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "TestAgent/1.0" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(WithBaseURL(server.URL), WithUserAgent("TestAgent/1.0"))
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithReferer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://example.com" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(WithBaseURL(server.URL), WithReferer("https://example.com"))
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value != "abc123" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(WithBaseURL(server.URL), WithCookies(map[string]string{"session": "abc123"}))
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(WithBaseURL(server.URL), WithBasicAuth("admin", "secret"))
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithBearerAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer my-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(WithBaseURL(server.URL), WithBearerAuth("my-token"))
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithMaxRetries(t *testing.T) {
	c := New(WithMaxRetries(3))
	assert.Equal(t, 3, c.MaxRetries)
}

func TestNew_WithRetryStrategy(t *testing.T) {
	strategy := DefaultBackoffStrategy(2 * time.Second)
	c := New(WithRetryStrategy(strategy))
	assert.NotNil(t, c.RetryStrategy)
}

func TestNew_WithRetryIf(t *testing.T) {
	retryIf := func(req *http.Request, resp *http.Response, err error) bool {
		return resp != nil && resp.StatusCode >= 500
	}
	c := New(WithRetryIf(retryIf))
	assert.NotNil(t, c.RetryIf)
}

func TestNew_WithMiddleware(t *testing.T) {
	mw := func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			req.Header.Set("X-Middleware", "applied")
			return next(req)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Middleware") != "applied" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(WithBaseURL(server.URL), WithMiddleware(mw))
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithInsecureSkipVerify(t *testing.T) {
	server := createTestTLSServer()
	defer server.Close()

	c := New(WithBaseURL(server.URL), WithInsecureSkipVerify())
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithTLSConfig(t *testing.T) {
	server := createTestTLSServer()
	defer server.Close()

	c := New(
		WithBaseURL(server.URL),
		WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
	)
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithProxy(t *testing.T) {
	c := New(WithProxy("http://proxy.example.com:8080"))
	transport, ok := c.HTTPClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.NotNil(t, transport.Proxy)
}

func TestNew_WithProxy_InvalidURL(t *testing.T) {
	// Invalid proxy URL should be silently ignored
	c := New(WithProxy("://invalid"))
	assert.NotNil(t, c)
}

func TestNew_WithTransportTimeouts(t *testing.T) {
	c := New(
		WithDialTimeout(5*time.Second),
		WithTLSHandshakeTimeout(3*time.Second),
		WithResponseHeaderTimeout(10*time.Second),
	)
	transport, ok := c.HTTPClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 3*time.Second, transport.TLSHandshakeTimeout)
	assert.Equal(t, 10*time.Second, transport.ResponseHeaderTimeout)
}

func TestNew_WithConnectionPool(t *testing.T) {
	c := New(
		WithMaxIdleConns(50),
		WithMaxIdleConnsPerHost(10),
		WithMaxConnsPerHost(20),
		WithIdleConnTimeout(30*time.Second),
	)
	transport, ok := c.HTTPClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 50, transport.MaxIdleConns)
	assert.Equal(t, 10, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 20, transport.MaxConnsPerHost)
	assert.Equal(t, 30*time.Second, transport.IdleConnTimeout)
}

func TestNew_WithRedirectPolicy(t *testing.T) {
	c := New(WithRedirectPolicy(NewProhibitRedirectPolicy()))
	assert.NotNil(t, c.HTTPClient.CheckRedirect)
}

func TestNew_WithLogger(t *testing.T) {
	logger := NewDefaultLogger(nil, LevelDebug)
	c := New(WithLogger(logger))
	assert.NotNil(t, c.Logger)
}

func TestNew_WithEncoders(t *testing.T) {
	customMarshal := func(v any) ([]byte, error) { return nil, nil }
	customUnmarshal := func(data []byte, v any) error { return nil }

	c := New(
		WithJSONMarshal(customMarshal),
		WithJSONUnmarshal(customUnmarshal),
		WithXMLMarshal(customMarshal),
		WithXMLUnmarshal(customUnmarshal),
		WithYAMLMarshal(customMarshal),
		WithYAMLUnmarshal(customUnmarshal),
	)
	assert.NotNil(t, c.JSONEncoder)
	assert.NotNil(t, c.JSONDecoder)
	assert.NotNil(t, c.XMLEncoder)
	assert.NotNil(t, c.XMLDecoder)
	assert.NotNil(t, c.YAMLEncoder)
	assert.NotNil(t, c.YAMLDecoder)
}

func TestNew_MultipleOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("User-Agent") != "MyApp/2.0" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	}))
	defer server.Close()

	c := New(
		WithBaseURL(server.URL),
		WithTimeout(10*time.Second),
		WithBearerAuth("token123"),
		WithUserAgent("MyApp/2.0"),
		WithMaxRetries(2),
	)

	assert.Equal(t, server.URL, c.BaseURL)
	assert.Equal(t, 10*time.Second, c.HTTPClient.Timeout)
	assert.Equal(t, 2, c.MaxRetries)

	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 42 * time.Second}
	c := New(WithHTTPClient(customClient))
	assert.Equal(t, 42*time.Second, c.HTTPClient.Timeout)
}

func TestNew_WithTransport(t *testing.T) {
	transport := &http.Transport{MaxIdleConns: 99}
	c := New(WithTransport(transport))
	tr, ok := c.HTTPClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 99, tr.MaxIdleConns)
}

func TestNew_WithCookieJar(t *testing.T) {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	c := New(WithCookieJar(jar))
	assert.Equal(t, jar, c.HTTPClient.Jar)
}

func TestNew_WithAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Custom my-auth-value" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(
		WithBaseURL(server.URL),
		WithAuth(CustomAuth{Header: "Custom my-auth-value"}),
	)
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}
