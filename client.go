package requests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/kaptinlin/orderedobject"
	"golang.org/x/net/http2"
	"golang.org/x/net/publicsuffix"
)

// Client represents an HTTP client.
type Client struct {
	mu             sync.RWMutex
	baseURL        string
	headers        *http.Header
	orderedHeaders *orderedobject.Object[[]string]
	cookies        []*http.Cookie
	middlewares    []Middleware
	tlsConfig      *tls.Config
	retry          RetryPolicy
	httpClient     *http.Client
	jsonEncoder    Encoder
	jsonDecoder    Decoder
	xmlEncoder     Encoder
	xmlDecoder     Decoder
	yamlEncoder    Encoder
	yamlDecoder    Decoder
	logger         Logger
	dialTimeout    time.Duration
	resolver       *net.Resolver
	localAddr      net.Addr
	dialContext    func(context.Context, string, string) (net.Conn, error)
	auth           AuthMethod
}

type clientSnapshot struct {
	baseURL        string
	headers        http.Header
	orderedHeaders *orderedobject.Object[[]string]
	cookies        []*http.Cookie
	middlewares    []Middleware
	retry          RetryPolicy
	httpClient     *http.Client
	jsonEncoder    Encoder
	jsonDecoder    Decoder
	xmlEncoder     Encoder
	xmlDecoder     Decoder
	yamlEncoder    Encoder
	yamlDecoder    Decoder
	logger         Logger
	auth           AuthMethod
}

// New creates a Client with functional options applied.
// It returns an error when any option cannot be applied.
func New(opts ...Option) (*Client, error) {
	c := newClient()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// Clone returns a new Client with the current defaults plus opts applied.
func (c *Client) Clone(opts ...Option) (*Client, error) {
	clone := c.clone()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(clone); err != nil {
			return nil, err
		}
	}
	return clone, nil
}

func newClient() *Client {
	return &Client{
		httpClient:  &http.Client{},
		jsonEncoder: DefaultJSONEncoder,
		jsonDecoder: DefaultJSONDecoder,
		xmlEncoder:  DefaultXMLEncoder,
		xmlDecoder:  DefaultXMLDecoder,
		yamlEncoder: DefaultYAMLEncoder,
		yamlDecoder: DefaultYAMLDecoder,
		retry:       DefaultRetryPolicy(),
	}
}

func (c *Client) clone() *Client {
	c.mu.RLock()
	defer c.mu.RUnlock()

	clone := &Client{
		baseURL:        c.baseURL,
		headers:        cloneHeaderPtr(c.headers),
		orderedHeaders: cloneOrderedHeaders(c.orderedHeaders),
		cookies:        cloneCookies(c.cookies),
		middlewares:    slices.Clone(c.middlewares),
		tlsConfig:      cloneTLSConfig(c.tlsConfig),
		retry:          c.retry,
		httpClient:     nil,
		jsonEncoder:    c.jsonEncoder,
		jsonDecoder:    c.jsonDecoder,
		xmlEncoder:     c.xmlEncoder,
		xmlDecoder:     c.xmlDecoder,
		yamlEncoder:    c.yamlEncoder,
		yamlDecoder:    c.yamlDecoder,
		logger:         c.logger,
		dialTimeout:    c.dialTimeout,
		resolver:       c.resolver,
		localAddr:      c.localAddr,
		dialContext:    c.dialContext,
		auth:           c.auth,
	}
	clone.httpClient = cloneHTTPClient(c.httpClient, clone.tlsConfig)
	return clone
}

func cloneHeaderPtr(headers *http.Header) *http.Header {
	if headers == nil {
		return nil
	}
	clone := headers.Clone()
	return &clone
}

func cloneCookies(cookies []*http.Cookie) []*http.Cookie {
	if len(cookies) == 0 {
		return nil
	}
	clones := make([]*http.Cookie, len(cookies))
	for i, cookie := range cookies {
		if cookie == nil {
			continue
		}
		clone := new(*cookie) //nolint:gosec // clone preserves caller-provided cookie attributes
		clone.Unparsed = slices.Clone(cookie.Unparsed)
		clones[i] = clone
	}
	return clones
}

