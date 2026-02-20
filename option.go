package requests

import (
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// ClientOption configures a Client. Use with New().
type ClientOption func(*Client)

// WithBaseURL sets the base URL for the client.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) { c.SetBaseURL(baseURL) }
}

// WithTimeout sets the default timeout for the client.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) { c.SetDefaultTimeout(timeout) }
}

// WithHeader sets a default header on the client.
func WithHeader(key, value string) ClientOption {
	return func(c *Client) { c.SetDefaultHeader(key, value) }
}

// WithHeaders sets all default headers on the client.
func WithHeaders(headers *http.Header) ClientOption {
	return func(c *Client) { c.SetDefaultHeaders(headers) }
}

// WithContentType sets the default Content-Type header.
func WithContentType(contentType string) ClientOption {
	return func(c *Client) { c.SetDefaultContentType(contentType) }
}

// WithAccept sets the default Accept header.
func WithAccept(accept string) ClientOption {
	return func(c *Client) { c.SetDefaultAccept(accept) }
}

// WithUserAgent sets the default User-Agent header.
func WithUserAgent(userAgent string) ClientOption {
	return func(c *Client) { c.SetDefaultUserAgent(userAgent) }
}

// WithReferer sets the default Referer header.
func WithReferer(referer string) ClientOption {
	return func(c *Client) { c.SetDefaultReferer(referer) }
}

// WithCookies sets default cookies on the client.
func WithCookies(cookies map[string]string) ClientOption {
	return func(c *Client) { c.SetDefaultCookies(cookies) }
}

// WithCookieJar sets the cookie jar for the client.
func WithCookieJar(jar *cookiejar.Jar) ClientOption {
	return func(c *Client) { c.SetDefaultCookieJar(jar) }
}

// WithAuth sets the authentication method for the client.
func WithAuth(auth AuthMethod) ClientOption {
	return func(c *Client) { c.SetAuth(auth) }
}

// WithBasicAuth sets HTTP Basic Authentication credentials.
func WithBasicAuth(username, password string) ClientOption {
	return func(c *Client) {
		c.SetAuth(BasicAuth{Username: username, Password: password})
	}
}

// WithBearerAuth sets a Bearer token for authentication.
func WithBearerAuth(token string) ClientOption {
	return func(c *Client) { c.SetAuth(BearerAuth{Token: token}) }
}

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(maxRetries int) ClientOption {
	return func(c *Client) { c.SetMaxRetries(maxRetries) }
}

// WithRetryStrategy sets the backoff strategy for retries.
func WithRetryStrategy(strategy BackoffStrategy) ClientOption {
	return func(c *Client) { c.SetRetryStrategy(strategy) }
}

// WithRetryIf sets the custom retry condition function.
func WithRetryIf(retryIf RetryIfFunc) ClientOption {
	return func(c *Client) { c.SetRetryIf(retryIf) }
}

// WithMiddleware adds middleware to the client.
func WithMiddleware(middlewares ...Middleware) ClientOption {
	return func(c *Client) { c.AddMiddleware(middlewares...) }
}

// WithTLSConfig sets the TLS configuration for the client.
func WithTLSConfig(config *tls.Config) ClientOption {
	return func(c *Client) { c.SetTLSConfig(config) }
}

// WithInsecureSkipVerify configures the client to skip TLS certificate verification.
func WithInsecureSkipVerify() ClientOption {
	return func(c *Client) { c.InsecureSkipVerify() }
}

// WithCertificates sets TLS client certificates.
func WithCertificates(certs ...tls.Certificate) ClientOption {
	return func(c *Client) { c.SetCertificates(certs...) }
}

// WithRootCertificate sets the root certificate from a PEM file path.
func WithRootCertificate(pemFilePath string) ClientOption {
	return func(c *Client) { c.SetRootCertificate(pemFilePath) }
}

// WithRootCertificateFromString sets the root certificate from a PEM string.
func WithRootCertificateFromString(pemCerts string) ClientOption {
	return func(c *Client) { c.SetRootCertificateFromString(pemCerts) }
}

