package requests

import (
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

// Client represents an HTTP client
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
	HttpClient    *http.Client
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
		HttpClient:  httpClient,
		JSONEncoder: DefaultJSONEncoder,
		JSONDecoder: DefaultJSONDecoder,
		XMLEncoder:  DefaultXMLEncoder,
		XMLDecoder:  DefaultXMLDecoder,
		YAMLEncoder: DefaultYAMLEncoder,
		YAMLDecoder: DefaultYAMLDecoder,
		TLSConfig:   config.TLSConfig,
	}

	// If a TLS configuration is provided, apply it to the Transport.
	if client.TLSConfig != nil && httpClient.Transport != nil {
		httpTransport := httpClient.Transport.(*http.Transport)
		httpTransport.TLSClientConfig = client.TLSConfig
	} else if client.TLSConfig != nil {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: client.TLSConfig,
		}
	}

	if config.Middlewares != nil {
		client.Middlewares = config.Middlewares
	} else {
		client.Middlewares = make([]Middleware, 0)
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

// SetBaseURL sets the base URL for the client
func (c *Client) SetBaseURL(baseURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.BaseURL = baseURL
}

// AddMiddleware adds a middleware to the client
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

	if c.HttpClient == nil {
		c.HttpClient = &http.Client{}
	}

	// Apply the TLS configuration to the existing transport if possible.
	// If the current transport is not an *http.Transport, replace it.
	if transport, ok := c.HttpClient.Transport.(*http.Transport); ok {
		transport.TLSClientConfig = config
	} else {
		c.HttpClient.Transport = &http.Transport{
			TLSClientConfig: config,
		}
	}

	return c
}

// InsecureSkipVerify sets the TLS configuration to skip certificate verification.
func (c *Client) InsecureSkipVerify() *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.TLSConfig == nil {
		c.TLSConfig = &tls.Config{}
	}

	c.TLSConfig.InsecureSkipVerify = true

	if c.HttpClient == nil {
		c.HttpClient = &http.Client{}
	}
	if transport, ok := c.HttpClient.Transport.(*http.Transport); ok {
		transport.TLSClientConfig = c.TLSConfig
	} else {
		c.HttpClient.Transport = &http.Transport{
			TLSClientConfig: c.TLSConfig,
		}
	}

	return c
}

// SetHTTPClient sets the HTTP client for the client
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.HttpClient = httpClient
}

// SetDefaultHeaders sets the default headers for the client
func (c *Client) SetDefaultHeaders(headers *http.Header) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Headers = headers
}

// SetDefaultHeader adds or updates a default header
func (c *Client) SetDefaultHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Headers == nil {
		c.Headers = &http.Header{}
	}
	c.Headers.Set(key, value)
}

// AddDefaultHeader adds a default header
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

	if c.Headers != nil { // Only attempt to delete if Headers is initialized
		c.Headers.Del(key)
	}
}

// SetDefaultContentType sets the default content type for the client
func (c *Client) SetDefaultContentType(contentType string) {
	c.SetDefaultHeader("Content-Type", contentType)
}

// SetDefaultAccept sets the default accept header for the client
func (c *Client) SetDefaultAccept(accept string) {
	c.SetDefaultHeader("Accept", accept)
}

// SetDefaultUserAgent sets the default user agent for the client
func (c *Client) SetDefaultUserAgent(userAgent string) {
	c.SetDefaultHeader("User-Agent", userAgent)
}

// SetDefaultReferer sets the default referer for the client
func (c *Client) SetDefaultReferer(referer string) {
	c.SetDefaultHeader("Referer", referer)
}

// SetDefaultTimeout sets the default timeout for the client
func (c *Client) SetDefaultTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.HttpClient.Timeout = timeout
}

// SetDefaultTransport sets the default transport for the client
func (c *Client) SetDefaultTransport(transport http.RoundTripper) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.HttpClient.Transport = transport
}

// SetDefaultCookieJar sets the default cookie jar for the client
func (c *Client) SetDefaultCookieJar(jar *cookiejar.Jar) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.HttpClient.Jar = jar
}

// SetDefaultCookies sets the default cookies for the client
func (c *Client) SetDefaultCookies(cookies map[string]string) {
	for name, value := range cookies {
		c.SetDefaultCookie(name, value)
	}
}

