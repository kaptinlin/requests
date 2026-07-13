package requests

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

func TestNew_NoOptions(t *testing.T) {
	c := newTestClient(t)
	require.NotNil(t, c)
	assert.NotNil(t, c.httpClient)
	assert.Nil(t, c.UnsafeHTTPClient().Transport)
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
	h := http.Header{}
	h.Set("X-One", "1")
	h.Set("X-Two", "2")
	c := newTestClient(t, WithHeaders(h))
	assert.Equal(t, "1", c.headers.Get("X-One"))
	assert.Equal(t, "2", c.headers.Get("X-Two"))
}

func TestNew_WithHeadersCapturesCallerValues(t *testing.T) {
	observed := make(chan []string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed <- r.Header.Values("X-Snapshot")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	headers := http.Header{"X-Snapshot": {"initial", "second"}}
	client := newTestClient(t, WithBaseURL(server.URL), WithHeaders(headers))

	headers["X-Snapshot"][0] = "mutated"
	headers.Add("X-Snapshot", "third")

	resp, err := client.Get("/").Send(t.Context())
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode())
	if diff := cmp.Diff([]string{"initial", "second"}, <-observed); diff != "" {
		t.Errorf("request headers mismatch (-want +got):\n%s", diff)
	}
}

func TestNew_WithNilHeadersCanBeExtended(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "value", r.Header.Get("X-Later"))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(t,
		WithBaseURL(server.URL),
		WithHeaders(nil),
		WithHeader("X-Later", "value"),
	)
	resp, err := client.Get("/").Send(t.Context())
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode())
}

func TestNew_WithHeadersCapturesCallerValuesForAdapters(t *testing.T) {
	var observed []string
	transport := testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		observed = req.Header.Values("X-Snapshot")
		return nil, assert.AnError
	})
	headers := http.Header{"X-Snapshot": {"initial", "second"}}
	client := newTestClient(t, WithHeaders(headers), WithTransport(transport))

	headers["X-Snapshot"][0] = "mutated"
	headers.Add("X-Snapshot", "third")

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)
	_, err = client.AsTransport().RoundTrip(req)
	assert.ErrorIs(t, err, assert.AnError)
	if diff := cmp.Diff([]string{"initial", "second"}, observed); diff != "" {
		t.Errorf("adapter request headers mismatch (-want +got):\n%s", diff)
	}
}

func TestClientCloneOwnsIndependentHeaders(t *testing.T) {
	observed := make(chan string, 2)
	transport := testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		observed <- req.Header.Get("X-Owner")
		return nil, assert.AnError
	})
	base := newTestClient(t,
		WithHeaders(http.Header{"X-Owner": {"base"}}),
		WithTransport(transport),
	)
	clone, err := base.Clone(WithHeader("X-Owner", "clone"))
	require.NoError(t, err)

	for _, client := range []*Client{base, clone} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com", nil)
		require.NoError(t, err)
		_, err = client.AsTransport().RoundTrip(req)
		assert.ErrorIs(t, err, assert.AnError)
	}

	assert.Equal(t, "base", <-observed)
	assert.Equal(t, "clone", <-observed)
}

func TestNew_WithHeadersDoesNotRaceWithCallerMutation(t *testing.T) {
	headers := http.Header{"X-Snapshot": {"initial"}}
	transport := testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		_ = req.Header.Get("X-Snapshot")
		return nil, assert.AnError
	})
	client := newTestClient(t, WithHeaders(headers), WithTransport(transport))

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		<-start
		for i := range 1_000 {
			headers["X-Snapshot"][0] = fmt.Sprintf("caller-%d", i)
		}
	})
	wg.Go(func() {
		<-start
		for range 1_000 {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com", nil)
			if err != nil {
				t.Errorf("NewRequestWithContext() error = %v", err)
				return
			}
			_, _ = client.AsTransport().RoundTrip(req)
		}
	})
	close(start)
	wg.Wait()
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