func cloneTLSConfig(config *tls.Config) *tls.Config {
	if config == nil {
		return nil
	}
	return config.Clone()
}

func cloneHTTPClient(client *http.Client, tlsConfig *tls.Config) *http.Client {
	if client == nil {
		return &http.Client{}
	}
	clone := *client
	if transport, ok := client.Transport.(*http.Transport); ok {
		clonedTransport := transport.Clone()
		if tlsConfig != nil {
			clonedTransport.TLSClientConfig = tlsConfig
		}
		clone.Transport = clonedTransport
	}
	return &clone
}

// setBaseURL sets the base URL.
func (c *Client) setBaseURL(baseURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.baseURL = baseURL
}

// addMiddleware appends client-level middleware.
func (c *Client) addMiddleware(middlewares ...Middleware) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.middlewares = append(c.middlewares, middlewares...)
}

func (c *Client) syncTLSConfigLocked() {
	if c.httpClient == nil {
		c.httpClient = &http.Client{}
	}
	if c.tlsConfig == nil {
		return
	}
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.TLSClientConfig = c.tlsConfig
		if isHTTP2Configured(transport) {
			ensureHTTP2NextProtos(transport)
			transport.ForceAttemptHTTP2 = true
		}
		return
	}
	if transport, ok := c.httpClient.Transport.(*http2.Transport); ok {
		transport.TLSClientConfig = c.tlsConfig
		return
	}
	c.httpClient.Transport = &http.Transport{TLSClientConfig: c.tlsConfig}
}

// setTLSConfig replaces the TLS configuration.
func (c *Client) setTLSConfig(config *tls.Config) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tlsConfig = config
	c.syncTLSConfigLocked()
	return c
}

// ensureTLSConfig initializes the TLS configuration if nil.
// Must be called with c.mu held.
func (c *Client) ensureTLSConfig() {
	if c.tlsConfig == nil {
		c.tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}
}

// InsecureSkipVerify sets the TLS configuration to skip certificate verification.
func (c *Client) insecureSkipVerify() *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureTLSConfig()
	c.tlsConfig.InsecureSkipVerify = true
	c.syncTLSConfigLocked()
	return c
}

// setCertificates replaces the TLS client certificates.
func (c *Client) setCertificates(certs ...tls.Certificate) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureTLSConfig()
	c.tlsConfig.Certificates = certs
	c.syncTLSConfigLocked()
	return c
}

// setTLSServerName sets the TLS server name (SNI).
func (c *Client) setTLSServerName(serverName string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureTLSConfig()
	c.tlsConfig.ServerName = serverName
	c.syncTLSConfigLocked()
	return c
}

// setRootCertificate loads root certificates from a PEM file.
func (c *Client) setRootCertificate(pemFilePath string) *Client {
	cleanPath := filepath.Clean(pemFilePath)
	rootPemData, err := os.ReadFile(cleanPath)
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("failed to read root certificate: %v", err)
		}
		return c
	}
	return c.addRootCAs(rootPemData)
}

// setRootCertificateFromString loads root certificates from PEM text.
func (c *Client) setRootCertificateFromString(pemCerts string) *Client {
	return c.addRootCAs([]byte(pemCerts))
}

func (c *Client) addRootCAs(pemCerts []byte) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureTLSConfig()
	if c.tlsConfig.RootCAs == nil {
		c.tlsConfig.RootCAs = x509.NewCertPool()
	}
	c.tlsConfig.RootCAs.AppendCertsFromPEM(pemCerts)
	c.syncTLSConfigLocked()
	return c
}

// setHTTPClient replaces the underlying HTTP client.
func (c *Client) setHTTPClient(httpClient *http.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.httpClient = httpClient
}

// setDefaultHeaders replaces the default semantic headers.
func (c *Client) setDefaultHeaders(headers *http.Header) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.headers = headers
	c.orderedHeaders = nil
}

