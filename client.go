package requests

import (
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

	"golang.org/x/net/http2"
)

// Client represents an HTTP client.
type Client struct {
	mu            sync.RWMutex
	BaseURL       string
	Headers       *http.Header
	Cookies       []*http.Cookie
	Middlewares   []Middleware
	TLSConfig     *tls.Config
	MaxRetries    int             // Maximum number of retry attempts
	RetryStrategy BackoffStrategy // The backoff strategy function
	RetryIf       RetryIfFunc     // Custom function to determine retry based on request and response
	HTTPClient    *http.Client
	JSONEncoder   Encoder
	JSONDecoder   Decoder
	XMLEncoder    Encoder
	XMLDecoder    Decoder
	YAMLEncoder   Encoder
	YAMLDecoder   Decoder
	Logger        Logger
	auth          AuthMethod
}

// Config sets up the initial configuration for the HTTP client.
type Config struct {
	BaseURL       string            // The base URL for all requests made by this client.
	Headers       *http.Header      // Default headers to be sent with each request.
	Cookies       map[string]string // Default Cookies to be sent with each request.
	Timeout       time.Duration     // Timeout for requests.
	CookieJar     *cookiejar.Jar    // Cookie jar for the client.
	Middlewares   []Middleware      // Middleware stack for request/response manipulation.
	TLSConfig     *tls.Config       // TLS configuration for the client.
	Transport     http.RoundTripper // Custom transport for the client.
	MaxRetries    int               // Maximum number of retry attempts
	RetryStrategy BackoffStrategy   // The backoff strategy function
	RetryIf       RetryIfFunc       // Custom function to determine retry based on request and response
	Logger        Logger            // Logger instance for the client
	HTTP2         bool              // Whether to use HTTP/2. Transport takes priority over HTTP2 if both are set.

	// Transport-level timeouts (applied to http.Transport)
	DialTimeout           time.Duration // TCP connection timeout
	TLSHandshakeTimeout   time.Duration // TLS handshake timeout
	ResponseHeaderTimeout time.Duration // Time to first response byte

	// Connection pool settings (applied to http.Transport)
	MaxIdleConns        int           // Max idle connections across all hosts (0 = default 100)
	MaxIdleConnsPerHost int           // Max idle connections per host (0 = default 2)
	MaxConnsPerHost     int           // Max total connections per host (0 = no limit)
	IdleConnTimeout     time.Duration // How long idle connections live (0 = default 90s)
}

// hasTransportConfig checks if any transport-level configuration is set.
func (cfg *Config) hasTransportConfig() bool {
	return cfg.DialTimeout > 0 || cfg.TLSHandshakeTimeout > 0 ||
		cfg.ResponseHeaderTimeout > 0 || cfg.MaxIdleConns > 0 ||
		cfg.MaxIdleConnsPerHost > 0 || cfg.MaxConnsPerHost > 0 ||
		cfg.IdleConnTimeout > 0
}

// URL creates a new HTTP client with the given base URL.
func URL(baseURL string) *Client {
	return Create(&Config{BaseURL: baseURL})
}

// Create initializes a new HTTP client with the given configuration.
func Create(config *Config) *Client {
	if config == nil {
		config = &Config{}
	}

	httpClient := &http.Client{}

	if config.Transport != nil {
		httpClient.Transport = config.Transport
	}

	if config.Timeout != 0 {
		httpClient.Timeout = config.Timeout
	}

	if config.CookieJar != nil {
		httpClient.Jar = config.CookieJar
	}

	// Return a new Client instance.
	client := &Client{
		BaseURL:     config.BaseURL,
		Headers:     config.Headers,
		HTTPClient:  httpClient,
		JSONEncoder: DefaultJSONEncoder,
		JSONDecoder: DefaultJSONDecoder,
		XMLEncoder:  DefaultXMLEncoder,
		XMLDecoder:  DefaultXMLDecoder,
		YAMLEncoder: DefaultYAMLEncoder,
		YAMLDecoder: DefaultYAMLDecoder,
		TLSConfig:   config.TLSConfig,
	}

	// Configure Transport, handle both TLS and HTTP/2
	switch {
	case client.TLSConfig != nil && config.HTTP2:
		client.HTTPClient.Transport = &http2.Transport{
			TLSClientConfig: client.TLSConfig,
		}
	case client.TLSConfig != nil:
		if httpClient.Transport != nil {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				transport.TLSClientConfig = client.TLSConfig
			}
		} else {
			client.HTTPClient.Transport = &http.Transport{
				TLSClientConfig: client.TLSConfig,
			}
		}
	case config.HTTP2:
		client.HTTPClient.Transport = &http2.Transport{}
	}

	// Apply transport-level timeouts and connection pool settings
	applyTransportConfig(client, config)

	if config.Middlewares != nil {
		client.Middlewares = config.Middlewares
	}

	if config.Cookies != nil {
		client.SetDefaultCookies(config.Cookies)
	}

	if config.MaxRetries != 0 {
		client.MaxRetries = config.MaxRetries
	}

	if config.RetryStrategy != nil {
		client.RetryStrategy = config.RetryStrategy
	} else {
		client.RetryStrategy = DefaultBackoffStrategy(1 * time.Second)
	}

	if config.RetryIf != nil {
		client.RetryIf = config.RetryIf
	} else {
		client.RetryIf = DefaultRetryIf
	}

	if config.Logger != nil {
		client.Logger = config.Logger
	}

	return client
}

