package requests

import (
	"fmt"
	"maps"
	"net"
	"net/http"
	"strings"
)

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
	domains []string
}

// NewRedirectSpecifiedDomainPolicy creates a new RedirectSpecifiedDomainPolicy that only allows redirects to the specified domains.
func NewRedirectSpecifiedDomainPolicy(domains ...string) *RedirectSpecifiedDomainPolicy {
	return &RedirectSpecifiedDomainPolicy{domains: domains}
}

// Apply checks if the redirect target domain is in the allowed domains list.
func (s *RedirectSpecifiedDomainPolicy) Apply(req *http.Request, via []*http.Request) error {
	// Pre-allocate with expected size for better performance (Go 1.24+ Swiss Tables)
	hosts := make(map[string]bool, len(s.domains))
	for _, h := range s.domains {
		hosts[strings.ToLower(h)] = true
	}
	if ok := hosts[getHostname(req.URL.Host)]; !ok {
		return ErrRedirectNotAllowed
	}

	return nil
}

// checkHostAndAddHeaders is a helper function that checks if the hostnames are the same and adds the headers.
func checkHostAndAddHeaders(cur *http.Request, pre *http.Request) {
	curHostname := getHostname(cur.URL.Host)
	preHostname := getHostname(pre.URL.Host)
	if strings.EqualFold(curHostname, preHostname) {
		maps.Copy(cur.Header, pre.Header)
	}
}
