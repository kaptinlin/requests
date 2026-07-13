package requests

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"golang.org/x/net/idna"
)

// sensitiveHeaders are headers that should be stripped when redirecting across hosts or
// downgrading from HTTPS to HTTP.
var sensitiveHeaders = []string{
	"Authorization",
	"Cookie",
	"Cookie2",
	"Proxy-Authorization",
	"Www-Authenticate",
}

// RedirectPolicy applies redirect behavior to outgoing requests.
type RedirectPolicy interface {
	// Apply applies the redirect policy to req based on the prior requests in via.
	Apply(req *http.Request, via []*http.Request) error
}

// ProhibitRedirectPolicy is a redirect policy that does not allow any redirects.
type ProhibitRedirectPolicy struct {
}

// NewProhibitRedirectPolicy creates a new ProhibitRedirectPolicy that prevents any redirects.
func NewProhibitRedirectPolicy() *ProhibitRedirectPolicy {
	return &ProhibitRedirectPolicy{}
}

// Apply rejects all redirects by returning ErrAutoRedirectDisabled.
func (p *ProhibitRedirectPolicy) Apply(_ *http.Request, _ []*http.Request) error {
	return ErrAutoRedirectDisabled
}

// AllowRedirectPolicy is a redirect policy that allows a flexible number of redirects.
type AllowRedirectPolicy struct {
	numberRedirects int
}

// NewAllowRedirectPolicy creates a new AllowRedirectPolicy that allows up to the specified number of redirects.
func NewAllowRedirectPolicy(numberRedirects int) *AllowRedirectPolicy {
	return &AllowRedirectPolicy{numberRedirects: numberRedirects}
}

// Apply allows redirects up to the configured limit, returning ErrTooManyRedirects if exceeded.
func (a *AllowRedirectPolicy) Apply(req *http.Request, via []*http.Request) error {
	if len(via) >= a.numberRedirects {
		return fmt.Errorf("stopped after %d redirects: %w", a.numberRedirects, ErrTooManyRedirects)
	}
	stripSensitiveHeadersOnRedirect(req, via[0])
	return nil
}

// getHostname extracts the hostname from a host string, removing any port number.
func getHostname(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(host)
}

// RedirectSpecifiedDomainPolicy is a redirect policy that checks if the redirect is allowed based on the hostnames.
type RedirectSpecifiedDomainPolicy struct {
	allowedHosts map[string]bool
}

// NewRedirectSpecifiedDomainPolicy creates a new RedirectSpecifiedDomainPolicy that only allows redirects to the specified domains.
func NewRedirectSpecifiedDomainPolicy(domains ...string) *RedirectSpecifiedDomainPolicy {
	hosts := make(map[string]bool, len(domains))
	for _, h := range domains {
		hosts[strings.ToLower(h)] = true
	}
	return &RedirectSpecifiedDomainPolicy{allowedHosts: hosts}
}

// Apply checks if the redirect target domain is in the allowed domains list.
func (s *RedirectSpecifiedDomainPolicy) Apply(req *http.Request, _ []*http.Request) error {
	if !s.allowedHosts[getHostname(req.URL.Host)] {
		return ErrRedirectNotAllowed
	}
	return nil
}

// stripSensitiveHeaders removes sensitive headers from the given header map.
func stripSensitiveHeaders(h http.Header) {
	for _, header := range sensitiveHeaders {
		h.Del(header)
	}
}

func stripSensitiveHeadersOnRedirect(cur *http.Request, pre *http.Request) {
	if sameOrigin(cur.URL, pre.URL) {
		return
	}

	stripSensitiveHeaders(cur.Header)
}

func sameOrigin(left, right *url.URL) bool {
	leftHostname, leftOK := canonicalHostname(left)
	rightHostname, rightOK := canonicalHostname(right)

	return strings.EqualFold(left.Scheme, right.Scheme) &&
		leftOK && rightOK &&
		leftHostname == rightHostname &&
		effectivePort(left) == effectivePort(right)
}

func canonicalHostname(u *url.URL) (string, bool) {
	hostname := u.Hostname()
	if hostname == "" {
		return "", false
	}
	if addr, err := netip.ParseAddr(hostname); err == nil {
		return addr.String(), true
	}

	hostname, err := idna.Lookup.ToASCII(hostname)
	if err != nil {
		return "", false
	}
	return strings.ToLower(hostname), true
}

func effectivePort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}

	switch strings.ToLower(u.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}
