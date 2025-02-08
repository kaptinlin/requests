package requests

import (
	"crypto/tls"
)

// TLSFingerprint is used to configure TLS client fingerprint
type TLSFingerprint struct {
	// Basic TLS configuration
	MinVersion       uint16
	MaxVersion       uint16
	CipherSuites     []uint16
	CurvePreferences []tls.CurveID

	// JA3 related
	JA3 string // JA3 fingerprint string

	// ALPN protocols
	ALPN []string

	// Session related
	SessionTicketsDisabled bool
	SessionCache           tls.ClientSessionCache
}
