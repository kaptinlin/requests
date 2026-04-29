package http3

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"maps"

	"github.com/kaptinlin/requests"
	"github.com/quic-go/quic-go"
	qhttp3 "github.com/quic-go/quic-go/http3"
)

type settings struct {
	tlsConfig              *tls.Config
	quicConfig             *quic.Config
	enableDatagrams        bool
	additionalSettings     map[uint64]uint64
	maxResponseHeaderBytes int
	disableCompression     bool
	logger                 *slog.Logger
}

type profile struct {
	settings settings
}

// Option configures an HTTP/3 transport profile.
type Option func(*settings)

// WithTLSConfig sets the TLS configuration for the HTTP/3 transport.
func WithTLSConfig(tlsConfig *tls.Config) Option {
	return func(s *settings) {
		s.tlsConfig = tlsConfig
	}
}

// WithQUICConfig sets the QUIC configuration for the HTTP/3 transport.
func WithQUICConfig(quicConfig *quic.Config) Option {
	return func(s *settings) {
		s.quicConfig = quicConfig
	}
}

// WithDatagrams enables HTTP/3 datagram support.
func WithDatagrams() Option {
	return func(s *settings) {
		s.enableDatagrams = true
	}
}

// WithAdditionalSettings sets additional HTTP/3 settings.
func WithAdditionalSettings(values map[uint64]uint64) Option {
	return func(s *settings) {
		s.additionalSettings = maps.Clone(values)
	}
}

// WithMaxResponseHeaderBytes sets the response header byte limit.
func WithMaxResponseHeaderBytes(n int) Option {
	return func(s *settings) {
		s.maxResponseHeaderBytes = n
	}
}

// WithoutCompression disables automatic gzip request and response handling in the HTTP/3 transport.
func WithoutCompression() Option {
	return func(s *settings) {
		s.disableCompression = true
	}
}

// WithLogger sets the HTTP/3 transport logger.
func WithLogger(logger *slog.Logger) Option {
	return func(s *settings) {
		s.logger = logger
	}
}

// Profile returns an HTTP/3 client profile.
func Profile(opts ...Option) requests.Profile {
	return profile{settings: newSettings(opts...)}
}

func (p profile) Name() string {
	return "HTTP/3"
}

func (p profile) Apply(c *requests.Client) error {
	if c == nil {
		return fmt.Errorf("%w: client", requests.ErrInvalidConfigValue)
	}
	settings := p.settings
	if settings.tlsConfig == nil {
		settings.tlsConfig = c.TLSConfig
	}
	c.SetDefaultTransport(settings.transport())
	return nil
}

// Transport returns a configured HTTP/3 transport.
func Transport(opts ...Option) *qhttp3.Transport {
	return newSettings(opts...).transport()
}

func newSettings(opts ...Option) settings {
	var s settings
	for _, opt := range opts {
		opt(&s)
	}
	return s
}

func (s settings) transport() *qhttp3.Transport {
	return &qhttp3.Transport{
		TLSClientConfig:        cloneTLSConfig(s.tlsConfig),
		QUICConfig:             cloneQUICConfig(s.quicConfig, s.enableDatagrams),
		EnableDatagrams:        s.enableDatagrams,
		AdditionalSettings:     maps.Clone(s.additionalSettings),
		MaxResponseHeaderBytes: s.maxResponseHeaderBytes,
		DisableCompression:     s.disableCompression,
		Logger:                 s.logger,
	}
}

func cloneTLSConfig(config *tls.Config) *tls.Config {
	if config == nil {
		return nil
	}
	return config.Clone()
}

func cloneQUICConfig(config *quic.Config, enableDatagrams bool) *quic.Config {
	if config == nil {
		return nil
	}
	clone := config.Clone()
	if enableDatagrams {
		clone.EnableDatagrams = true
	}
	return clone
}
