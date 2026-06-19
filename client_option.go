package requests

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/kaptinlin/orderedobject"
)

// Option configures a Client during construction.
type Option func(*Client) error

func invalidOptionValue(name string) error {
	return fmt.Errorf("%w: %s", ErrInvalidConfigValue, name)
}

func validateDurationOption(name string, value time.Duration) error {
	if value < 0 {
		return invalidOptionValue(name)
	}
	return nil
}

func validateIntOption(name string, value int) error {
	if value < 0 {
		return invalidOptionValue(name)
	}
	return nil
}

func validateEncoderOption(name string, encoder Encoder) error {
	if encoder == nil {
		return invalidOptionValue(name)
	}
	return nil
}

func validateDecoderOption(name string, decoder Decoder) error {
	if decoder == nil {
		return invalidOptionValue(name)
	}
	return nil
}

// WithBaseURL sets the base URL for the client.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) error {
		if baseURL != "" {
			if _, err := url.Parse(baseURL); err != nil {
				return fmt.Errorf("invalid BaseURL: %w", err)
			}
		}
		c.setBaseURL(baseURL)
		return nil
	}
}

// WithTimeout sets the default timeout for the client.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) error {
		if err := validateDurationOption("Timeout", timeout); err != nil {
			return err
		}
		c.setDefaultTimeout(timeout)
		return nil
	}
}

// WithHeader sets a default header on the client.
func WithHeader(key, value string) Option {
	return func(c *Client) error {
		c.setDefaultHeader(key, value)
		return nil
	}
}

// WithHeaders sets all default headers on the client.
func WithHeaders(headers *http.Header) Option {
	return func(c *Client) error {
		c.setDefaultHeaders(headers)
		return nil
	}
}

// WithOrderedHeaders sets ordered default headers on the client.
func WithOrderedHeaders(headers *orderedobject.Object[[]string]) Option {
	return func(c *Client) error {
		c.setDefaultOrderedHeaders(headers)
		return nil
	}
}

// WithContentType sets the default Content-Type header.
func WithContentType(contentType string) Option {
	return func(c *Client) error {
		c.setDefaultContentType(contentType)
		return nil
	}
}

// WithAccept sets the default Accept header.
func WithAccept(accept string) Option {
	return func(c *Client) error {
		c.setDefaultAccept(accept)
		return nil
	}
}

// WithUserAgent sets the default User-Agent header.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) error {
		c.setDefaultUserAgent(userAgent)
		return nil
	}
}

// WithReferer sets the default Referer header.
func WithReferer(referer string) Option {
	return func(c *Client) error {
		c.setDefaultReferer(referer)
		return nil
	}
}

// WithCookies sets default cookies on the client.
func WithCookies(cookies map[string]string) Option {
	return func(c *Client) error {
		c.setDefaultCookies(cookies)
		return nil
	}
}

// WithCookieJar sets the cookie jar for the client.
func WithCookieJar(jar *cookiejar.Jar) Option {
	return func(c *Client) error {
		c.setDefaultCookieJar(jar)
		return nil
	}
}

// WithSession enables cookie and TLS session reuse.
func WithSession() Option {
	return func(c *Client) error {
		c.enableSession()
		return nil
	}
}

// WithAuth sets the authentication method for the client.
func WithAuth(auth AuthMethod) Option {
	return func(c *Client) error {
		c.setAuth(auth)
		return nil
	}
}

// WithBasicAuth sets HTTP Basic Authentication credentials.
func WithBasicAuth(username, password string) Option {
	return func(c *Client) error {
		c.setAuth(BasicAuth{Username: username, Password: password})
		return nil
	}
}

// WithBearerAuth sets a Bearer token for authentication.
func WithBearerAuth(token string) Option {
	return func(c *Client) error {
		c.setAuth(BearerAuth{Token: token})
		return nil
	}
}

// WithRetry sets the default retry policy.
func WithRetry(policy RetryPolicy) Option {
	return func(c *Client) error {
		if err := validateIntOption("Retry.Max", policy.Max); err != nil {
			return err
		}
		c.setRetry(policy)
		return nil
	}
}

// WithoutRetry disables retry attempts.
func WithoutRetry() Option {
	return func(c *Client) error {
		c.setRetry(RetryPolicy{})
		return nil
	}
}

// WithMiddleware adds middleware to the client.
func WithMiddleware(middlewares ...Middleware) Option {
	return func(c *Client) error {
		c.addMiddleware(middlewares...)
		return nil
	}
}

