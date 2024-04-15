
## pool.go
package requests

import (
	"bytes"

	"github.com/valyala/bytebufferpool"
)

var bufferPool bytebufferpool.Pool

// GetBuffer retrieves a buffer from the pool
func GetBuffer() *bytebufferpool.ByteBuffer {
	return bufferPool.Get()
}

// PutBuffer returns a buffer to the pool
func PutBuffer(b *bytebufferpool.ByteBuffer) {
	bufferPool.Put(b)
}

// poolReader wraps bytes.Reader to return the buffer to the pool when closed.
type poolReader struct {
	*bytes.Reader
	poolBuf *bytebufferpool.ByteBuffer
}

func (r *poolReader) Close() error {
	PutBuffer(r.poolBuf)
	return nil
}


## coder.go
package requests

import (
	"io"
)

type Encoder interface {
	Encode(v any) (io.Reader, error)
	ContentType() string
}

type Decoder interface {
	Decode(r io.Reader, v any) error
}


## xml.go
package requests

import (
	"bytes"
	"encoding/xml"
	"io"
)

type XMLEncoder struct {
	MarshalFunc func(v any) ([]byte, error)
}

func (e *XMLEncoder) Encode(v any) (io.Reader, error) {
	var err error
	var data []byte

	if e.MarshalFunc != nil {
		data, err = e.MarshalFunc(v)
	} else {
		data, err = xml.Marshal(v) // Fallback to standard XML marshal if no custom function is provided
	}

	if err != nil {
		return nil, err
	}

	buf := GetBuffer()
	_, err = buf.Write(data)
	if err != nil {
		PutBuffer(buf)
		return nil, err
	}

	return &poolReader{Reader: bytes.NewReader(buf.B), poolBuf: buf}, nil
}

func (e *XMLEncoder) ContentType() string {
	return "application/xml;charset=utf-8"
}

var DefaultXMLEncoder = &XMLEncoder{
	MarshalFunc: xml.Marshal,
}

type XMLDecoder struct {
	UnmarshalFunc func(data []byte, v any) error
}

func (d *XMLDecoder) Decode(r io.Reader, v any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if d.UnmarshalFunc != nil {
		return d.UnmarshalFunc(data, v)
	}

	return xml.Unmarshal(data, v) // Fallback to standard XML unmarshal
}

var DefaultXMLDecoder = &XMLDecoder{
	UnmarshalFunc: xml.Unmarshal,
}

## json.go
package requests

import (
	"bytes"
	"encoding/json"
	"io"
)

type JSONEncoder struct {
	MarshalFunc func(v any) ([]byte, error)
}

func (e *JSONEncoder) Encode(v any) (io.Reader, error) {
	data, err := e.MarshalFunc(v)

	if err != nil {
		return nil, err
	}

	buf := GetBuffer()
	_, err = buf.Write(data)
	if err != nil {
		PutBuffer(buf) // Ensure the buffer is returned to the pool in case of an error
		return nil, err
	}

	// Here, we need to ensure the buffer will be returned to the pool after being read.
	// One approach is to wrap the bytes.Reader in a custom type that returns the buffer on close.
	reader := &poolReader{Reader: bytes.NewReader(buf.B), poolBuf: buf}
	return reader, nil
}

func (e *JSONEncoder) ContentType() string {
	return "application/json;charset=utf-8"
}

var DefaultJSONEncoder = &JSONEncoder{
	MarshalFunc: json.Marshal,
}

type JSONDecoder struct {
	UnmarshalFunc func(data []byte, v any) error
}

func (d *JSONDecoder) Decode(r io.Reader, v any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	return d.UnmarshalFunc(data, v)
}

var DefaultJSONDecoder = &JSONDecoder{
	UnmarshalFunc: json.Unmarshal,
}

## form.go
package requests

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/google/go-querystring/query"
)

// FormEncoder handles encoding of form data.
type FormEncoder struct{}