// SetBaseURL sets the base URL for the client.
func (c *Client) SetBaseURL(baseURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.BaseURL = baseURL
}

// AddMiddleware adds a middleware to the client.
func (c *Client) AddMiddleware(middlewares ...Middleware) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Middlewares = append(c.Middlewares, middlewares...)
}

// SetTLSConfig sets the TLS configuration for the client.
func (c *Client) SetTLSConfig(config *tls.Config) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.TLSConfig = config

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}

	// Apply the TLS configuration to the existing transport if possible.
	// If the current transport is not an *http.Transport, replace it.
	if transport, ok := c.HTTPClient.Transport.(*http.Transport); ok {
		transport.TLSClientConfig = config
	} else {
		c.HTTPClient.Transport = &http.Transport{
			TLSClientConfig: config,
		}
	}

	return c
}

// ensureTLSConfig initializes the TLS configuration if nil.
// Must be called with c.mu held.
func (c *Client) ensureTLSConfig() {
	if c.TLSConfig == nil {
		c.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}
}

// InsecureSkipVerify sets the TLS configuration to skip certificate verification.
func (c *Client) InsecureSkipVerify() *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureTLSConfig()
	c.TLSConfig.InsecureSkipVerify = true

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}
	if transport, ok := c.HTTPClient.Transport.(*http.Transport); ok {
		transport.TLSClientConfig = c.TLSConfig
	} else {
		c.HTTPClient.Transport = &http.Transport{
			TLSClientConfig: c.TLSConfig,
		}
	}

	return c
}

// SetCertificates sets the TLS certificates for the client.
func (c *Client) SetCertificates(certs ...tls.Certificate) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureTLSConfig()
	c.TLSConfig.Certificates = certs
	return c
}

// SetRootCertificate sets the root certificate for the client.
func (c *Client) SetRootCertificate(pemFilePath string) *Client {
	cleanPath := filepath.Clean(pemFilePath)
	rootPemData, err := os.ReadFile(cleanPath)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Errorf("failed to read root certificate: %v", err)
		}
		return c
	}
	return c.handleCAs("root", rootPemData)
}

// SetRootCertificateFromString sets the root certificate for the client from a string.
func (c *Client) SetRootCertificateFromString(pemCerts string) *Client {
	return c.handleCAs("root", []byte(pemCerts))
}

// SetClientRootCertificate sets the client root certificate for the client.
func (c *Client) SetClientRootCertificate(pemFilePath string) *Client {
	cleanPath := filepath.Clean(pemFilePath)
	rootPemData, err := os.ReadFile(cleanPath)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Errorf("failed to read client root certificate: %v", err)
		}
		return c
	}
	return c.handleCAs("client", rootPemData)
}

// SetClientRootCertificateFromString sets the client root certificate for the client from a string.
func (c *Client) SetClientRootCertificateFromString(pemCerts string) *Client {
	return c.handleCAs("client", []byte(pemCerts))
}

// handleCAs sets the TLS certificates for the client.
func (c *Client) handleCAs(scope string, permCerts []byte) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureTLSConfig()
	switch scope {
	case "root":
		if c.TLSConfig.RootCAs == nil {
			c.TLSConfig.RootCAs = x509.NewCertPool()
		}
		c.TLSConfig.RootCAs.AppendCertsFromPEM(permCerts)
	case "client":
		if c.TLSConfig.ClientCAs == nil {
			c.TLSConfig.ClientCAs = x509.NewCertPool()
		}
		c.TLSConfig.ClientCAs.AppendCertsFromPEM(permCerts)
	}
	return c
}

