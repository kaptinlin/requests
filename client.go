package requests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
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
	BaseURL        string                          // BaseURL is prepended to relative request paths.
	Headers        *http.Header                    // Headers contains the default headers sent with each request.
	OrderedHeaders *orderedobject.Object[[]string] // OrderedHeaders contains ordered default headers.
	Cookies        []*http.Cookie                  // Cookies contains the default cookies sent with each request.
	Middlewares    []Middleware                    // Middlewares contains the client-level middleware chain.
	TLSConfig      *tls.Config                     // TLSConfig configures TLS settings for the underlying transport.
	MaxRetries     int                             // MaxRetries is the maximum number of retry attempts.
	RetryStrategy  BackoffStrategy                 // RetryStrategy computes the delay before the next retry.
	RetryIf        RetryIfFunc                     // RetryIf decides whether a request should be retried.
	HTTPClient     *http.Client                    // HTTPClient is the underlying HTTP client used to send requests.
	JSONEncoder    Encoder                         // JSONEncoder encodes JSON request bodies.
	JSONDecoder    Decoder                         // JSONDecoder decodes JSON response bodies.
	XMLEncoder     Encoder                         // XMLEncoder encodes XML request bodies.
	XMLDecoder     Decoder                         // XMLDecoder decodes XML response bodies.
	YAMLEncoder    Encoder                         // YAMLEncoder encodes YAML request bodies.
	YAMLDecoder    Decoder                         // YAMLDecoder decodes YAML response bodies.
	Logger         Logger                          // Logger receives client log output when configured.
	dialTimeout    time.Duration
	resolver       *net.Resolver
	localAddr      net.Addr
	dialContext    func(context.Context, string, string) (net.Conn, error)
	auth           AuthMethod
}

// Config sets up the initial configuration for the HTTP client.
type Config struct {
	BaseURL           string                          // BaseURL is the base URL for requests made by this client.
	Headers           *http.Header                    // Headers contains the default headers sent with each request.
	OrderedHeaders    *orderedobject.Object[[]string] // OrderedHeaders contains ordered default headers.
	Cookies           map[string]string               // Cookies contains the default cookies sent with each request.
	Timeout           time.Duration                   // Timeout is the default request timeout.
	CookieJar         *cookiejar.Jar                  // CookieJar stores and sends cookies for the client.
	Middlewares       []Middleware                    // Middlewares contains the middleware stack for request and response handling.
	TLSConfig         *tls.Config                     // TLSConfig configures TLS settings for the client.
	TLSClientCertFile string                          // TLSClientCertFile is the path to the client certificate file.
	TLSClientKeyFile  string                          // TLSClientKeyFile is the path to the client private key file.
	TLSServerName     string                          // TLSServerName is the TLS server name used for SNI.
	Transport         http.RoundTripper               // Transport is the custom transport used by the client.
	MaxRetries        int                             // MaxRetries is the maximum number of retry attempts.
	RetryStrategy     BackoffStrategy                 // RetryStrategy computes the delay before the next retry.
	RetryIf           RetryIfFunc                     // RetryIf decides whether a request should be retried.
	Logger            Logger                          // Logger receives client log output when configured.
	HTTP2             bool                            // HTTP2 enables HTTP/2 on the default HTTP transport.
	Resolver          *net.Resolver                   // Resolver customizes name resolution for the default transport dialer.
	// DialContext is the dial function used by the default transport.
	DialContext func(context.Context, string, string) (net.Conn, error)
	LocalAddr   net.Addr // LocalAddr is the local address used by the default transport dialer.

	// Transport-level timeouts.
	DialTimeout           time.Duration // DialTimeout is the TCP connection timeout.
	TLSHandshakeTimeout   time.Duration // TLSHandshakeTimeout is the TLS handshake timeout.
	ResponseHeaderTimeout time.Duration // ResponseHeaderTimeout is the time to the first response byte.

	// Connection pool settings.
	MaxIdleConns        int           // MaxIdleConns is the maximum number of idle connections across all hosts.
	MaxIdleConnsPerHost int           // MaxIdleConnsPerHost is the maximum number of idle connections per host.
	MaxConnsPerHost     int           // MaxConnsPerHost is the maximum number of connections per host.
	IdleConnTimeout     time.Duration // IdleConnTimeout is how long idle connections remain open.
}

