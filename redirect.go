package requests

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// RedirectPolicy is an interface that defines the Apply method
type RedirectPolicy interface {
	Apply(req *http.Request, via []*http.Request) error
}

// ProhibitRedirectPolicy is a redirect policy that does not allow any redirects
type ProhibitRedirectPolicy struct {
}

func NewProhibitRedirectPolicy() *ProhibitRedirectPolicy {
	return &ProhibitRedirectPolicy{}
}

// Apply is a method that implements the RedirectPolicy interface
func (p *ProhibitRedirectPolicy) Apply(req *http.Request, via []*http.Request) error {
	return ErrAutoRedirectDisabled
}

// AllowRedirectPolicy is a redirect policy that allows a flexible number of redirects
type AllowRedirectPolicy struct {
	numberRedirects int
}

// New is a method that creates a new AllowRedirectPolicy
func NewAllowRedirectPolicy(numberRedirects int) *AllowRedirectPolicy {
	return &AllowRedirectPolicy{numberRedirects: numberRedirects}
}

// Apply is a method that implements the RedirectPolicy interface
func (a *AllowRedirectPolicy) Apply(req *http.Request, via []*http.Request) error {
	if len(via) >= a.numberRedirects {
		return fmt.Errorf("stopped after %d redirects: %w", a.numberRedirects, ErrTooManyRedirects)
	}
	checkHostAndAddHeaders(req, via[0])
	return nil
}

// getHostname is a helper function that returns the hostname of the request
func getHostname(host string) (hostname string) {
	if strings.Index(host, ":") > 0 {
		host, _, _ = net.SplitHostPort(host)
	}
	hostname = strings.ToLower(host)
	return
}

// RedirectSpecifiedDomainPolicy is a redirect policy that checks if the redirect is allowed based on the hostnames
type RedirectSpecifiedDomainPolicy struct {
	domains []string
}

// New is a method that creates a new RedirectSpecifiedDomainPolicy
func NewRedirectSpecifiedDomainPolicy(domains ...string) *RedirectSpecifiedDomainPolicy {
	return &RedirectSpecifiedDomainPolicy{domains: domains}
}

// Apply is a method that implements the RedirectPolicy interface
func (s *RedirectSpecifiedDomainPolicy) Apply(req *http.Request, via []*http.Request) error {
	hosts := make(map[string]bool)
	for _, h := range s.domains {
		hosts[strings.ToLower(h)] = true
	}
	if ok := hosts[getHostname(req.URL.Host)]; !ok {
		return ErrRedirectNotAllowed
	}

	return nil
}

// checkHostAndAddHeaders is a helper function that checks if the hostnames are the same and adds the headers
func checkHostAndAddHeaders(cur *http.Request, pre *http.Request) {
	curHostname := getHostname(cur.URL.Host)
	preHostname := getHostname(pre.URL.Host)
	if strings.EqualFold(curHostname, preHostname) {
		for key, val := range pre.Header {
			cur.Header[key] = val
		}
	}
}