// Encode encodes the given value into URL-encoded form data.
func (e *FormEncoder) Encode(v any) (io.Reader, error) {
	switch data := v.(type) {
	case url.Values:
		// Directly encode url.Values data.
		return strings.NewReader(data.Encode()), nil
	case map[string][]string:
		// Convert and encode map[string][]string data as url.Values.
		values := url.Values(data)
		return strings.NewReader(values.Encode()), nil
	case map[string]string:
		// Convert and encode map[string]string data as url.Values.
		values := make(url.Values)
		for key, value := range data {
			values.Set(key, value)
		}
		return strings.NewReader(values.Encode()), nil
	default:
		// Attempt to use query.Values for encoding struct types.
		if values, err := query.Values(v); err == nil {
			return strings.NewReader(values.Encode()), nil
		} else {
			// Return an error if encoding fails or type is unsupported.
			return nil, fmt.Errorf("%w: %v", ErrEncodingFailed, err)
		}
	}
}

var DefaultFormEncoder = &FormEncoder{}

## client.go
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



## request.go
package requests

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-querystring/query"
)

// RequestBuilder facilitates building and executing HTTP requests
type RequestBuilder struct {
	client   *Client
	method   string
	path     string
	headers  *http.Header
	cookies  []*http.Cookie
	query    url.Values
	bodyData interface{}
}

// NewRequestBuilder creates a new RequestBuilder with default settings
func (c *Client) NewRequestBuilder(method, path string) *RequestBuilder {
	return &RequestBuilder{
		client:  c,
		method:  method,
		path:    path,
		query:   url.Values{},
		headers: &http.Header{},
	}
}

// Method sets the HTTP method for the request.
func (b *RequestBuilder) Method(method string) *RequestBuilder {
	b.method = method
	return b
}

// Path sets the URL path for the request.
func (b *RequestBuilder) Path(path string) *RequestBuilder {
	b.path = path
	return b
}

// Query adds query parameters to the request
func (b *RequestBuilder) Query(params url.Values) *RequestBuilder {
	for key, values := range params {
		for _, value := range values {
			b.query.Add(key, value)
		}
	}
	return b
}

// QueryStruct adds query parameters to the request based on a struct tagged with url tags.
func (b *RequestBuilder) QueryStruct(queryStruct interface{}) *RequestBuilder {
	values, _ := query.Values(queryStruct) // Safely ignore error for simplicity
	for key, value := range values {
		for _, v := range value {
			b.query.Add(key, v)
		}
	}
	return b
}

// WithHeaders adds headers to the request
func (b *RequestBuilder) Headers(headers http.Header) *RequestBuilder {
	for key, values := range headers {
		for _, value := range values {
			b.headers.Add(key, value)
		}
	}
	return b
}

// AddHeader adds a header to the request.
func (b *RequestBuilder) AddHeader(key, value string) *RequestBuilder {
	b.headers.Add(key, value)
	return b
}

// SetHeader sets (or replaces) a header in the request.
func (b *RequestBuilder) SetHeader(key, value string) *RequestBuilder {
	b.headers.Set(key, value)
	return b
}

// DelHeader removes a header from the request.
func (b *RequestBuilder) DelHeader(key string) *RequestBuilder {
	b.headers.Del(key)
	return b
}

// SetCookies method for map
func (b *RequestBuilder) SetCookies(cookies map[string]string) *RequestBuilder {
	for name, value := range cookies {
		b.SetCookie(name, value)
	}
	return b
}

// SetCookie method
func (b *RequestBuilder) SetCookie(name, value string) *RequestBuilder {
	if b.cookies == nil {
		b.cookies = []*http.Cookie{}
	}
	b.cookies = append(b.cookies, &http.Cookie{Name: name, Value: value})
	return b
}

// DelCookie method
func (b *RequestBuilder) DelCookie(name string) *RequestBuilder {
	if b.cookies != nil {
		for i, cookie := range b.cookies {
			if cookie.Name == name {
				b.cookies = append(b.cookies[:i], b.cookies[i+1:]...)
				break
			}
		}
	}
	return b
}

// ContentType sets the Content-Type header for the request.
func (b *RequestBuilder) ContentType(contentType string) *RequestBuilder {
	b.headers.Set("Content-Type", contentType)
	return b
}

// Accept sets the Accept header for the request.
func (b *RequestBuilder) Accept(accept string) *RequestBuilder {
	b.headers.Set("Accept", accept)
	return b
}

