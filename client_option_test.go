package requests

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

func TestNew_NoOptions(t *testing.T) {
	c := newTestClient(t)
	require.NotNil(t, c)
	assert.NotNil(t, c.httpClient)
	assert.NotNil(t, c.jsonEncoder)
	assert.NotNil(t, c.jsonDecoder)
	assert.Empty(t, c.baseURL)
}

func TestNew_WithBaseURL(t *testing.T) {
	c := newTestClient(t, WithBaseURL("https://api.example.com"))
	assert.Equal(t, "https://api.example.com", c.baseURL)
}

func TestNew_WithTimeout(t *testing.T) {
	c := newTestClient(t, WithTimeout(5*time.Second))
	assert.Equal(t, 5*time.Second, c.httpClient.Timeout)
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

	c := newTestClient(t, WithBaseURL(server.URL), WithHeader("X-Custom", "value"))
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithHeaders(t *testing.T) {
	h := &http.Header{}
	h.Set("X-One", "1")
	h.Set("X-Two", "2")
	c := newTestClient(t, WithHeaders(h))
	assert.Equal(t, "1", c.headers.Get("X-One"))
	assert.Equal(t, "2", c.headers.Get("X-Two"))
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

	c := newTestClient(t, WithBaseURL(server.URL), WithContentType("application/json"))
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

	c := newTestClient(t, WithBaseURL(server.URL), WithAccept("application/xml"))
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

	c := newTestClient(t, WithBaseURL(server.URL), WithUserAgent("TestAgent/1.0"))
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

	c := newTestClient(t, WithBaseURL(server.URL), WithReferer("https://example.com"))
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

	c := newTestClient(t, WithBaseURL(server.URL), WithCookies(map[string]string{"session": "abc123"}))
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

	c := newTestClient(t, WithBaseURL(server.URL), WithBasicAuth("admin", "secret"))
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

	c := newTestClient(t, WithBaseURL(server.URL), WithBearerAuth("my-token"))
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithRetry(t *testing.T) {
	strategy := DefaultBackoffStrategy(2 * time.Second)
	retryIf := func(req *http.Request, resp *http.Response, err error) bool {
		return resp != nil && resp.StatusCode >= 500
	}
	c := newTestClient(t, WithRetry(RetryPolicy{
		Max:         3,
		Backoff:     strategy,
		ShouldRetry: retryIf,
	}))
	assert.Equal(t, 3, c.retry.Max)
	assert.NotNil(t, c.retry.Backoff)
	assert.NotNil(t, c.retry.ShouldRetry)
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

	c := newTestClient(t, WithBaseURL(server.URL), WithMiddleware(mw))
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func createOptionTestTLSServer(t *testing.T) *httptest.Server {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	cert, err := tls.LoadX509KeyPair(".github/testdata/cert.pem", ".github/testdata/key.pem")
	require.NoError(t, err)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	server.StartTLS()
	return server
}

func TestNew_WithInsecureSkipVerify(t *testing.T) {
	server := createOptionTestTLSServer(t)
	defer server.Close()

	c := newTestClient(t, WithBaseURL(server.URL), WithInsecureSkipVerify())
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithTLSConfig(t *testing.T) {
	server := createOptionTestTLSServer(t)
	defer server.Close()

	c := newTestClient(t,
		WithBaseURL(server.URL),
		WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
	)
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithClientCertificateAndTLSServerName(t *testing.T) {
	c := newTestClient(t,
		WithClientCertificate(".github/testdata/cert.pem", ".github/testdata/key.pem"),
		WithTLSServerName("example.com"),
	)
	require.NotNil(t, c.tlsConfig)
	assert.Len(t, c.tlsConfig.Certificates, 1)
	assert.Equal(t, "example.com", c.tlsConfig.ServerName)
}

func TestNew_WithCertificatesAndRootCertificates(t *testing.T) {
	cert, err := tls.LoadX509KeyPair(".github/testdata/cert.pem", ".github/testdata/key.pem")
	require.NoError(t, err)

	c := newTestClient(t,
		WithCertificates(cert),
		WithRootCertificate(".github/testdata/cert.pem"),
		WithRootCertificateFromString("-----BEGIN CERTIFICATE-----"),
	)
	require.NotNil(t, c.tlsConfig)
	assert.Len(t, c.tlsConfig.Certificates, 1)
	assert.NotNil(t, c.tlsConfig.RootCAs)
}

func TestNew_WithProxy(t *testing.T) {
	c := newTestClient(t, WithProxy("http://proxy.example.com:8080"))
	transport, ok := c.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.NotNil(t, transport.Proxy)
}

func TestNew_WithProxy_InvalidURL(t *testing.T) {
	c, err := New(WithProxy("://invalid"))

	require.Error(t, err)
	assert.Nil(t, c)
}

func TestNew_ReturnsOptionErrors(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want error
	}{
		{name: "invalid base URL", opts: []Option{WithBaseURL("://bad")}},
		{name: "negative timeout", opts: []Option{WithTimeout(-time.Nanosecond)}, want: ErrInvalidConfigValue},
		{name: "negative dial timeout", opts: []Option{WithDialTimeout(-time.Nanosecond)}, want: ErrInvalidConfigValue},
		{name: "negative TLS handshake timeout", opts: []Option{WithTLSHandshakeTimeout(-time.Nanosecond)}, want: ErrInvalidConfigValue},
		{name: "negative response header timeout", opts: []Option{WithResponseHeaderTimeout(-time.Nanosecond)}, want: ErrInvalidConfigValue},
		{name: "negative idle conn timeout", opts: []Option{WithIdleConnTimeout(-time.Nanosecond)}, want: ErrInvalidConfigValue},
		{name: "negative retries", opts: []Option{WithRetry(RetryPolicy{Max: -1})}, want: ErrInvalidConfigValue},
		{name: "negative max idle conns", opts: []Option{WithMaxIdleConns(-1)}, want: ErrInvalidConfigValue},
		{name: "negative max idle conns per host", opts: []Option{WithMaxIdleConnsPerHost(-1)}, want: ErrInvalidConfigValue},
		{name: "negative max conns per host", opts: []Option{WithMaxConnsPerHost(-1)}, want: ErrInvalidConfigValue},
		{name: "nil JSON encoder", opts: []Option{WithJSONEncoder(nil)}, want: ErrInvalidConfigValue},
		{name: "nil JSON decoder", opts: []Option{WithJSONDecoder(nil)}, want: ErrInvalidConfigValue},
		{name: "unsupported proxy scheme", opts: []Option{WithProxy("ftp://proxy.example.com")}, want: ErrUnsupportedScheme},
		{name: "missing client certificate files", opts: []Option{WithClientCertificate("missing-cert.pem", "missing-key.pem")}},
		{name: "missing root certificate file", opts: []Option{WithRootCertificate("missing-root.pem")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(tt.opts...)

			require.Error(t, err)
			assert.Nil(t, c)
			if tt.want != nil {
				assert.ErrorIs(t, err, tt.want)
			}
		})
	}
}

func TestNew_WithTransportTimeouts(t *testing.T) {
	c := newTestClient(t,
		WithDialTimeout(5*time.Second),
		WithTLSHandshakeTimeout(3*time.Second),
		WithResponseHeaderTimeout(10*time.Second),
	)
	transport, ok := c.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 3*time.Second, transport.TLSHandshakeTimeout)
	assert.Equal(t, 10*time.Second, transport.ResponseHeaderTimeout)
}

func TestNew_WithConnectionPool(t *testing.T) {
	c := newTestClient(t,
		WithMaxIdleConns(50),
		WithMaxIdleConnsPerHost(10),
		WithMaxConnsPerHost(20),
		WithIdleConnTimeout(30*time.Second),
	)
	transport, ok := c.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 50, transport.MaxIdleConns)
	assert.Equal(t, 10, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 20, transport.MaxConnsPerHost)
	assert.Equal(t, 30*time.Second, transport.IdleConnTimeout)
}

func TestNew_WithRedirectPolicy(t *testing.T) {
	c := newTestClient(t, WithRedirectPolicy(NewProhibitRedirectPolicy()))
	assert.NotNil(t, c.httpClient.CheckRedirect)
}

func TestNew_WithLogger(t *testing.T) {
	logger := NewDefaultLogger(nil, LevelDebug)
	c := newTestClient(t, WithLogger(logger))
	assert.NotNil(t, c.logger)
}

func TestNew_WithEncoders(t *testing.T) {
	c := newTestClient(t,
		WithJSONEncoder(DefaultJSONEncoder),
		WithJSONDecoder(DefaultJSONDecoder),
		WithXMLEncoder(DefaultXMLEncoder),
		WithXMLDecoder(DefaultXMLDecoder),
		WithYAMLEncoder(DefaultYAMLEncoder),
		WithYAMLDecoder(DefaultYAMLDecoder),
	)
	assert.NotNil(t, c.jsonEncoder)
	assert.NotNil(t, c.jsonDecoder)
	assert.NotNil(t, c.xmlEncoder)
	assert.NotNil(t, c.xmlDecoder)
	assert.NotNil(t, c.yamlEncoder)
	assert.NotNil(t, c.yamlDecoder)
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

	c := newTestClient(t,
		WithBaseURL(server.URL),
		WithTimeout(10*time.Second),
		WithBearerAuth("token123"),
		WithUserAgent("MyApp/2.0"),
		WithRetry(RetryPolicy{Max: 2}),
	)

	assert.Equal(t, server.URL, c.baseURL)
	assert.Equal(t, 10*time.Second, c.httpClient.Timeout)
	assert.Equal(t, 2, c.retry.Max)

	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestNew_WithHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 42 * time.Second}
	c := newTestClient(t, WithHTTPClient(customClient))
	assert.Equal(t, 42*time.Second, c.httpClient.Timeout)
}

func TestNew_WithTransport(t *testing.T) {
	transport := &http.Transport{MaxIdleConns: 99}
	c := newTestClient(t, WithTransport(transport))
	tr, ok := c.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 99, tr.MaxIdleConns)
}