// SetHTTPClient sets the HTTP client for the client.
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.HTTPClient = httpClient
}

// SetDefaultHeaders sets the default headers for the client.
func (c *Client) SetDefaultHeaders(headers *http.Header) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Headers = headers
}

// SetDefaultHeader adds or updates a default header.
func (c *Client) SetDefaultHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Headers == nil {
		c.Headers = &http.Header{}
	}
	c.Headers.Set(key, value)
}

// AddDefaultHeader adds a default header.
func (c *Client) AddDefaultHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Headers == nil {
		c.Headers = &http.Header{}
	}
	c.Headers.Add(key, value)
}

// DelDefaultHeader removes a default header.
func (c *Client) DelDefaultHeader(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Headers == nil {
		return
	}
	c.Headers.Del(key)
}

// SetDefaultContentType sets the default content type for the client.
func (c *Client) SetDefaultContentType(contentType string) {
	c.SetDefaultHeader("Content-Type", contentType)
}

// SetDefaultAccept sets the default accept header for the client.
func (c *Client) SetDefaultAccept(accept string) {
	c.SetDefaultHeader("Accept", accept)
}

// SetDefaultUserAgent sets the default user agent for the client.
func (c *Client) SetDefaultUserAgent(userAgent string) {
	c.SetDefaultHeader("User-Agent", userAgent)
}

// SetDefaultReferer sets the default referer for the client.
func (c *Client) SetDefaultReferer(referer string) {
	c.SetDefaultHeader("Referer", referer)
}

// SetDefaultTimeout sets the default timeout for the client.
func (c *Client) SetDefaultTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.HTTPClient.Timeout = timeout
}

// SetDefaultTransport sets the default transport for the client.
func (c *Client) SetDefaultTransport(transport http.RoundTripper) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.HTTPClient.Transport = transport
}

// SetDefaultCookieJar sets the default cookie jar for the client.
func (c *Client) SetDefaultCookieJar(jar *cookiejar.Jar) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.HTTPClient.Jar = jar
}

// SetDefaultCookies sets the default cookies for the client.
func (c *Client) SetDefaultCookies(cookies map[string]string) {
	for name, value := range cookies {
		c.SetDefaultCookie(name, value)
	}
}

// SetDefaultCookie sets a default cookie for the client.
func (c *Client) SetDefaultCookie(name, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Cookies = append(c.Cookies, &http.Cookie{Name: name, Value: value})
}

// DelDefaultCookie removes a default cookie from the client.
func (c *Client) DelDefaultCookie(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Cookies == nil {
		return
	}

	c.Cookies = slices.DeleteFunc(c.Cookies, func(cookie *http.Cookie) bool {
		return cookie.Name == name
	})
}

// SetJSONMarshal sets the JSON marshal function for the client's JSONEncoder.
func (c *Client) SetJSONMarshal(marshalFunc func(v any) ([]byte, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.JSONEncoder = &JSONEncoder{
		MarshalFunc: marshalFunc,
	}
}

// SetJSONUnmarshal sets the JSON unmarshal function for the client's JSONDecoder.
func (c *Client) SetJSONUnmarshal(unmarshalFunc func(data []byte, v any) error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.JSONDecoder = &JSONDecoder{
		UnmarshalFunc: unmarshalFunc,
	}
}

// SetXMLMarshal sets the XML marshal function for the client's XMLEncoder.
func (c *Client) SetXMLMarshal(marshalFunc func(v any) ([]byte, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.XMLEncoder = &XMLEncoder{
		MarshalFunc: marshalFunc,
	}
}

// SetXMLUnmarshal sets the XML unmarshal function for the client's XMLDecoder.
func (c *Client) SetXMLUnmarshal(unmarshalFunc func(data []byte, v any) error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.XMLDecoder = &XMLDecoder{
		UnmarshalFunc: unmarshalFunc,
	}
}

// SetYAMLMarshal sets the YAML marshal function for the client's YAMLEncoder.
func (c *Client) SetYAMLMarshal(marshalFunc func(v any) ([]byte, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.YAMLEncoder = &YAMLEncoder{
		MarshalFunc: marshalFunc,
	}
}

// SetYAMLUnmarshal sets the YAML unmarshal function for the client's YAMLDecoder.
func (c *Client) SetYAMLUnmarshal(unmarshalFunc func(data []byte, v any) error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.YAMLDecoder = &YAMLDecoder{
		UnmarshalFunc: unmarshalFunc,
	}
}

// SetMaxRetries sets the maximum number of retry attempts.
func (c *Client) SetMaxRetries(maxRetries int) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.MaxRetries = maxRetries
	return c
}

// SetRetryStrategy sets the backoff strategy for retries.
func (c *Client) SetRetryStrategy(strategy BackoffStrategy) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.RetryStrategy = strategy
	return c
}

// SetRetryIf sets the custom retry condition function.
func (c *Client) SetRetryIf(retryIf RetryIfFunc) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.RetryIf = retryIf
	return c
}

