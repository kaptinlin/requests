package requests

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// RedirectPolicy is an interface that defines the Apply method
type RedirectPolicy interface {
	Apply(req *http.Request, via []*http.Request) error
}

// NoRedirectPolicy is a redirect policy that does not allow any redirects
type NoRedirectPolicy struct {
}

func NewNoRedirectPolicy() *NoRedirectPolicy {
	return &NoRedirectPolicy{}
}

// Apply is a method that implements the RedirectPolicy interface
func (n *NoRedirectPolicy) Apply(req *http.Request, via []*http.Request) error {
	return ErrAutoRedirectDisabled
}

// FlexibleRedirectPolicy is a redirect policy that allows a flexible number of redirects
type FlexibleRedirectPolicy struct {
	noOfRedirect int
}

// New is a method that creates a new FlexibleRedirectPolicy
func NewFlexibleRedirectPolicy(noOfRedirect int) *FlexibleRedirectPolicy {
	return &FlexibleRedirectPolicy{noOfRedirect: noOfRedirect}
}

// Apply is a method that implements the RedirectPolicy interface
func (f *FlexibleRedirectPolicy) Apply(req *http.Request, via []*http.Request) error {
	if len(via) >= f.noOfRedirect {
		return fmt.Errorf("stopped after %d redirects", f.noOfRedirect)
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

// DomainCheckRedirectPolicy is a redirect policy that checks if the redirect is allowed based on the hostnames
type DomainCheckRedirectPolicy struct {
	hostnames []string
}

// New is a method that creates a new DomainCheckRedirectPolicy
func NewDomainCheckRedirectPolicy(hostnames ...string) *DomainCheckRedirectPolicy {
	return &DomainCheckRedirectPolicy{hostnames: hostnames}
}

// Apply is a method that implements the RedirectPolicy interface
func (d *DomainCheckRedirectPolicy) Apply(req *http.Request, via []*http.Request) error {
	hosts := make(map[string]bool)
	for _, h := range d.hostnames {
		hosts[strings.ToLower(h)] = true
	}
	if ok := hosts[getHostname(req.URL.Host)]; !ok {
		return errors.New("redirect is not allowed as per DomainCheckRedirectPolicy")
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