// WithProfile applies a coherent client identity profile.
func WithProfile(profile Profile) Option {
	return func(c *Client) error {
		return applyProfileOptions(c, profile)
	}
}

// WithTLSConfig sets the TLS configuration for the client.
func WithTLSConfig(config *tls.Config) Option {
	return func(c *Client) error {
		c.setTLSConfig(config)
		return nil
	}
}

// WithInsecureSkipVerify configures the client to skip TLS certificate verification.
func WithInsecureSkipVerify() Option {
	return func(c *Client) error {
		c.insecureSkipVerify()
		return nil
	}
}

// WithCertificates sets TLS client certificates.
func WithCertificates(certs ...tls.Certificate) Option {
	return func(c *Client) error {
		c.setCertificates(certs...)
		return nil
	}
}

// WithClientCertificate loads and sets a client certificate and key from file paths.
func WithClientCertificate(certFile, keyFile string) Option {
	return func(c *Client) error {
		cert, err := tls.LoadX509KeyPair(filepath.Clean(certFile), filepath.Clean(keyFile))
		if err != nil {
			return fmt.Errorf("load client certificate: %w", err)
		}
		c.setCertificates(cert)
		return nil
	}
}

// WithTLSServerName sets the TLS server name (SNI).
func WithTLSServerName(serverName string) Option {
	return func(c *Client) error {
		c.setTLSServerName(serverName)
		return nil
	}
}

// WithRootCertificate sets the root certificate from a PEM file path.
func WithRootCertificate(pemFilePath string) Option {
	return func(c *Client) error {
		rootPemData, err := os.ReadFile(filepath.Clean(pemFilePath))
		if err != nil {
			return fmt.Errorf("read root certificate: %w", err)
		}
		c.addRootCAs(rootPemData)
		return nil
	}
}

// WithRootCertificateFromString sets the root certificate from a PEM string.
func WithRootCertificateFromString(pemCerts string) Option {
	return func(c *Client) error {
		c.setRootCertificateFromString(pemCerts)
		return nil
	}
}

// WithTransport sets the HTTP transport for the client.
func WithTransport(transport http.RoundTripper) Option {
	return func(c *Client) error {
		c.setDefaultTransport(transport)
		return nil
	}
}

// WithHTTP2 enables HTTP/2 transport support.
func WithHTTP2() Option {
	return func(c *Client) error {
		c.enableHTTP2()
		return nil
	}
}

// WithHTTPClient sets the underlying http.Client.
// When combined with transport-modifying options (WithProxy, WithDialTimeout, etc.),
// place WithHTTPClient first since it replaces the entire http.Client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) error {
		c.setHTTPClient(httpClient)
		return nil
	}
}

// WithDialTimeout sets the TCP connection timeout on the underlying transport.
func WithDialTimeout(d time.Duration) Option {
	return func(c *Client) error {
		if err := validateDurationOption("DialTimeout", d); err != nil {
			return err
		}
		c.setDialTimeout(d)
		return nil
	}
}

// WithResolver sets the resolver used by the default transport dialer.
func WithResolver(resolver *net.Resolver) Option {
	return func(c *Client) error {
		c.setResolver(resolver)
		return nil
	}
}

// WithDialContext sets the dial function on the underlying transport.
func WithDialContext(dial func(context.Context, string, string) (net.Conn, error)) Option {
	return func(c *Client) error {
		c.setDialContext(dial)
		return nil
	}
}

// WithLocalAddr sets the local address used by the default transport dialer.
func WithLocalAddr(addr net.Addr) Option {
	return func(c *Client) error {
		c.setLocalAddr(addr)
		return nil
	}
}

// WithTLSHandshakeTimeout sets the TLS handshake timeout on the underlying transport.
func WithTLSHandshakeTimeout(d time.Duration) Option {
	return func(c *Client) error {
		if err := validateDurationOption("TLSHandshakeTimeout", d); err != nil {
			return err
		}
		c.setTLSHandshakeTimeout(d)
		return nil
	}
}

// WithResponseHeaderTimeout sets the time to wait for response headers.
func WithResponseHeaderTimeout(d time.Duration) Option {
	return func(c *Client) error {
		if err := validateDurationOption("ResponseHeaderTimeout", d); err != nil {
			return err
		}
		c.setResponseHeaderTimeout(d)
		return nil
	}
}

// WithMaxIdleConns sets the maximum number of idle connections across all hosts.
func WithMaxIdleConns(n int) Option {
	return func(c *Client) error {
		if err := validateIntOption("MaxIdleConns", n); err != nil {
			return err
		}
		c.setMaxIdleConns(n)
		return nil
	}
}