// SetAuth configures an authentication method for the client.
func (c *Client) SetAuth(auth AuthMethod) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if auth.Valid() {
		c.auth = auth
	}
}

// SetRedirectPolicy sets the redirect policy for the client.
func (c *Client) SetRedirectPolicy(policies ...RedirectPolicy) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.HTTPClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		for _, p := range policies {
			if err := p.Apply(req, via); err != nil {
				return err
			}
		}
		return nil
	}
	return c
}

// SetLogger sets logger instance in client.
func (c *Client) SetLogger(logger Logger) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Logger = logger
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

// SetDialTimeout sets the TCP connection timeout on the underlying transport.
func (c *Client) SetDialTimeout(d time.Duration) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.DialContext = (&net.Dialer{Timeout: d}).DialContext
	})
}

// SetTLSHandshakeTimeout sets the TLS handshake timeout on the underlying transport.
func (c *Client) SetTLSHandshakeTimeout(d time.Duration) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.TLSHandshakeTimeout = d
	})
}

// SetResponseHeaderTimeout sets the time to wait for response headers after the request
// is sent. This does not include the time to read the response body.
func (c *Client) SetResponseHeaderTimeout(d time.Duration) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.ResponseHeaderTimeout = d
	})
}

// SetMaxIdleConns sets the maximum number of idle connections across all hosts.
func (c *Client) SetMaxIdleConns(n int) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.MaxIdleConns = n
	})
}

// SetMaxIdleConnsPerHost sets the maximum number of idle connections per host.
func (c *Client) SetMaxIdleConnsPerHost(n int) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.MaxIdleConnsPerHost = n
	})
}

// SetMaxConnsPerHost sets the maximum total number of connections per host.
func (c *Client) SetMaxConnsPerHost(n int) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.MaxConnsPerHost = n
	})
}

// SetIdleConnTimeout sets how long idle connections remain in the pool before being closed.
func (c *Client) SetIdleConnTimeout(d time.Duration) *Client {
	return c.withTransport(func(t *http.Transport) {
		t.IdleConnTimeout = d
	})
}

// applyTransportConfig applies transport-level timeouts and connection pool settings
// from Config to the client's transport. Only modifies settings that are explicitly set
// (non-zero). Skips if the transport is not *http.Transport (e.g., HTTP/2 transport).
func applyTransportConfig(c *Client, config *Config) {
	transport, ok := c.HTTPClient.Transport.(*http.Transport)
	if !ok {
		// No *http.Transport available (nil or HTTP/2); create one if any config is set
		if !config.hasTransportConfig() {
			return
		}

		if c.HTTPClient.Transport == nil {
			transport = &http.Transport{}
			c.HTTPClient.Transport = transport
		} else {
			return // Non-nil, non-http.Transport (e.g., http2.Transport): skip
		}
	}

	if config.DialTimeout > 0 {
		transport.DialContext = (&net.Dialer{Timeout: config.DialTimeout}).DialContext
	}
	if config.TLSHandshakeTimeout > 0 {
		transport.TLSHandshakeTimeout = config.TLSHandshakeTimeout
	}
	if config.ResponseHeaderTimeout > 0 {
		transport.ResponseHeaderTimeout = config.ResponseHeaderTimeout
	}
	if config.MaxIdleConns > 0 {
		transport.MaxIdleConns = config.MaxIdleConns
	}
	if config.MaxIdleConnsPerHost > 0 {
		transport.MaxIdleConnsPerHost = config.MaxIdleConnsPerHost
	}
	if config.MaxConnsPerHost > 0 {
		transport.MaxConnsPerHost = config.MaxConnsPerHost
	}
	if config.IdleConnTimeout > 0 {
		transport.IdleConnTimeout = config.IdleConnTimeout
	}
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