type clientSnapshot struct {
	BaseURL        string
	Headers        http.Header
	OrderedHeaders *orderedobject.Object[[]string]
	Cookies        []*http.Cookie
	Middlewares    []Middleware
	MaxRetries     int
	RetryStrategy  BackoffStrategy
	RetryIf        RetryIfFunc
	HTTPClient     *http.Client
	JSONEncoder    Encoder
	XMLEncoder     Encoder
	YAMLEncoder    Encoder
	Logger         Logger
	auth           AuthMethod
}

// Validate checks whether the config contains deterministic invalid values.
func (cfg *Config) Validate() error {
	var errs []error

	if _, err := url.Parse(cfg.BaseURL); cfg.BaseURL != "" && err != nil {
		errs = append(errs, fmt.Errorf("invalid BaseURL: %w", err))
	}

	if cfg.Timeout < 0 {
		errs = append(errs, fmt.Errorf("%w: Timeout", ErrInvalidConfigValue))
	}
	if cfg.DialTimeout < 0 {
		errs = append(errs, fmt.Errorf("%w: DialTimeout", ErrInvalidConfigValue))
	}
	if cfg.TLSHandshakeTimeout < 0 {
		errs = append(errs, fmt.Errorf("%w: TLSHandshakeTimeout", ErrInvalidConfigValue))
	}
	if cfg.ResponseHeaderTimeout < 0 {
		errs = append(errs, fmt.Errorf("%w: ResponseHeaderTimeout", ErrInvalidConfigValue))
	}
	if cfg.IdleConnTimeout < 0 {
		errs = append(errs, fmt.Errorf("%w: IdleConnTimeout", ErrInvalidConfigValue))
	}
	if cfg.MaxRetries < 0 {
		errs = append(errs, fmt.Errorf("%w: MaxRetries", ErrInvalidConfigValue))
	}
	if cfg.MaxIdleConns < 0 {
		errs = append(errs, fmt.Errorf("%w: MaxIdleConns", ErrInvalidConfigValue))
	}
	if cfg.MaxIdleConnsPerHost < 0 {
		errs = append(errs, fmt.Errorf("%w: MaxIdleConnsPerHost", ErrInvalidConfigValue))
	}
	if cfg.MaxConnsPerHost < 0 {
		errs = append(errs, fmt.Errorf("%w: MaxConnsPerHost", ErrInvalidConfigValue))
	}
	if (cfg.TLSClientCertFile == "") != (cfg.TLSClientKeyFile == "") {
		errs = append(errs, ErrInvalidTLSClientCertificateConfig)
	}

	return errors.Join(errs...)
}

// hasTransportConfig checks if any transport-level configuration is set.
func (cfg *Config) hasTransportConfig() bool {
	return cfg.DialTimeout > 0 || cfg.TLSHandshakeTimeout > 0 ||
		cfg.ResponseHeaderTimeout > 0 || cfg.MaxIdleConns > 0 ||
		cfg.MaxIdleConnsPerHost > 0 || cfg.MaxConnsPerHost > 0 ||
		cfg.IdleConnTimeout > 0 || cfg.Resolver != nil ||
		cfg.DialContext != nil || cfg.LocalAddr != nil
}

// New creates a Client with functional options applied.
// It calls Create(nil) to initialize a client with default settings,
// then applies each option in order.
func New(opts ...ClientOption) *Client {
	c := Create(nil)
	for _, opt := range opts {
		opt(c)
	}
	return c
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

	client := &Client{
		BaseURL:        config.BaseURL,
		Headers:        config.Headers,
		OrderedHeaders: cloneOrderedHeaders(config.OrderedHeaders),
		HTTPClient:     httpClient,
		JSONEncoder:    DefaultJSONEncoder,
		JSONDecoder:    DefaultJSONDecoder,
		XMLEncoder:     DefaultXMLEncoder,
		XMLDecoder:     DefaultXMLDecoder,
		YAMLEncoder:    DefaultYAMLEncoder,
		YAMLDecoder:    DefaultYAMLDecoder,
		TLSConfig:      config.TLSConfig,
		dialTimeout:    config.DialTimeout,
		resolver:       config.Resolver,
		localAddr:      config.LocalAddr,
		dialContext:    config.DialContext,
	}
	if client.OrderedHeaders != nil {
		client.Headers = new(headerFromOrderedHeaders(client.OrderedHeaders))
	}

	if config.TLSServerName != "" {
		if client.TLSConfig == nil {
			client.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		client.TLSConfig.ServerName = config.TLSServerName
	}

	if config.TLSClientCertFile != "" && config.TLSClientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(filepath.Clean(config.TLSClientCertFile), filepath.Clean(config.TLSClientKeyFile))
		if err == nil {
			if client.TLSConfig == nil {
				client.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
			}
			client.TLSConfig.Certificates = []tls.Certificate{cert}
		}
	}

	if client.TLSConfig != nil {
		if httpClient.Transport != nil {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				transport.TLSClientConfig = client.TLSConfig
			}
		} else {
			client.HTTPClient.Transport = &http.Transport{
				TLSClientConfig: client.TLSConfig,
			}
		}
	}

	applyTransportConfig(client, config)
	if config.HTTP2 {
		client.EnableHTTP2()
	}

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