// UserAgent sets the User-Agent header for the request.
func (b *RequestBuilder) UserAgent(userAgent string) *RequestBuilder {
	b.headers.Set("User-Agent", userAgent)
	return b
}

// Referer sets the Referer header for the request.
func (b *RequestBuilder) Referer(referer string) *RequestBuilder {
	b.headers.Set("Referer", referer)
	return b
}

// SetBasicAuth sets the Authorization header to use HTTP Basic Authentication
// with the provided username and password.
func (b *RequestBuilder) SetBasicAuth(username, password string) *RequestBuilder {
	auth := username + ":" + password
	encoded := base64.StdEncoding.EncodeToString([]byte(auth))
	b.headers.Set("Authorization", "Basic "+encoded)
	return b
}

// Body sets the request body
func (b *RequestBuilder) Body(body interface{}) *RequestBuilder {
	b.bodyData = body
	return b
}

func (b *RequestBuilder) JSONBody(v interface{}) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "application/json")
	return b
}

func (b *RequestBuilder) XMLBody(v interface{}) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "application/xml")
	return b
}

func (b *RequestBuilder) YAMLBody(v interface{}) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "application/yaml")
	return b
}

func (b *RequestBuilder) FormBody(v interface{}) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "application/x-www-form-urlencoded")
	return b
}

func (b *RequestBuilder) TextBody(v string) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "text/plain")
	return b
}

func (b *RequestBuilder) RawBody(v []byte) *RequestBuilder {
	b.bodyData = v
	return b
}