// setDefaultOrderedHeaders replaces ordered default headers.
func (c *Client) setDefaultOrderedHeaders(headers *orderedobject.Object[[]string]) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.orderedHeaders = cloneOrderedHeaders(headers)
	if c.orderedHeaders == nil {
		c.headers = nil
		return
	}
	c.headers = new(headerFromOrderedHeaders(c.orderedHeaders))
}

// setDefaultHeader adds or updates a default header.
func (c *Client) setDefaultHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.headers == nil {
		c.headers = &http.Header{}
	}
	c.headers.Set(key, value)
	if c.orderedHeaders != nil {
		setOrderedHeaderValues(&c.orderedHeaders, key, []string{value})
	}
}

// addDefaultHeader adds a default header value.
func (c *Client) addDefaultHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.headers == nil {
		c.headers = &http.Header{}
	}
	c.headers.Add(key, value)
	if c.orderedHeaders != nil {
		addOrderedHeaderValue(&c.orderedHeaders, key, value)
	}
}

// delDefaultHeader removes a default header.
func (c *Client) delDefaultHeader(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.headers == nil {
		return
	}
	c.headers.Del(key)
	if c.orderedHeaders != nil {
		deleteOrderedHeader(c.orderedHeaders, key)
	}
}

// setDefaultContentType sets the default content type.
func (c *Client) setDefaultContentType(contentType string) {
	c.setDefaultHeader("Content-Type", contentType)
}

// setDefaultAccept sets the default Accept header.
func (c *Client) setDefaultAccept(accept string) {
	c.setDefaultHeader("Accept", accept)
}

// setDefaultUserAgent sets the default User-Agent header.
func (c *Client) setDefaultUserAgent(userAgent string) {
	c.setDefaultHeader("User-Agent", userAgent)
}

// setDefaultReferer sets the default Referer header.
func (c *Client) setDefaultReferer(referer string) {
	c.setDefaultHeader("Referer", referer)
}

// setDefaultTimeout sets the underlying http.Client timeout.
func (c *Client) setDefaultTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.httpClient.Timeout = timeout
}

// setDefaultTransport replaces the underlying transport.
func (c *Client) setDefaultTransport(transport http.RoundTripper) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.httpClient.Transport = transport
}

// setDefaultCookieJar replaces the underlying cookie jar.
func (c *Client) setDefaultCookieJar(jar *cookiejar.Jar) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.httpClient.Jar = jar
}

// enableSession enables cookie and TLS session reuse without replacing existing session stores.
func (c *Client) enableSession() *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.httpClient == nil {
		c.httpClient = &http.Client{}
	}
	if c.httpClient.Jar == nil {
		jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err == nil {
			c.httpClient.Jar = jar
		} else if c.logger != nil {
			c.logger.Errorf("failed to create cookie jar: %v", err)
		}
	}

	c.ensureTLSConfig()
	if c.tlsConfig.ClientSessionCache == nil {
		c.tlsConfig.ClientSessionCache = tls.NewLRUClientSessionCache(0)
	}
	switch c.httpClient.Transport.(type) {
	case nil, *http.Transport, *http2.Transport:
		c.syncTLSConfigLocked()
	}
	return c
}

// setDefaultCookies adds default cookies.
func (c *Client) setDefaultCookies(cookies map[string]string) {
	for name, value := range cookies {
		c.setDefaultCookie(name, value)
	}
}

// setDefaultCookie adds a default cookie.
func (c *Client) setDefaultCookie(name, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cookies = append(c.cookies, &http.Cookie{Name: name, Value: value}) //nolint:gosec // callers control default cookie attributes
}

// delDefaultCookie removes a default cookie.
func (c *Client) delDefaultCookie(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cookies == nil {
		return
	}

	c.cookies = slices.DeleteFunc(c.cookies, func(cookie *http.Cookie) bool {
		return cookie.Name == name
	})
}

