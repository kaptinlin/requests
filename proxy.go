package requests

import (
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
)

type noProxy struct {
	domains  []string // domain names (with optional leading dot for subdomain matching)
	ips      []net.IP
	cidrs    []*net.IPNet
	wildcard bool
}

// parseNoProxy parses a comma-separated NO_PROXY string into a noProxy struct.
// Supported entry formats: domain names (with optional leading dot), IP addresses,
// CIDR subnets, and "*" for wildcard (bypass all).
func parseNoProxy(bypass string) *noProxy {
	np := &noProxy{}
	for entry := range strings.SplitSeq(bypass, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if entry == "*" {
			np.wildcard = true
			return np
		}
		if _, cidr, err := net.ParseCIDR(entry); err == nil {
			np.cidrs = append(np.cidrs, cidr)
			continue
		}
		if ip := net.ParseIP(entry); ip != nil {
			np.ips = append(np.ips, ip)
			continue
		}
		np.domains = append(np.domains, strings.ToLower(entry))
	}
	return np
}

// matches checks if a host (hostname or IP, with optional port) matches any bypass rule.
func (np *noProxy) matches(host string) bool {
	if np == nil {
		return false
	}
	if np.wildcard {
		return true
	}

	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	hostname = strings.ToLower(hostname)

	if ip := net.ParseIP(hostname); ip != nil {
		for _, bypassIP := range np.ips {
			if bypassIP.Equal(ip) {
				return true
			}
		}
		for _, cidr := range np.cidrs {
			if cidr.Contains(ip) {
				return true
			}
		}
		return false
	}

	for _, domain := range np.domains {
		if suffix, ok := strings.CutPrefix(domain, "."); ok {
			if strings.HasSuffix(hostname, "."+suffix) {
				return true
			}
			continue
		}
		if hostname == domain || strings.HasSuffix(hostname, "."+domain) {
			return true
		}
	}

	return false
}

// verifyProxy validates the given proxy URL, supporting http, https, and socks5 schemes.
func verifyProxy(proxyURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	switch parsedURL.Scheme {
	case "http", "https", "socks5":
		return parsedURL, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedScheme, parsedURL.Scheme)
	}
}

// ensureTransport returns the client's transport as *http.Transport, creating one if needed.
// Must be called with c.mu held.
func (c *Client) ensureTransport() (*http.Transport, error) {
	if c.httpClient.Transport == nil {
		c.httpClient.Transport = &http.Transport{}
	}
	transport, ok := c.httpClient.Transport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("%w: expected *http.Transport, got %T", ErrInvalidTransportType, c.httpClient.Transport)
	}
	return transport, nil
}

// setProxy configures the client to use a proxy. Supports http, https, and socks5 proxies.
func (c *Client) setProxy(proxyURL string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	validatedProxyURL, err := verifyProxy(proxyURL)
	if err != nil {
		return err
	}

	transport, err := c.ensureTransport()
	if err != nil {
		return err
	}

	transport.Proxy = http.ProxyURL(validatedProxyURL)
	return nil
}

// setProxyWithBypass configures the client to use a proxy with a NO_PROXY bypass list.
// The bypass parameter is a comma-separated string of hosts that should not use the proxy.
// Supported formats: domain names, IPs, CIDR subnets, and "*" for wildcard.
func (c *Client) setProxyWithBypass(proxyURL, bypass string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	validatedProxyURL, err := verifyProxy(proxyURL)
	if err != nil {
		return err
	}

	transport, err := c.ensureTransport()
	if err != nil {
		return err
	}

	np := parseNoProxy(bypass)
	proxyFunc := http.ProxyURL(validatedProxyURL)

	transport.Proxy = func(req *http.Request) (*url.URL, error) {
		if np.matches(req.URL.Host) {
			return nil, nil
		}
		return proxyFunc(req)
	}
	return nil
}

// setProxyFromEnv configures the client to use proxy settings from environment variables
// (HTTP_PROXY, HTTPS_PROXY, NO_PROXY).
func (c *Client) setProxyFromEnv() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	transport, err := c.ensureTransport()
	if err != nil {
		return err
	}

	transport.Proxy = http.ProxyFromEnvironment
	return nil
}

// setProxies configures multiple proxies with round-robin rotation.
// Each outgoing request (including retries) picks the next proxy in order.
func (c *Client) setProxies(proxyURLs ...string) error {
	selector, err := RoundRobinProxies(proxyURLs...)
	if err != nil {
		return err
	}
	return c.setProxySelector(selector)
}

// setProxySelector sets a custom proxy selection function matching the
// http.Transport.Proxy signature. Return nil *url.URL for direct connection.
func (c *Client) setProxySelector(selector func(*http.Request) (*url.URL, error)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	transport, err := c.ensureTransport()
	if err != nil {
		return err
	}
	transport.Proxy = selector
	return nil
}

// verifyProxies validates and parses multiple proxy URLs.
func verifyProxies(proxyURLs []string) ([]*url.URL, error) {
	if len(proxyURLs) == 0 {
		return nil, ErrNoProxies
	}
	proxies := make([]*url.URL, len(proxyURLs))
	for i, raw := range proxyURLs {
		u, err := verifyProxy(raw)
		if err != nil {
			return nil, err
		}
		proxies[i] = u
	}
	return proxies, nil
}

// RoundRobinProxies returns a proxy function that cycles through proxies in order.
// Safe for concurrent use.
func RoundRobinProxies(proxyURLs ...string) (func(*http.Request) (*url.URL, error), error) {
	proxies, err := verifyProxies(proxyURLs)
	if err != nil {
		return nil, err
	}
	n := uint64(len(proxies))
	var counter atomic.Uint64
	return func(_ *http.Request) (*url.URL, error) {
		idx := counter.Add(1) - 1
		return proxies[idx%n], nil
	}, nil
}

// RandomProxies returns a proxy function that selects a random proxy for each request.
// Safe for concurrent use.
func RandomProxies(proxyURLs ...string) (func(*http.Request) (*url.URL, error), error) {
	proxies, err := verifyProxies(proxyURLs)
	if err != nil {
		return nil, err
	}
	n := len(proxies)
	return func(_ *http.Request) (*url.URL, error) {
		return proxies[rand.IntN(n)], nil //nolint:gosec // proxy rotation does not require cryptographic randomness
	}, nil
}

// removeProxy clears any configured proxy, allowing direct connections.
func (c *Client) removeProxy() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.httpClient.Transport == nil {
		return
	}

	transport, ok := c.httpClient.Transport.(*http.Transport)
	if !ok {
		return
	}

	transport.Proxy = nil
}