// WithMaxIdleConnsPerHost sets the maximum number of idle connections per host.
func WithMaxIdleConnsPerHost(n int) Option {
	return func(c *Client) error {
		if err := validateIntOption("MaxIdleConnsPerHost", n); err != nil {
			return err
		}
		c.setMaxIdleConnsPerHost(n)
		return nil
	}
}

// WithMaxConnsPerHost sets the maximum total number of connections per host.
func WithMaxConnsPerHost(n int) Option {
	return func(c *Client) error {
		if err := validateIntOption("MaxConnsPerHost", n); err != nil {
			return err
		}
		c.setMaxConnsPerHost(n)
		return nil
	}
}

// WithIdleConnTimeout sets how long idle connections remain in the pool.
func WithIdleConnTimeout(d time.Duration) Option {
	return func(c *Client) error {
		if err := validateDurationOption("IdleConnTimeout", d); err != nil {
			return err
		}
		c.setIdleConnTimeout(d)
		return nil
	}
}

// WithRedirectPolicy sets the redirect policy for the client.
func WithRedirectPolicy(policies ...RedirectPolicy) Option {
	return func(c *Client) error {
		c.setRedirectPolicy(policies...)
		return nil
	}
}

// WithProxy sets the proxy URL for the client.
func WithProxy(proxyURL string) Option {
	return func(c *Client) error {
		return c.setProxy(proxyURL)
	}
}

// WithProxyBypass sets a proxy URL with a NO_PROXY-style bypass list.
func WithProxyBypass(proxyURL, bypass string) Option {
	return func(c *Client) error {
		return c.setProxyWithBypass(proxyURL, bypass)
	}
}

// WithProxyFromEnv uses proxy settings from HTTP_PROXY, HTTPS_PROXY, and NO_PROXY.
func WithProxyFromEnv() Option {
	return func(c *Client) error {
		return c.setProxyFromEnv()
	}
}

// WithProxies sets multiple proxies with round-robin rotation.
func WithProxies(proxyURLs ...string) Option {
	return func(c *Client) error {
		return c.setProxies(proxyURLs...)
	}
}

// WithProxySelector sets a custom proxy selection function.
func WithProxySelector(selector func(*http.Request) (*url.URL, error)) Option {
	return func(c *Client) error {
		return c.setProxySelector(selector)
	}
}

// WithoutProxy clears any configured proxy.
func WithoutProxy() Option {
	return func(c *Client) error {
		c.removeProxy()
		return nil
	}
}

// WithLogger sets the logger for the client.
func WithLogger(logger Logger) Option {
	return func(c *Client) error {
		c.setLogger(logger)
		return nil
	}
}

// WithJSONEncoder sets the JSON encoder.
func WithJSONEncoder(encoder Encoder) Option {
	return func(c *Client) error {
		if err := validateEncoderOption("JSONEncoder", encoder); err != nil {
			return err
		}
		c.jsonEncoder = encoder
		return nil
	}
}

// WithJSONDecoder sets the JSON decoder.
func WithJSONDecoder(decoder Decoder) Option {
	return func(c *Client) error {
		if err := validateDecoderOption("JSONDecoder", decoder); err != nil {
			return err
		}
		c.jsonDecoder = decoder
		return nil
	}
}

// WithXMLEncoder sets the XML encoder.
func WithXMLEncoder(encoder Encoder) Option {
	return func(c *Client) error {
		if err := validateEncoderOption("XMLEncoder", encoder); err != nil {
			return err
		}
		c.xmlEncoder = encoder
		return nil
	}
}

// WithXMLDecoder sets the XML decoder.
func WithXMLDecoder(decoder Decoder) Option {
	return func(c *Client) error {
		if err := validateDecoderOption("XMLDecoder", decoder); err != nil {
			return err
		}
		c.xmlDecoder = decoder
		return nil
	}
}

// WithYAMLEncoder sets the YAML encoder.
func WithYAMLEncoder(encoder Encoder) Option {
	return func(c *Client) error {
		if err := validateEncoderOption("YAMLEncoder", encoder); err != nil {
			return err
		}
		c.yamlEncoder = encoder
		return nil
	}
}

// WithYAMLDecoder sets the YAML decoder.
func WithYAMLDecoder(decoder Decoder) Option {
	return func(c *Client) error {
		if err := validateDecoderOption("YAMLDecoder", decoder); err != nil {
			return err
		}
		c.yamlDecoder = decoder
		return nil
	}
}