func TestNew_WithTLSConfigNilClearsEarlierTLSState(t *testing.T) {
	client := newTestClient(t,
		WithInsecureSkipVerify(),
		WithTLSConfig(nil),
	)

	assert.Nil(t, client.GetTLSConfig())
	transport, ok := client.UnsafeHTTPClient().Transport.(*http.Transport)
	require.True(t, ok)
	assert.Nil(t, transport.TLSClientConfig)
}

func TestNew_WithTLSConfigCapturesTopLevelValues(t *testing.T) {
	tlsConfig := &tls.Config{
		ServerName: "initial.example.com",
		NextProtos: []string{"h2", "http/1.1"},
		MinVersion: tls.VersionTLS13,
	}
	client := newTestClient(t, WithTLSConfig(tlsConfig))

	tlsConfig.ServerName = "mutated.example.com"
	tlsConfig.NextProtos = []string{"mutated"}
	tlsConfig.MinVersion = tls.VersionTLS12

	got := client.GetTLSConfig()
	require.NotNil(t, got)
	assert.NotSame(t, tlsConfig, got)
	assert.Equal(t, "initial.example.com", got.ServerName)
	assert.Equal(t, uint16(tls.VersionTLS13), got.MinVersion)
	if diff := cmp.Diff([]string{"h2", "http/1.1"}, got.NextProtos); diff != "" {
		t.Errorf("GetTLSConfig().NextProtos mismatch (-want +got):\n%s", diff)
	}

	transport, ok := client.UnsafeHTTPClient().Transport.(*http.Transport)
	require.True(t, ok)
	assert.NotSame(t, tlsConfig, transport.TLSClientConfig)
	assert.Equal(t, "initial.example.com", transport.TLSClientConfig.ServerName)
}

func TestClientCloneOwnsIndependentTLSConfig(t *testing.T) {
	base := newTestClient(t, WithTLSConfig(&tls.Config{ServerName: "base.example.com"}))
	clone, err := base.Clone(WithTLSServerName("clone.example.com"))
	require.NoError(t, err)

	baseConfig := base.GetTLSConfig()
	cloneConfig := clone.GetTLSConfig()
	require.NotNil(t, baseConfig)
	require.NotNil(t, cloneConfig)
	assert.NotSame(t, baseConfig, cloneConfig)
	assert.Equal(t, "base.example.com", baseConfig.ServerName)
	assert.Equal(t, "clone.example.com", cloneConfig.ServerName)
}

func TestNew_WithTLSConfigDoesNotRaceWithCallerMutation(t *testing.T) {
	tlsConfig := &tls.Config{ServerName: "initial.example.com"}
	client := newTestClient(t, WithTLSConfig(tlsConfig))

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		<-start
		for i := range 1_000 {
			tlsConfig.ServerName = fmt.Sprintf("caller-%d.example.com", i)
		}
	})
	wg.Go(func() {
		<-start
		for range 1_000 {
			_ = client.GetTLSConfig().ServerName
		}
	})
	close(start)
	wg.Wait()
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
	rootPEM, err := os.ReadFile(".github/testdata/cert.pem")
	require.NoError(t, err)

	c := newTestClient(t,
		WithCertificates(cert),
		WithRootCertificate(".github/testdata/cert.pem"),
		WithRootCertificateFromString(string(rootPEM)),
	)
	require.NotNil(t, c.tlsConfig)
	assert.Len(t, c.tlsConfig.Certificates, 1)
	assert.NotNil(t, c.tlsConfig.RootCAs)
}