func (c *Client) syncTLSConfigLocked() {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}
	if c.TLSConfig == nil {
		return
	}
	if transport, ok := c.HTTPClient.Transport.(*http.Transport); ok {
		transport.TLSClientConfig = c.TLSConfig
		if isHTTP2Configured(transport) {
			ensureHTTP2NextProtos(transport)
			transport.ForceAttemptHTTP2 = true
		}
		return
	}
	if transport, ok := c.HTTPClient.Transport.(*http2.Transport); ok {
		transport.TLSClientConfig = c.TLSConfig
		return
	}
	c.HTTPClient.Transport = &http.Transport{TLSClientConfig: c.TLSConfig}
}

// SetTLSConfig sets the TLS configuration for the client.
func (c *Client) SetTLSConfig(config *tls.Config) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.TLSConfig = config
	c.syncTLSConfigLocked()
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
	c.syncTLSConfigLocked()
	return c
}

// SetCertificates sets the TLS certificates for the client.
func (c *Client) SetCertificates(certs ...tls.Certificate) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureTLSConfig()
	c.TLSConfig.Certificates = certs
	c.syncTLSConfigLocked()
	return c
}

// SetClientCertificate loads and sets a client certificate and private key from files.
func (c *Client) SetClientCertificate(certFile, keyFile string) *Client {
	cert, err := tls.LoadX509KeyPair(filepath.Clean(certFile), filepath.Clean(keyFile))
	if err != nil {
		if c.Logger != nil {
			c.Logger.Errorf("failed to load client certificate: %v", err)
		}
		return c
	}
	return c.SetCertificates(cert)
}

// SetTLSServerName sets the TLS server name (SNI) for the client.
func (c *Client) SetTLSServerName(serverName string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureTLSConfig()
	c.TLSConfig.ServerName = serverName
	c.syncTLSConfigLocked()
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
func (c *Client) handleCAs(scope string, pemCerts []byte) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureTLSConfig()
	switch scope {
	case "root":
		if c.TLSConfig.RootCAs == nil {
			c.TLSConfig.RootCAs = x509.NewCertPool()
		}
		c.TLSConfig.RootCAs.AppendCertsFromPEM(pemCerts)
	case "client":
		if c.TLSConfig.ClientCAs == nil {
			c.TLSConfig.ClientCAs = x509.NewCertPool()
		}
		c.TLSConfig.ClientCAs.AppendCertsFromPEM(pemCerts)
	}
	c.syncTLSConfigLocked()
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
	c.OrderedHeaders = nil
}

// SetDefaultOrderedHeaders sets ordered default headers for the client.
func (c *Client) SetDefaultOrderedHeaders(headers *orderedobject.Object[[]string]) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.OrderedHeaders = cloneOrderedHeaders(headers)
	if c.OrderedHeaders == nil {
		c.Headers = nil
		return
	}
	c.Headers = new(headerFromOrderedHeaders(c.OrderedHeaders))
}

// SetDefaultHeader adds or updates a default header.
func (c *Client) SetDefaultHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Headers == nil {
		c.Headers = &http.Header{}
	}
	c.Headers.Set(key, value)
	if c.OrderedHeaders != nil {
		setOrderedHeaderValues(&c.OrderedHeaders, key, []string{value})
	}
}

// AddDefaultHeader adds a default header.
func (c *Client) AddDefaultHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Headers == nil {
		c.Headers = &http.Header{}
	}
	c.Headers.Add(key, value)
	if c.OrderedHeaders != nil {
		addOrderedHeaderValue(&c.OrderedHeaders, key, value)
	}
}