// WithTransport sets the HTTP transport for the client.
func WithTransport(transport http.RoundTripper) ClientOption {
	return func(c *Client) { c.SetDefaultTransport(transport) }
}

// WithHTTPClient sets the underlying http.Client.
// When combined with transport-modifying options (WithProxy, WithDialTimeout, etc.),
// place WithHTTPClient first since it replaces the entire http.Client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) { c.SetHTTPClient(httpClient) }
}

// WithDialTimeout sets the TCP connection timeout on the underlying transport.
func WithDialTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.SetDialTimeout(d) }
}

// WithTLSHandshakeTimeout sets the TLS handshake timeout on the underlying transport.
func WithTLSHandshakeTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.SetTLSHandshakeTimeout(d) }
}

// WithResponseHeaderTimeout sets the time to wait for response headers.
func WithResponseHeaderTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.SetResponseHeaderTimeout(d) }
}

// WithMaxIdleConns sets the maximum number of idle connections across all hosts.
func WithMaxIdleConns(n int) ClientOption {
	return func(c *Client) { c.SetMaxIdleConns(n) }
}

// WithMaxIdleConnsPerHost sets the maximum number of idle connections per host.
func WithMaxIdleConnsPerHost(n int) ClientOption {
	return func(c *Client) { c.SetMaxIdleConnsPerHost(n) }
}

// WithMaxConnsPerHost sets the maximum total number of connections per host.
func WithMaxConnsPerHost(n int) ClientOption {
	return func(c *Client) { c.SetMaxConnsPerHost(n) }
}

// WithIdleConnTimeout sets how long idle connections remain in the pool.
func WithIdleConnTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.SetIdleConnTimeout(d) }
}

// WithRedirectPolicy sets the redirect policy for the client.
func WithRedirectPolicy(policies ...RedirectPolicy) ClientOption {
	return func(c *Client) { c.SetRedirectPolicy(policies...) }
}

// WithProxy sets the proxy URL for the client.
// Parse errors are silently ignored to maintain the fluent pattern;
// use Client.SetProxy() directly for error handling.
func WithProxy(proxyURL string) ClientOption {
	return func(c *Client) { _ = c.SetProxy(proxyURL) }
}

// WithLogger sets the logger for the client.
func WithLogger(logger Logger) ClientOption {
	return func(c *Client) { c.SetLogger(logger) }
}

// WithJSONMarshal sets a custom JSON marshal function.
func WithJSONMarshal(marshalFunc func(v any) ([]byte, error)) ClientOption {
	return func(c *Client) { c.SetJSONMarshal(marshalFunc) }
}

// WithJSONUnmarshal sets a custom JSON unmarshal function.
func WithJSONUnmarshal(unmarshalFunc func(data []byte, v any) error) ClientOption {
	return func(c *Client) { c.SetJSONUnmarshal(unmarshalFunc) }
}

// WithXMLMarshal sets a custom XML marshal function.
func WithXMLMarshal(marshalFunc func(v any) ([]byte, error)) ClientOption {
	return func(c *Client) { c.SetXMLMarshal(marshalFunc) }
}

// WithXMLUnmarshal sets a custom XML unmarshal function.
func WithXMLUnmarshal(unmarshalFunc func(data []byte, v any) error) ClientOption {
	return func(c *Client) { c.SetXMLUnmarshal(unmarshalFunc) }
}

// WithYAMLMarshal sets a custom YAML marshal function.
func WithYAMLMarshal(marshalFunc func(v any) ([]byte, error)) ClientOption {
	return func(c *Client) { c.SetYAMLMarshal(marshalFunc) }
}

// WithYAMLUnmarshal sets a custom YAML unmarshal function.
func WithYAMLUnmarshal(unmarshalFunc func(data []byte, v any) error) ClientOption {
	return func(c *Client) { c.SetYAMLUnmarshal(unmarshalFunc) }
}