func TestNew_WithCertificatesCapturesMutableMetadata(t *testing.T) {
	certificate := tls.Certificate{
		Certificate:                  [][]byte{{1, 2, 3}},
		SupportedSignatureAlgorithms: []tls.SignatureScheme{tls.PKCS1WithSHA256},
		OCSPStaple:                   []byte{4, 5, 6},
		SignedCertificateTimestamps:  [][]byte{{7, 8, 9}},
	}
	client := newTestClient(t, WithCertificates(certificate))

	certificate.Certificate[0][0] = 10
	certificate.SupportedSignatureAlgorithms[0] = tls.PSSWithSHA256
	certificate.OCSPStaple[0] = 11
	certificate.SignedCertificateTimestamps[0][0] = 12

	config := client.GetTLSConfig()
	require.NotNil(t, config)
	require.Len(t, config.Certificates, 1)
	got := config.Certificates[0]
	want := struct {
		Certificate [][]byte
		Signatures  []tls.SignatureScheme
		OCSP        []byte
		SCTs        [][]byte
	}{
		Certificate: [][]byte{{1, 2, 3}},
		Signatures:  []tls.SignatureScheme{tls.PKCS1WithSHA256},
		OCSP:        []byte{4, 5, 6},
		SCTs:        [][]byte{{7, 8, 9}},
	}
	actual := struct {
		Certificate [][]byte
		Signatures  []tls.SignatureScheme
		OCSP        []byte
		SCTs        [][]byte
	}{
		Certificate: got.Certificate,
		Signatures:  got.SupportedSignatureAlgorithms,
		OCSP:        got.OCSPStaple,
		SCTs:        got.SignedCertificateTimestamps,
	}
	if diff := cmp.Diff(want, actual); diff != "" {
		t.Errorf("captured certificate mismatch (-want +got):\n%s", diff)
	}
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
		{name: "nil HTTP client", opts: []Option{WithHTTPClient(nil)}, want: ErrInvalidConfigValue},
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

type nilAuth struct{}

func (*nilAuth) Apply(*http.Request) {}

func (*nilAuth) Valid() bool { return true }

func TestNew_RejectsInvalidAuth(t *testing.T) {
	tests := []struct {
		name   string
		option Option
	}{
		{name: "nil", option: WithAuth(nil)},
		{name: "typed nil", option: WithAuth((*nilAuth)(nil))},
		{name: "empty basic", option: WithBasicAuth("", "")},
		{name: "empty bearer", option: WithBearerAuth("")},
		{name: "empty custom", option: WithAuth(CustomAuth{})},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := New(test.option)
			assert.Nil(t, client)
			assert.ErrorIs(t, err, ErrInvalidConfigValue)
		})
	}
}

func TestNew_RejectsNilMiddleware(t *testing.T) {
	valid := func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc { return next }
	tests := []struct {
		name   string
		option Option
	}{
		{name: "nil", option: WithMiddleware(nil)},
		{name: "nil after valid", option: WithMiddleware(valid, nil)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := New(test.option)
			assert.Nil(t, client)
			assert.ErrorIs(t, err, ErrInvalidConfigValue)
		})
	}
}

func TestNew_RejectsNilRedirectPolicy(t *testing.T) {
	tests := []struct {
		name   string
		option Option
	}{
		{name: "empty", option: WithRedirectPolicy()},
		{name: "nil", option: WithRedirectPolicy(nil)},
		{name: "typed nil", option: WithRedirectPolicy((*AllowRedirectPolicy)(nil))},
		{name: "nil after valid", option: WithRedirectPolicy(NewProhibitRedirectPolicy(), nil)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := New(test.option)
			assert.Nil(t, client)
			assert.ErrorIs(t, err, ErrInvalidConfigValue)
		})
	}
}

func TestNew_RejectsInvalidRootCertificatePEM(t *testing.T) {
	invalidPath := filepath.Join(t.TempDir(), "invalid.pem")
	require.NoError(t, os.WriteFile(invalidPath, []byte("not a certificate"), 0o600))
	tests := []struct {
		name   string
		option Option
	}{
		{name: "empty string", option: WithRootCertificateFromString("")},
		{name: "invalid string", option: WithRootCertificateFromString("not a certificate")},
		{name: "invalid file", option: WithRootCertificate(invalidPath)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := New(test.option)
			assert.Nil(t, client)
			assert.ErrorIs(t, err, ErrInvalidConfigValue)
			if assert.Error(t, err) {
				assert.NotContains(t, err.Error(), "not a certificate")
			}
		})
	}
}

func TestClone_UsesConstructionValidation(t *testing.T) {
	var gotHeader string
	transport := testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		gotHeader = req.Header.Get("X-Base")
		return nil, assert.AnError
	})
	base := newTestClient(t, WithHeader("X-Base", "value"), WithTransport(transport))

	clone, err := base.Clone(WithMiddleware(nil))

	assert.Nil(t, clone)
	assert.ErrorIs(t, err, ErrInvalidConfigValue)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)
	_, err = base.AsTransport().RoundTrip(request)
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, "value", gotHeader)
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

