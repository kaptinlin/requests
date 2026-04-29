package fingerprint

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"slices"

	"github.com/kaptinlin/requests"
	utls "github.com/refraction-networking/utls"
)

type profile struct {
	name      string
	helloID   utls.ClientHelloID
	tlsConfig *tls.Config
}

// Option configures a TLS fingerprint profile.
type Option func(*profile)

// WithTLSConfig sets the TLS configuration used by the uTLS handshake.
func WithTLSConfig(config *tls.Config) Option {
	return func(p *profile) {
		p.tlsConfig = config
	}
}

// Chrome returns the Chrome TLS fingerprint preset provided by uTLS.
// The exact browser version is controlled by the uTLS dependency.
func Chrome(opts ...Option) requests.Profile {
	return Custom("Chrome", utls.HelloChrome_Auto, opts...)
}

// Firefox returns the Firefox TLS fingerprint preset provided by uTLS.
// The exact browser version is controlled by the uTLS dependency.
func Firefox(opts ...Option) requests.Profile {
	return Custom("Firefox", utls.HelloFirefox_Auto, opts...)
}

// Custom returns a TLS fingerprint profile for helloID.
func Custom(name string, helloID utls.ClientHelloID, opts ...Option) requests.Profile {
	p := profile{
		name:    name,
		helloID: helloID,
	}
	for _, opt := range opts {
		opt(&p)
	}
	if p.name == "" {
		p.name = helloName(helloID)
	}
	return p
}

func (p profile) Name() string {
	return p.name
}

func (p profile) Apply(c *requests.Client) error {
	if c == nil {
		return fmt.Errorf("%w: client", requests.ErrInvalidConfigValue)
	}
	if isEmptyHelloID(p.helloID) {
		return fmt.Errorf("%w: hello id", requests.ErrInvalidConfigValue)
	}

	client := c.GetHTTPClient()
	var transport *http.Transport
	switch current := client.Transport.(type) {
	case nil:
		transport = &http.Transport{}
		c.SetDefaultTransport(transport)
	case *http.Transport:
		transport = current
	default:
		return fmt.Errorf("%w: expected *http.Transport, got %T", requests.ErrInvalidTransportType, current)
	}

	if p.tlsConfig != nil {
		c.SetTLSConfig(p.tlsConfig)
	}
	return ConfigureTransport(transport, p.helloID)
}

func helloName(helloID utls.ClientHelloID) string {
	if isEmptyHelloID(helloID) {
		return "TLS"
	}
	return (&helloID).Str()
}

func isEmptyHelloID(helloID utls.ClientHelloID) bool {
	return helloID.Client == "" && helloID.Version == ""
}

func ensureNextProtos(config *tls.Config) {
	nextProtos := slices.Clone(config.NextProtos)
	if !slices.Contains(nextProtos, "h2") {
		nextProtos = slices.Insert(nextProtos, 0, "h2")
	}
	if !slices.Contains(nextProtos, "http/1.1") {
		nextProtos = append(nextProtos, "http/1.1")
	}
	config.NextProtos = nextProtos
}