func TestNew_WithCookieJar(t *testing.T) {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	c := newTestClient(t, WithCookieJar(jar))
	assert.Equal(t, jar, c.httpClient.Jar)
}

func TestNew_WithSession(t *testing.T) {
	c := newTestClient(t, WithSession())
	require.NotNil(t, c.httpClient.Jar)
	require.NotNil(t, c.tlsConfig)
	assert.NotNil(t, c.tlsConfig.ClientSessionCache)
}

func TestEnableSessionPreservesExistingSessionStores(t *testing.T) {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	cache := tls.NewLRUClientSessionCache(1)
	c := newTestClient(t,
		WithCookieJar(jar),
		WithTLSConfig(&tls.Config{ClientSessionCache: cache}),
	)

	c.enableSession()

	assert.Equal(t, jar, c.httpClient.Jar)
	assert.Equal(t, cache, c.tlsConfig.ClientSessionCache)
}

func TestNew_WithHTTP2(t *testing.T) {
	c := newTestClient(t, WithHTTP2())
	transport, ok := c.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assertHTTP2Configured(t, transport)
}

func TestNew_WithHTTP2PreservesOptionOrder(t *testing.T) {
	t.Run("proxy before HTTP2", func(t *testing.T) {
		c := newTestClient(t, WithProxy("http://127.0.0.1:8080"), WithHTTP2())

		transport, ok := c.httpClient.Transport.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, transport.Proxy)
		assertHTTP2Configured(t, transport)
	})

	t.Run("proxy after HTTP2", func(t *testing.T) {
		c := newTestClient(t, WithHTTP2(), WithProxy("http://127.0.0.1:8080"))

		transport, ok := c.httpClient.Transport.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, transport.Proxy)
		assertHTTP2Configured(t, transport)
	})

	t.Run("dial timeout before HTTP2", func(t *testing.T) {
		c := newTestClient(t, WithDialTimeout(5*time.Second), WithHTTP2())

		transport, ok := c.httpClient.Transport.(*http.Transport)
		require.True(t, ok)
		assert.NotNil(t, transport.DialContext)
		assertHTTP2Configured(t, transport)
	})

	t.Run("dial timeout after HTTP2", func(t *testing.T) {
		c := newTestClient(t, WithHTTP2(), WithDialTimeout(5*time.Second))

		transport, ok := c.httpClient.Transport.(*http.Transport)
		require.True(t, ok)
		assert.NotNil(t, transport.DialContext)
		assertHTTP2Configured(t, transport)
	})

	t.Run("TLS config before HTTP2", func(t *testing.T) {
		tlsConfig := &tls.Config{ServerName: "example.com"}
		c := newTestClient(t, WithTLSConfig(tlsConfig), WithHTTP2())

		transport, ok := c.httpClient.Transport.(*http.Transport)
		require.True(t, ok)
		assert.Same(t, tlsConfig, transport.TLSClientConfig)
		assertHTTP2Configured(t, transport)
	})

	t.Run("TLS config after HTTP2", func(t *testing.T) {
		tlsConfig := &tls.Config{ServerName: "example.com"}
		c := newTestClient(t, WithHTTP2(), WithTLSConfig(tlsConfig))

		transport, ok := c.httpClient.Transport.(*http.Transport)
		require.True(t, ok)
		assert.Same(t, tlsConfig, transport.TLSClientConfig)
		assertHTTP2Configured(t, transport)
	})
}

func TestNew_WithDialOptions(t *testing.T) {
	resolver := &net.Resolver{}
	localAddr := &net.TCPAddr{IP: net.IPv4zero}
	c := newTestClient(t,
		WithResolver(resolver),
		WithLocalAddr(localAddr),
	)

	transport, ok := c.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.NotNil(t, transport.DialContext)
}

func TestNew_WithDialContext(t *testing.T) {
	called := false
	c := newTestClient(t, WithDialContext(func(context.Context, string, string) (net.Conn, error) {
		called = true
		return nil, assert.AnError
	}))

	transport, ok := c.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	_, err := transport.DialContext(context.Background(), "tcp", "127.0.0.1:1")
	assert.ErrorIs(t, err, assert.AnError)
	assert.True(t, called)
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

	c := newTestClient(t,
		WithBaseURL(server.URL),
		WithAuth(CustomAuth{Header: "Custom my-auth-value"}),
	)
	resp, err := c.Get("/").Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}