func TestNew_TransportMutatingOptionsRejectCustomTransport(t *testing.T) {
	tests := []struct {
		name string
		opt  Option
	}{
		{name: "HTTP2", opt: WithHTTP2()},
		{name: "TLS config", opt: WithTLSConfig(&tls.Config{})},
		{name: "insecure TLS", opt: WithInsecureSkipVerify()},
		{name: "certificates", opt: WithCertificates(tls.Certificate{})},
		{name: "TLS server name", opt: WithTLSServerName("example.com")},
		{name: "session", opt: WithSession()},
		{name: "dial timeout", opt: WithDialTimeout(time.Second)},
		{name: "resolver", opt: WithResolver(&net.Resolver{})},
		{
			name: "dial context",
			opt: WithDialContext(func(context.Context, string, string) (net.Conn, error) {
				return nil, assert.AnError
			}),
		},
		{name: "local addr", opt: WithLocalAddr(&net.TCPAddr{IP: net.IPv4zero})},
		{name: "TLS handshake timeout", opt: WithTLSHandshakeTimeout(time.Second)},
		{name: "response header timeout", opt: WithResponseHeaderTimeout(time.Second)},
		{name: "max idle conns", opt: WithMaxIdleConns(1)},
		{name: "max idle conns per host", opt: WithMaxIdleConnsPerHost(1)},
		{name: "max conns per host", opt: WithMaxConnsPerHost(1)},
		{name: "idle conn timeout", opt: WithIdleConnTimeout(time.Second)},
		{name: "without proxy", opt: WithoutProxy()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customClient := &http.Client{Transport: testRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, assert.AnError
			})}

			c, err := New(WithHTTPClient(customClient), tt.opt)

			require.Error(t, err)
			assert.Nil(t, c)
			assert.ErrorIs(t, err, ErrInvalidTransportType)
		})
	}
}

func TestClone_TransportMutationFailureDoesNotChangeBaseClient(t *testing.T) {
	roundTrip := testRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, assert.AnError
	})
	transport := &roundTrip
	base := newTestClient(t, WithTransport(transport))

	clone, err := base.Clone(WithTLSConfig(&tls.Config{ServerName: "example.com"}))

	require.Nil(t, clone)
	assert.ErrorIs(t, err, ErrInvalidTransportType)
	assert.Same(t, transport, base.UnsafeHTTPClient().Transport)
	assert.Nil(t, base.GetTLSConfig())
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