func (c *Client) snapshot() clientSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	headers := http.Header{}
	if c.headers != nil {
		headers = c.headers.Clone()
	}

	cookies := make([]*http.Cookie, len(c.cookies))
	for i, cookie := range c.cookies {
		if cookie == nil {
			continue
		}
		clone := new(*cookie) //nolint:gosec // snapshot preserves caller-provided cookie attributes
		clone.Unparsed = slices.Clone(cookie.Unparsed)
		cookies[i] = clone
	}

	middlewares := slices.Clone(c.middlewares)

	return clientSnapshot{
		baseURL:        c.baseURL,
		headers:        headers,
		orderedHeaders: cloneOrderedHeaders(c.orderedHeaders),
		cookies:        cookies,
		middlewares:    middlewares,
		retry:          c.retry,
		httpClient:     c.httpClient,
		jsonEncoder:    c.jsonEncoder,
		jsonDecoder:    c.jsonDecoder,
		xmlEncoder:     c.xmlEncoder,
		xmlDecoder:     c.xmlDecoder,
		yamlEncoder:    c.yamlEncoder,
		yamlDecoder:    c.yamlDecoder,
		logger:         c.logger,
		auth:           c.auth,
	}
}

// GetHTTPClient returns the underlying HTTP client in a thread-safe way.
func (c *Client) GetHTTPClient() *http.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.httpClient
}

// GetBaseURL returns the configured base URL in a thread-safe way.
func (c *Client) GetBaseURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL
}

// GetTLSConfig returns a clone of the configured TLS settings.
func (c *Client) GetTLSConfig() *tls.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.tlsConfig == nil {
		return nil
	}
	return c.tlsConfig.Clone()
}

// enableHTTP2 enables HTTP/2 on the underlying HTTP transport.
func (c *Client) enableHTTP2() *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.enableHTTP2Locked()
	return c
}

func (c *Client) enableHTTP2Locked() {
	if c.httpClient == nil {
		c.httpClient = &http.Client{}
	}
	transport, err := c.ensureTransport()
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("failed to enable HTTP/2: %v", err)
		}
		return
	}
	if c.tlsConfig != nil {
		transport.TLSClientConfig = c.tlsConfig
	}
	if err := configureHTTP2Transport(transport); err != nil && c.logger != nil {
		c.logger.Errorf("failed to enable HTTP/2: %v", err)
	}
}

func configureHTTP2Transport(transport *http.Transport) error {
	if transport == nil {
		return nil
	}
	if isHTTP2Configured(transport) {
		ensureHTTP2NextProtos(transport)
		transport.ForceAttemptHTTP2 = true
		return nil
	}

	transport.ForceAttemptHTTP2 = true
	return http2.ConfigureTransport(transport)
}

func isHTTP2Configured(transport *http.Transport) bool {
	if transport == nil || transport.TLSNextProto == nil {
		return false
	}
	_, ok := transport.TLSNextProto[http2.NextProtoTLS]
	return ok
}

func ensureHTTP2NextProtos(transport *http.Transport) {
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	}
	if !slices.Contains(transport.TLSClientConfig.NextProtos, http2.NextProtoTLS) {
		transport.TLSClientConfig.NextProtos = slices.Concat(
			[]string{http2.NextProtoTLS},
			transport.TLSClientConfig.NextProtos,
		)
	}
	if !slices.Contains(transport.TLSClientConfig.NextProtos, "http/1.1") {
		transport.TLSClientConfig.NextProtos = append(transport.TLSClientConfig.NextProtos, "http/1.1")
	}
}

func (c *Client) setRetry(policy RetryPolicy) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.retry = policy
	return c
}

// setAuth configures a client-level authentication method.
func (c *Client) setAuth(auth AuthMethod) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if auth.Valid() {
		c.auth = auth
	}
}

// setRedirectPolicy replaces the redirect policy chain.
func (c *Client) setRedirectPolicy(policies ...RedirectPolicy) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		for _, p := range policies {
			if err := p.Apply(req, via); err != nil {
				return err
			}
		}
		return nil
	}
	return c
}

// setLogger sets the client logger.
func (c *Client) setLogger(logger Logger) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger = logger
	return c
}