// DelDefaultHeader removes a default header.
func (c *Client) DelDefaultHeader(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Headers == nil {
		return
	}
	c.Headers.Del(key)
	if c.OrderedHeaders != nil {
		deleteOrderedHeader(c.OrderedHeaders, key)
	}
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

// EnableSession enables cookie and TLS session reuse without replacing existing session stores.
func (c *Client) EnableSession() *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}
	if c.HTTPClient.Jar == nil {
		jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err == nil {
			c.HTTPClient.Jar = jar
		} else if c.Logger != nil {
			c.Logger.Errorf("failed to create cookie jar: %v", err)
		}
	}

	c.ensureTLSConfig()
	if c.TLSConfig.ClientSessionCache == nil {
		c.TLSConfig.ClientSessionCache = tls.NewLRUClientSessionCache(0)
	}
	switch c.HTTPClient.Transport.(type) {
	case nil, *http.Transport, *http2.Transport:
		c.syncTLSConfigLocked()
	}
	return c
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

func (c *Client) snapshot() clientSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	headers := http.Header{}
	if c.Headers != nil {
		headers = c.Headers.Clone()
	}

	cookies := make([]*http.Cookie, len(c.Cookies))
	for i, cookie := range c.Cookies {
		if cookie == nil {
			continue
		}
		clone := new(*cookie)
		clone.Unparsed = slices.Clone(cookie.Unparsed)
		cookies[i] = clone
	}

	middlewares := slices.Clone(c.Middlewares)

	return clientSnapshot{
		BaseURL:        c.BaseURL,
		Headers:        headers,
		OrderedHeaders: cloneOrderedHeaders(c.OrderedHeaders),
		Cookies:        cookies,
		Middlewares:    middlewares,
		MaxRetries:     c.MaxRetries,
		RetryStrategy:  c.RetryStrategy,
		RetryIf:        c.RetryIf,
		HTTPClient:     c.HTTPClient,
		JSONEncoder:    c.JSONEncoder,
		XMLEncoder:     c.XMLEncoder,
		YAMLEncoder:    c.YAMLEncoder,
		Logger:         c.Logger,
		auth:           c.auth,
	}
}

// GetHTTPClient returns the underlying HTTP client in a thread-safe way.
func (c *Client) GetHTTPClient() *http.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.HTTPClient
}

// GetBaseURL returns the configured base URL in a thread-safe way.
func (c *Client) GetBaseURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.BaseURL
}

// SetMaxRetries sets the maximum number of retry attempts.
func (c *Client) SetMaxRetries(maxRetries int) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.MaxRetries = maxRetries
	return c
}

// EnableHTTP2 enables HTTP/2 on the underlying HTTP transport.
func (c *Client) EnableHTTP2() *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.enableHTTP2Locked()
	return c
}

func (c *Client) enableHTTP2Locked() {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{}
	}
	transport, err := c.ensureTransport()
	if err != nil {
		if c.Logger != nil {
			c.Logger.Errorf("failed to enable HTTP/2: %v", err)
		}
		return
	}
	if c.TLSConfig != nil {
		transport.TLSClientConfig = c.TLSConfig
	}
	if err := configureHTTP2Transport(transport); err != nil && c.Logger != nil {
		c.Logger.Errorf("failed to enable HTTP/2: %v", err)
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

// SetDialTimeout sets the TCP connection timeout on the underlying transport.
func (c *Client) SetDialTimeout(d time.Duration) *Client {
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

// SetResolver sets the resolver used by the default transport dialer.
func (c *Client) SetResolver(resolver *net.Resolver) *Client {
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

// SetDialContext sets the dial function on the underlying transport.
func (c *Client) SetDialContext(dial func(context.Context, string, string) (net.Conn, error)) *Client {
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

// SetLocalAddr sets the local address used by the default transport dialer.
func (c *Client) SetLocalAddr(addr net.Addr) *Client {
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
// (non-zero). Skips if the transport is not *http.Transport.
func applyTransportConfig(c *Client, config *Config) {
	transport, ok := c.HTTPClient.Transport.(*http.Transport)
	if !ok && (c.HTTPClient.Transport != nil || !config.hasTransportConfig()) {
		return
	}
	if !ok {
		transport = &http.Transport{}
		c.HTTPClient.Transport = transport
	}

	if config.DialContext != nil {
		transport.DialContext = config.DialContext
	} else if config.DialTimeout > 0 || config.Resolver != nil || config.LocalAddr != nil {
		transport.DialContext = (&net.Dialer{
			Timeout:   config.DialTimeout,
			Resolver:  config.Resolver,
			LocalAddr: config.LocalAddr,
		}).DialContext
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
