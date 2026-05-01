package fingerprint

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"slices"
	"sync"

	"github.com/kaptinlin/requests"
	utls "github.com/refraction-networking/utls"
)

// ConfigureTransport configures transport to use helloID for TLS handshakes.
func ConfigureTransport(transport *http.Transport, helloID utls.ClientHelloID) error {
	if transport == nil {
		return fmt.Errorf("%w: transport", requests.ErrInvalidConfigValue)
	}
	if isEmptyHelloID(helloID) {
		return fmt.Errorf("%w: hello id", requests.ErrInvalidConfigValue)
	}

	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	ensureNextProtos(transport.TLSClientConfig)
	transport.ForceAttemptHTTP2 = true

	sessionCache := sync.OnceValue(func() utls.ClientSessionCache {
		return utls.NewLRUClientSessionCache(0)
	})
	transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialContext := transport.DialContext
		if dialContext == nil {
			dialer := &net.Dialer{}
			dialContext = dialer.DialContext
		}
		rawConn, err := dialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		config := utlsConfig(transport.TLSClientConfig, addr)
		if transport.TLSClientConfig != nil && transport.TLSClientConfig.ClientSessionCache != nil {
			config.ClientSessionCache = sessionCache()
		}
		conn := utls.UClient(rawConn, config, helloID)
		if err := conn.HandshakeContext(ctx); err != nil {
			_ = rawConn.Close()
			return nil, err
		}
		return conn, nil
	}
	return nil
}

func utlsConfig(config *tls.Config, addr string) *utls.Config {
	if config == nil {
		config = &tls.Config{}
	}
	clone := config.Clone()
	if clone.ServerName == "" {
		if host, _, err := net.SplitHostPort(addr); err == nil && net.ParseIP(host) == nil {
			clone.ServerName = host
		}
	}
	if len(clone.NextProtos) == 0 {
		clone.NextProtos = []string{"h2", "http/1.1"}
	}

	return &utls.Config{
		Rand:                        clone.Rand,
		Time:                        clone.Time,
		Certificates:                utlsCertificates(clone.Certificates),
		VerifyPeerCertificate:       clone.VerifyPeerCertificate,
		RootCAs:                     clone.RootCAs,
		NextProtos:                  slices.Clone(clone.NextProtos),
		ServerName:                  clone.ServerName,
		InsecureSkipVerify:          clone.InsecureSkipVerify,
		CipherSuites:                slices.Clone(clone.CipherSuites),
		SessionTicketsDisabled:      clone.SessionTicketsDisabled,
		MinVersion:                  clone.MinVersion,
		MaxVersion:                  clone.MaxVersion,
		CurvePreferences:            utlsCurvePreferences(clone.CurvePreferences),
		DynamicRecordSizingDisabled: clone.DynamicRecordSizingDisabled,
		Renegotiation:               utls.RenegotiationSupport(clone.Renegotiation),
		KeyLogWriter:                clone.KeyLogWriter,
	}
}

func utlsCertificates(certs []tls.Certificate) []utls.Certificate {
	if len(certs) == 0 {
		return nil
	}
	converted := make([]utls.Certificate, len(certs))
	for i, cert := range certs {
		converted[i] = utls.Certificate{
			Certificate:                  cloneByteSlices(cert.Certificate),
			PrivateKey:                   cert.PrivateKey,
			SupportedSignatureAlgorithms: utlsSignatureSchemes(cert.SupportedSignatureAlgorithms),
			OCSPStaple:                   slices.Clone(cert.OCSPStaple),
			SignedCertificateTimestamps:  cloneByteSlices(cert.SignedCertificateTimestamps),
			Leaf:                         cert.Leaf,
		}
	}
	return converted
}

func utlsSignatureSchemes(schemes []tls.SignatureScheme) []utls.SignatureScheme {
	return convertSlice[utls.SignatureScheme](schemes)
}

func utlsCurvePreferences(curves []tls.CurveID) []utls.CurveID {
	return convertSlice[utls.CurveID](curves)
}

func convertSlice[To, From ~uint16](values []From) []To {
	if len(values) == 0 {
		return nil
	}
	converted := make([]To, len(values))
	for i, value := range values {
		converted[i] = To(value)
	}
	return converted
}

func cloneByteSlices(values [][]byte) [][]byte {
	if values == nil {
		return nil
	}
	cloned := make([][]byte, len(values))
	for i, value := range values {
		cloned[i] = slices.Clone(value)
	}
	return cloned
}