// withTransport executes a function on the client's transport, handling locking and error checking.
// Returns the client for method chaining. Errors from ensureTransport are silently ignored to
// maintain the fluent API pattern.
func (c *Client) withTransport(fn func(*http.Transport)) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	transport, err := c.ensureTransport()
	if err != nil {
		return c
	}
	fn(transport)
	return c
}

func (c *Client) applyDialContextLocked(transport *http.Transport) {
	if c.dialContext != nil {
		transport.DialContext = c.dialContext
		return
	}
	if c.dialTimeout == 0 && c.resolver == nil && c.localAddr == nil {
		transport.DialContext = nil
		return
	}
	dialer := &net.Dialer{
		Timeout:   c.dialTimeout,
		Resolver:  c.resolver,
		LocalAddr: c.localAddr,
	}
	transport.DialContext = dialer.DialContext
}

// setDialTimeout sets the TCP connection timeout on the underlying transport.
func (c *Client) setDialTimeout(d time.Duration) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dialTimeout = d
	transport, err := c.ensureTransport()
	if err != nil {
		return c
	}
	c.applyDialContextLocked(transport)
	return c
}

// setResolver sets the resolver used by the default transport dialer.
func (c *Client) setResolver(resolver *net.Resolver) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.resolver = resolver
	transport, err := c.ensureTransport()
	if err != nil {
		return c
	}
	c.applyDialContextLocked(transport)
	return c
}

// setDialContext sets the dial function on the underlying transport.
func (c *Client) setDialContext(dial func(context.Context, string, string) (net.Conn, error)) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dialContext = dial
	transport, err := c.ensureTransport()
	if err != nil {
		return c
	}
	c.applyDialContextLocked(transport)
	return c
}

// setLocalAddr sets the local address used by the default transport dialer.
func (c *Client) setLocalAddr(addr net.Addr) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.localAddr = addr
	transport, err := c.ensureTransport()
	if err != nil {
		return c
	}
	c.applyDialContextLocked(transport)
	return c
}

// setTLSHandshakeTimeout sets the TLS handshake timeout on the underlying transport.
func (c *Client) setTLSHandshakeTimeout(d time.Duration) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.TLSHandshakeTimeout = d
	})
}

// setResponseHeaderTimeout sets the time to wait for response headers after the request
// is sent. This does not include the time to read the response body.
func (c *Client) setResponseHeaderTimeout(d time.Duration) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.ResponseHeaderTimeout = d
	})
}

// setMaxIdleConns sets the maximum number of idle connections across all hosts.
func (c *Client) setMaxIdleConns(n int) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.MaxIdleConns = n
	})
}

// setMaxIdleConnsPerHost sets the maximum number of idle connections per host.
func (c *Client) setMaxIdleConnsPerHost(n int) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.MaxIdleConnsPerHost = n
	})
}

// setMaxConnsPerHost sets the maximum total number of connections per host.
func (c *Client) setMaxConnsPerHost(n int) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.MaxConnsPerHost = n
	})
}

// setIdleConnTimeout sets how long idle connections remain in the pool before being closed.
func (c *Client) setIdleConnTimeout(d time.Duration) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.IdleConnTimeout = d
	})
}

// Get initiates a GET request.
func (c *Client) Get(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodGet, path)
}

// Post initiates a POST request.
func (c *Client) Post(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodPost, path)
}

// Delete initiates a DELETE request.
func (c *Client) Delete(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodDelete, path)
}

// Put initiates a PUT request.
func (c *Client) Put(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodPut, path)
}

// Patch initiates a PATCH request.
func (c *Client) Patch(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodPatch, path)
}

// Options initiates an OPTIONS request.
func (c *Client) Options(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodOptions, path)
}

// Head initiates a HEAD request.
func (c *Client) Head(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodHead, path)
}

// Connect initiates a CONNECT request.
func (c *Client) Connect(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodConnect, path)
}

// Trace initiates a TRACE request.
func (c *Client) Trace(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodTrace, path)
}

// Custom initiates a custom request.
func (c *Client) Custom(path, method string) *RequestBuilder {
	return c.NewRequestBuilder(method, path)
}
