package requests

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
)

// JA3Spec defines the JA3 fingerprint specification
type JA3Spec struct {
	Version             uint16        // TLS version
	CipherSuites        []uint16      // Cipher suites
	Extensions          []uint16      // Extensions
	EllipticCurves      []tls.CurveID // Elliptic curves
	EllipticCurvePoints []uint8       // Elliptic curve point formats
}

// NewTLSConfigFromJA3 converts a JA3 string to TLS configuration
func NewTLSConfigFromJA3(ja3string string) (*tls.Config, error) {
	spec, err := parseJA3String(ja3string)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion:       spec.Version,
		MaxVersion:       spec.Version,
		CipherSuites:     spec.CipherSuites,
		CurvePreferences: spec.EllipticCurves,
		NextProtos:       []string{"h2", "http/1.1"},
	}, nil
}

// parseJA3String parses a JA3 string
func parseJA3String(ja3 string) (*JA3Spec, error) {
	parts := strings.Split(ja3, ",")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid JA3 string format")
	}

	version, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid TLS version: %v", err)
	}

	// Parse cipher suites
	var cipherSuites []uint16
	for _, c := range strings.Split(parts[1], "-") {
		if c == "" {
			continue
		}
		cs, err := strconv.ParseUint(c, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid cipher suite: %v", err)
		}
		cipherSuites = append(cipherSuites, uint16(cs))
	}

	// Parse extensions
	var extensions []uint16
	for _, e := range strings.Split(parts[2], "-") {
		if e == "" {
			continue
		}
		ext, err := strconv.ParseUint(e, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid extension: %v", err)
		}
		extensions = append(extensions, uint16(ext))
	}

	// Parse elliptic curves
	var curves []tls.CurveID
	for _, c := range strings.Split(parts[3], "-") {
		if c == "" {
			continue
		}
		curve, err := strconv.ParseUint(c, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid curve: %v", err)
		}
		curves = append(curves, tls.CurveID(curve))
	}

	// Parse elliptic curve point formats
	var points []uint8
	for _, p := range strings.Split(parts[4], "-") {
		if p == "" {
			continue
		}
		point, err := strconv.ParseUint(p, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid point format: %v", err)
		}
		points = append(points, uint8(point))
	}

	return &JA3Spec{
		Version:             uint16(version),
		CipherSuites:        cipherSuites,
		Extensions:          extensions,
		EllipticCurves:      curves,
		EllipticCurvePoints: points,
	}, nil
}

// Predefined JA3 fingerprints
var (
	// Chrome120JA3 is the JA3 fingerprint for Chrome 120
	Chrome120JA3 = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0"

	// Firefox120JA3 is the JA3 fingerprint for Firefox 120
	Firefox120JA3 = "771,4865-4867-4866-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513-41,29-23-24-25,0"
)