func TestNew_TransportOptionsPreserveNetHTTPDefaults(t *testing.T) {
	baseline := http.DefaultTransport.(*http.Transport)
	baselineValues := transportDefaultsFrom(baseline)

	tests := []struct {
		name   string
		option Option
		adjust func(*transportDefaults)
		check  func(*testing.T, *http.Transport)
	}{
		{
			name:   "proxy",
			option: WithProxy("http://proxy.example.com:8080"),
			check: func(t *testing.T, transport *http.Transport) {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com", nil)
				require.NoError(t, err)
				proxyURL, err := transport.Proxy(req)
				require.NoError(t, err)
				assert.Equal(t, "proxy.example.com:8080", proxyURL.Host)
			},
		},
		{
			name:   "proxy from environment",
			option: WithProxyFromEnv(),
			check: func(t *testing.T, transport *http.Transport) {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.example.com", nil)
				require.NoError(t, err)
				want, wantErr := baseline.Proxy(req)
				got, gotErr := transport.Proxy(req)
				assert.Equal(t, wantErr, gotErr)
				assert.Equal(t, want, got)
			},
		},
		{
			name:   "dial timeout",
			option: WithDialTimeout(5 * time.Second),
			check: func(t *testing.T, transport *http.Transport) {
				assert.NotNil(t, transport.DialContext)
			},
		},
		{
			name:   "TLS handshake timeout",
			option: WithTLSHandshakeTimeout(3 * time.Second),
			adjust: func(values *transportDefaults) { values.tlsHandshakeTimeout = 3 * time.Second },
		},
		{
			name:   "response header timeout",
			option: WithResponseHeaderTimeout(4 * time.Second),
			adjust: func(values *transportDefaults) { values.responseHeaderTimeout = 4 * time.Second },
		},
		{
			name:   "max idle connections",
			option: WithMaxIdleConns(50),
			adjust: func(values *transportDefaults) { values.maxIdleConns = 50 },
		},
		{
			name:   "max idle connections per host",
			option: WithMaxIdleConnsPerHost(10),
			adjust: func(values *transportDefaults) { values.maxIdleConnsPerHost = 10 },
		},
		{
			name:   "max connections per host",
			option: WithMaxConnsPerHost(7),
			adjust: func(values *transportDefaults) { values.maxConnsPerHost = 7 },
		},
		{
			name:   "idle connection timeout",
			option: WithIdleConnTimeout(30 * time.Second),
			adjust: func(values *transportDefaults) { values.idleConnTimeout = 30 * time.Second },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, test.option)
			transport, ok := client.UnsafeHTTPClient().Transport.(*http.Transport)
			require.True(t, ok)

			want := baselineValues
			if test.adjust != nil {
				test.adjust(&want)
			}
			if diff := cmp.Diff(want, transportDefaultsFrom(transport), cmp.AllowUnexported(transportDefaults{})); diff != "" {
				t.Errorf("transport defaults mismatch (-want +got):\n%s", diff)
			}
			if test.check != nil {
				test.check(t, transport)
			}
		})
	}
}

type transportDefaults struct {
	maxIdleConns          int
	maxIdleConnsPerHost   int
	maxConnsPerHost       int
	idleConnTimeout       time.Duration
	tlsHandshakeTimeout   time.Duration
	responseHeaderTimeout time.Duration
	expectContinueTimeout time.Duration
	forceAttemptHTTP2     bool
}

func transportDefaultsFrom(transport *http.Transport) transportDefaults {
	return transportDefaults{
		maxIdleConns:          transport.MaxIdleConns,
		maxIdleConnsPerHost:   transport.MaxIdleConnsPerHost,
		maxConnsPerHost:       transport.MaxConnsPerHost,
		idleConnTimeout:       transport.IdleConnTimeout,
		tlsHandshakeTimeout:   transport.TLSHandshakeTimeout,
		responseHeaderTimeout: transport.ResponseHeaderTimeout,
		expectContinueTimeout: transport.ExpectContinueTimeout,
		forceAttemptHTTP2:     transport.ForceAttemptHTTP2,
	}
}

func TestNew_WithoutProxyMaterializesDirectTransport(t *testing.T) {
	client := newTestClient(t, WithoutProxy())

	transport, ok := client.UnsafeHTTPClient().Transport.(*http.Transport)
	require.True(t, ok)
	assert.Nil(t, transport.Proxy)
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
		WithJSONEncoder(&JSONEncoder{}),
		WithJSONDecoder(&JSONDecoder{}),
		WithXMLEncoder(&XMLEncoder{}),
		WithXMLDecoder(&XMLDecoder{}),
		WithYAMLEncoder(&YAMLEncoder{}),
		WithYAMLDecoder(&YAMLDecoder{}),
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

	require.NoError(t, c.enableSession())

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
		assert.NotSame(t, tlsConfig, transport.TLSClientConfig)
		assert.Equal(t, "example.com", transport.TLSClientConfig.ServerName)
		assertHTTP2Configured(t, transport)
	})

	t.Run("TLS config after HTTP2", func(t *testing.T) {
		tlsConfig := &tls.Config{ServerName: "example.com"}
		c := newTestClient(t, WithHTTP2(), WithTLSConfig(tlsConfig))

		transport, ok := c.httpClient.Transport.(*http.Transport)
		require.True(t, ok)
		assert.NotSame(t, tlsConfig, transport.TLSClientConfig)
		assert.Equal(t, "example.com", transport.TLSClientConfig.ServerName)
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
