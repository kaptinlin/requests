package requests

import (
	"fmt"
	"maps"
	"net"
	"net/http"
	"strings"
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

// RedirectPolicy is an interface that defines the Apply method.
type RedirectPolicy interface {
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
func (p *ProhibitRedirectPolicy) Apply(req *http.Request, via []*http.Request) error {
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
	checkHostAndAddHeaders(req, via[0])
	return nil
}

// SmartRedirectPolicy is a redirect policy that downgrades POST to GET on 301/302/303
// redirects and strips sensitive headers on cross-host or scheme-downgrade redirects.
type SmartRedirectPolicy struct {
	maxRedirects int
}

// NewSmartRedirectPolicy creates a new SmartRedirectPolicy with the given redirect limit.
func NewSmartRedirectPolicy(maxRedirects int) *SmartRedirectPolicy {
	return &SmartRedirectPolicy{maxRedirects: maxRedirects}
}

// Apply enforces the redirect limit, performs method downgrade for 301/302/303, and
// strips sensitive headers on cross-host or HTTPS-to-HTTP redirects.
func (s *SmartRedirectPolicy) Apply(req *http.Request, via []*http.Request) error {
	if len(via) >= s.maxRedirects {
		return fmt.Errorf("stopped after %d redirects: %w", s.maxRedirects, ErrTooManyRedirects)
	}

	prev := via[len(via)-1]
	checkHostAndAddHeaders(req, prev)

	if prev.Response != nil {
		switch prev.Response.StatusCode {
		case http.StatusMovedPermanently, http.StatusFound: // 301, 302
			if req.Method == http.MethodPost {
				req.Method = http.MethodGet
				req.Body = nil
				req.ContentLength = 0
				dropPayloadHeaders(req.Header)
			}
		case http.StatusSeeOther: // 303
			if req.Method != http.MethodHead {
				req.Method = http.MethodGet
				req.Body = nil
				req.ContentLength = 0
				dropPayloadHeaders(req.Header)
			}
		}
	}

	return nil
}

// dropPayloadHeaders removes headers related to request body content.
func dropPayloadHeaders(h http.Header) {
	h.Del("Content-Type")
	h.Del("Content-Length")
	h.Del("Content-Encoding")
	h.Del("Transfer-Encoding")
}

// getHostname extracts the hostname from a host string, removing any port number.
func getHostname(host string) string {
	if strings.Contains(host, ":") {
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
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

// checkHostAndAddHeaders copies headers from the previous request when the host and scheme match.
// On cross-host or HTTPS-to-HTTP redirects, sensitive headers are stripped instead.
func checkHostAndAddHeaders(cur *http.Request, pre *http.Request) {
	sameHost := strings.EqualFold(getHostname(cur.URL.Host), getHostname(pre.URL.Host))
	schemeDowngrade := pre.URL.Scheme == "https" && cur.URL.Scheme == "http"

	if sameHost && !schemeDowngrade {
		maps.Copy(cur.Header, pre.Header)
	} else {
		stripSensitiveHeaders(cur.Header)
	}
}