// SetDefaultCookie sets a default cookie for the client
func (c *Client) SetDefaultCookie(name, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Cookies == nil {
		c.Cookies = make([]*http.Cookie, 0)
	}
	c.Cookies = append(c.Cookies, &http.Cookie{Name: name, Value: value})
}

// DelDefaultCookie removes a default cookie from the client
func (c *Client) DelDefaultCookie(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Cookies != nil { // Only attempt to delete if Cookies is initialized
		for i, cookie := range c.Cookies {
			if cookie.Name == name {
				c.Cookies = append(c.Cookies[:i], c.Cookies[i+1:]...)
				break
			}
		}
	}
}

// SetJSONMarshal sets the JSON marshal function for the client's JSONEncoder
func (c *Client) SetJSONMarshal(marshalFunc func(v any) ([]byte, error)) {
	c.JSONEncoder = &JSONEncoder{
		MarshalFunc: marshalFunc,
	}
}

// SetJSONUnmarshal sets the JSON unmarshal function for the client's JSONDecoder
func (c *Client) SetJSONUnmarshal(unmarshalFunc func(data []byte, v any) error) {
	c.JSONDecoder = &JSONDecoder{
		UnmarshalFunc: unmarshalFunc,
	}
}

// SetXMLMarshal sets the XML marshal function for the client's XMLEncoder
func (c *Client) SetXMLMarshal(marshalFunc func(v any) ([]byte, error)) {
	c.XMLEncoder = &XMLEncoder{
		MarshalFunc: marshalFunc,
	}
}

// SetXMLUnmarshal sets the XML unmarshal function for the client's XMLDecoder
func (c *Client) SetXMLUnmarshal(unmarshalFunc func(data []byte, v any) error) {
	c.XMLDecoder = &XMLDecoder{
		UnmarshalFunc: unmarshalFunc,
	}
}

// SetYAMLMarshal sets the YAML marshal function for the client's YAMLEncoder
func (c *Client) SetYAMLMarshal(marshalFunc func(v any) ([]byte, error)) {
	c.YAMLEncoder = &YAMLEncoder{
		MarshalFunc: marshalFunc,
	}
}

// SetYAMLUnmarshal sets the YAML unmarshal function for the client's YAMLDecoder
func (c *Client) SetYAMLUnmarshal(unmarshalFunc func(data []byte, v any) error) {
	c.YAMLDecoder = &YAMLDecoder{
		UnmarshalFunc: unmarshalFunc,
	}
}

// SetMaxRetries sets the maximum number of retry attempts
func (c *Client) SetMaxRetries(maxRetries int) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.MaxRetries = maxRetries
	return c
}

// SetRetryStrategy sets the backoff strategy for retries
func (c *Client) SetRetryStrategy(strategy BackoffStrategy) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.RetryStrategy = strategy
	return c
}

// SetRetryIf sets the custom retry condition function
func (c *Client) SetRetryIf(retryIf RetryIfFunc) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.RetryIf = retryIf
	return c
}

// SetAuth configures an authentication method for the client.
func (c *Client) SetAuth(auth AuthMethod) {
	if auth.Valid() {
		c.auth = auth
	}
}

// SetLogger sets logger instance in client.
func (c *Client) SetLogger(logger Logger) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Logger = logger
	return c
}

// Get initiates a GET request
func (c *Client) Get(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodGet, path)
}

// Post initiates a POST request
func (c *Client) Post(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodPost, path)
}

// Delete initiates a DELETE request
func (c *Client) Delete(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodDelete, path)
}

// Put initiates a PUT request
func (c *Client) Put(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodPut, path)
}

// Patch initiates a PATCH request
func (c *Client) Patch(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodPatch, path)
}

// Options initiates an OPTIONS request
func (c *Client) Options(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodOptions, path)
}

// Head initiates a HEAD request
func (c *Client) Head(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodHead, path)
}

// CONNECT initiates a CONNECT request
func (c *Client) CONNECT(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodConnect, path)
}

// TRACE initiates a TRACE request
func (c *Client) TRACE(path string) *RequestBuilder {
	return c.NewRequestBuilder(http.MethodTrace, path)
}

// Custom initiates a custom request
func (c *Client) Custom(path, method string) *RequestBuilder {
	return c.NewRequestBuilder(method, path)
}