// Send executes the HTTP request.
func (b *RequestBuilder) Send(ctx context.Context) (*Response, error) {
	var body io.Reader
	var err error

	// Determine the Content-Type of the request.
	contentType := b.headers.Get("Content-Type")
	if contentType == "" && b.bodyData != nil {
		switch b.bodyData.(type) {
		case url.Values, map[string][]string, map[string]string:
			contentType = "application/x-www-form-urlencoded"
		case map[string]interface{}, []interface{}, struct{}:
			contentType = "application/json"
		case string, []byte:
			contentType = "text/plain"
		}

		// Set the inferred Content-Type
		b.headers.Set("Content-Type", contentType)
	}

	// Encode the body based on the Content-Type.
	switch contentType {
	case "application/json":
		body, err = b.client.JSONEncoder.Encode(b.bodyData)
	case "application/xml":
		body, err = b.client.XMLEncoder.Encode(b.bodyData)
	case "application/yaml":
		body, err = b.client.YAMLEncoder.Encode(b.bodyData)
	case "application/x-www-form-urlencoded":
		body, err = DefaultFormEncoder.Encode(b.bodyData)
	case "text/plain", "application/octet-stream":
		switch data := b.bodyData.(type) {
		case string:
			body = strings.NewReader(data)
		case []byte:
			body = bytes.NewReader(data)
		}
	}

	if err != nil {
		return nil, err
	}

	// Parse the complete URL first to handle any modifications needed.
	parsedURL, err := url.Parse(b.client.BaseURL + b.path)
	if err != nil {
		return nil, err
	}

	// Combine query parameters from both the URL and the Query method.
	query := parsedURL.Query()
	for key, values := range b.query {
		for _, value := range values {
			query.Set(key, value) // Add new values, preserving existing ones.
		}
	}
	parsedURL.RawQuery = query.Encode()

	// Create the HTTP request with the fully prepared URL, including query parameters.
	req, err := http.NewRequestWithContext(ctx, b.method, parsedURL.String(), body)
	if err != nil {
		return nil, err
	}

	// Set the headers from the client and the request builder.
	if b.client.Headers != nil {
		for key := range *b.client.Headers {
			values := (*b.client.Headers)[key]
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
	if b.headers != nil {
		for key := range *b.headers {
			values := (*b.headers)[key]
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// Merge cookies from the client and the request builder.
	if b.client.Cookies != nil {
		for _, cookie := range b.client.Cookies {
			req.AddCookie(cookie)
		}
	}
	if b.cookies != nil {
		for _, cookie := range b.cookies {
			req.AddCookie(cookie)
		}
	}

	// Execute the HTTP request.
	resp, err := b.client.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Wrap and return the response.
	return NewResponse(ctx, resp, b.client)
}

## response.go
package requests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Response struct {
	RawResponse *http.Response
	BodyBytes   []byte
	Context     context.Context
	Client      *Client
}

// NewResponse creates a new wrapped response object, leveraging the buffer pool for efficient memory usage.
func NewResponse(ctx context.Context, resp *http.Response, client *Client) (*Response, error) {
	buf := GetBuffer() // Use the buffer pool
	defer PutBuffer(buf)

	_, err := buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()

	resp.Body = io.NopCloser(bytes.NewReader(buf.B))

	return &Response{
		RawResponse: resp,
		BodyBytes:   buf.Bytes(),
		Context:     ctx,
		Client:      client,
	}, nil
}

// StatusCode returns the HTTP status code of the response.
func (r *Response) StatusCode() int {
	return r.RawResponse.StatusCode
}

// Status returns the status string of the response (e.g., "200 OK").
func (r *Response) Status() string {
	return r.RawResponse.Status
}

// Header returns the response headers.
func (r *Response) Header() http.Header {
	return r.RawResponse.Header
}

// URL returns the request URL that elicited the response.
func (r *Response) URL() *url.URL {
	return r.RawResponse.Request.URL
}

// ContentType returns the value of the "Content-Type" header.
func (r *Response) ContentType() string {
	return r.Header().Get("Content-Type")
}

// Checks if the response Content-Type header matches a given content type.
func (r *Response) IsContentType(contentType string) bool {
	return strings.Contains(r.ContentType(), contentType)
}

// IsJSON checks if the response Content-Type indicates JSON.
func (r *Response) IsJSON() bool {
	return r.IsContentType("application/json")
}

// IsXML checks if the response Content-Type indicates XML.
func (r *Response) IsXML() bool {
	return r.IsContentType("application/xml")
}

// IsYAML checks if the response Content-Type indicates YAML.
func (r *Response) IsYAML() bool {
	return r.IsContentType("application/yaml")
}

// ContentLength returns the length of the response body.
func (r *Response) ContentLength() int {
	return len(r.BodyBytes)
}

// IsEmpty checks if the response body is empty.
func (r *Response) IsEmpty() bool {
	return r.ContentLength() == 0
}

// Cookies parses and returns the cookies set in the response.
func (r *Response) Cookies() []*http.Cookie {
	return r.RawResponse.Cookies()
}

// IsSuccess checks if the response status code indicates success (200 - 299).
func (r *Response) IsSuccess() bool {
	code := r.StatusCode()
	return code >= 200 && code <= 299
}

// Body returns the response body as a byte slice.
func (r *Response) Body() []byte {
	return r.BodyBytes
}

// String returns the response body as a string.
func (r *Response) String() string {
	return string(r.BodyBytes)
}

// Scan attempts to unmarshal the response body based on its content type.
func (r *Response) Scan(v interface{}) error {
	if r.IsJSON() {
		return r.ScanJSON(v)
	} else if r.IsXML() {
		return r.ScanXML(v)
	} else if r.IsYAML() {
		return r.ScanYAML(v)
	}
	return fmt.Errorf("%w: %s", ErrUnsupportedContentType, r.ContentType())
}

// ScanJSON unmarshals the response body into a struct via JSON decoding.
func (r *Response) ScanJSON(v interface{}) error {
	return r.Client.JSONDecoder.Decode(bytes.NewReader(r.BodyBytes), v)
}

// ScanXML unmarshals the response body into a struct via XML decoding.
func (r *Response) ScanXML(v interface{}) error {
	return r.Client.XMLDecoder.Decode(bytes.NewReader(r.BodyBytes), v)
}

// ScanYAML unmarshals the response body into a struct via YAML decoding.
func (r *Response) ScanYAML(v interface{}) error {
	return r.Client.YAMLDecoder.Decode(bytes.NewReader(r.BodyBytes), v)
}

// Close closes the response body.
func (r *Response) Close() error {
	return r.RawResponse.Body.Close()
}

## middleware.go

package requests

import "net/http"

// Middleware is a function that intercepts or modifies requests and responses.
type Middleware func(http.RoundTripper) http.RoundTripper


=====================
As a golang development expert, you are good at the best implementation of golang development and are familiar with design patterns. Please reply first “I have read the above requests code. Please assign improvement tasks to me.”
