package fingerprint

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	utls "github.com/refraction-networking/utls"
	"github.com/test-go/testify/require"

	"github.com/kaptinlin/requests"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

var errUnusedRoundTrip = errors.New("unused round trip")

func TestChromeProfileSendsRequest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client, err := requests.New(
		requests.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
		requests.WithProfile(Chrome()),
	)
	require.NoError(t, err)

	resp, err := client.Get(server.URL).Send(t.Context())
	require.NoError(t, err)
	defer resp.Close() //nolint:errcheck // test cleanup closes response body
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.Equal(t, "ok", resp.String())
}

func TestProfileUsesDialContextSetAfterProfile(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	var called atomic.Bool
	dialer := &net.Dialer{}
	client, err := requests.New(
		requests.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}),
		requests.WithProfile(Chrome()),
		requests.WithDialContext(func(ctx context.Context, network, addr string) (net.Conn, error) {
			called.Store(true)
			return dialer.DialContext(ctx, network, addr)
		}),
	)
	require.NoError(t, err)

	resp, err := client.Get(server.URL).Send(t.Context())
	require.NoError(t, err)
	defer resp.Close() //nolint:errcheck // test cleanup closes response body
	require.True(t, called.Load())
}

func TestProfileUsesTLSConfigSetAfterProfile(t *testing.T) {
	tlsConfig := &tls.Config{ServerName: "example.com"}
	client, err := requests.New(
		requests.WithProfile(Chrome()),
		requests.WithTLSConfig(tlsConfig),
	)
	require.NoError(t, err)

	transport, ok := client.UnsafeHTTPClient().Transport.(*http.Transport)
	require.True(t, ok)
	require.False(t, transport.TLSClientConfig == tlsConfig)
	require.Equal(t, "example.com", transport.TLSClientConfig.ServerName)
	require.NotNil(t, transport.DialTLSContext)
}

func TestProfilePreservesStandardTransportOptions(t *testing.T) {
	tests := []struct {
		name    string
		options []requests.Option
	}{
		{
			name: "proxy before profile",
			options: []requests.Option{
				requests.WithProxy("http://proxy.example.com:8080"),
				requests.WithProfile(Chrome()),
			},
		},
		{
			name: "proxy after profile",
			options: []requests.Option{
				requests.WithProfile(Chrome()),
				requests.WithProxy("http://proxy.example.com:8080"),
			},
		},
		{
			name: "session before profile",
			options: []requests.Option{
				requests.WithSession(),
				requests.WithProfile(Chrome()),
			},
		},
		{
			name: "session after profile",
			options: []requests.Option{
				requests.WithProfile(Chrome()),
				requests.WithSession(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := requests.New(test.options...)
			require.NoError(t, err)

			transport, ok := client.UnsafeHTTPClient().Transport.(*http.Transport)
			require.True(t, ok)
			require.NotNil(t, transport.DialTLSContext)
			if strings.HasPrefix(test.name, "proxy") {
				require.NotNil(t, transport.Proxy)
			}
			if strings.HasPrefix(test.name, "session") {
				require.NotNil(t, client.UnsafeHTTPClient().Jar)
				require.NotNil(t, transport.TLSClientConfig.ClientSessionCache)
			}
		})
	}
}

func TestProfileRejectsCustomTransport(t *testing.T) {
	client, err := requests.New(
		requests.WithTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errUnusedRoundTrip
		})),
		requests.WithProfile(Chrome()),
	)

	require.Nil(t, client)
	require.True(t, errors.Is(err, requests.ErrInvalidTransportType))
}

func TestConfigureTransportRejectsNilTransport(t *testing.T) {
	err := ConfigureTransport(nil, utls.HelloChrome_Auto)

	require.True(t, errors.Is(err, requests.ErrInvalidConfigValue))
}

func TestFirefoxProfileName(t *testing.T) {
	profile := Firefox()

	require.Equal(t, "Firefox", profile.Name())
}

func TestCustomProfileNameDefaultsToHelloID(t *testing.T) {
	profile := Custom("", utls.HelloChrome_Auto)

	require.True(t, strings.HasPrefix(profile.Name(), "Chrome-"))
}

func TestCustomProfileRejectsEmptyHelloID(t *testing.T) {
	client, err := requests.New(requests.WithProfile(Custom("empty", utls.ClientHelloID{})))

	require.Nil(t, client)
	require.True(t, errors.Is(err, requests.ErrInvalidConfigValue))
}

func TestProfileWithTLSConfigConfiguresTransport(t *testing.T) {
	tlsConfig := &tls.Config{
		ServerName:   "example.com",
		NextProtos:   []string{"acme-tls/1"},
		Certificates: []tls.Certificate{{Certificate: [][]byte{{1, 2, 3}}}},
		RootCAs:      x509.NewCertPool(),
	}
	tlsConfig.Certificates[0].SupportedSignatureAlgorithms = []tls.SignatureScheme{tls.PKCS1WithSHA256}
	tlsConfig.Certificates[0].OCSPStaple = []byte{4, 5, 6}
	tlsConfig.Certificates[0].SignedCertificateTimestamps = [][]byte{{7, 8, 9}}
	tlsConfig.CurvePreferences = []tls.CurveID{tls.X25519}
	profile := Custom("custom", utls.HelloChrome_Auto, WithTLSConfig(tlsConfig))
	tlsConfig.ServerName = "mutated.example.com"
	client, err := requests.New(requests.WithProfile(profile))
	require.NoError(t, err)

	transport, ok := client.UnsafeHTTPClient().Transport.(*http.Transport)
	require.True(t, ok)
	require.False(t, tlsConfig == transport.TLSClientConfig)
	require.Equal(t, "example.com", transport.TLSClientConfig.ServerName)
	require.Contains(t, transport.TLSClientConfig.NextProtos, "h2")
	require.Contains(t, transport.TLSClientConfig.NextProtos, "http/1.1")
	require.Contains(t, transport.TLSClientConfig.NextProtos, "acme-tls/1")
	require.True(t, transport.ForceAttemptHTTP2)
	require.NotNil(t, transport.DialTLSContext)
}

func TestConfigureTransportUsesTLSConfigForHandshake(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	_ = serverConn.Close()

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			ServerName:            "example.com",
			InsecureSkipVerify:    true,
			ClientSessionCache:    tls.NewLRUClientSessionCache(1),
			Certificates:          []tls.Certificate{{Certificate: [][]byte{{1, 2, 3}}}},
			CurvePreferences:      []tls.CurveID{tls.X25519},
			VerifyPeerCertificate: func([][]byte, [][]*x509.Certificate) error { return nil },
			RootCAs:               x509.NewCertPool(),
		},
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return clientConn, nil
		},
	}
	transport.TLSClientConfig.Certificates[0].SupportedSignatureAlgorithms = []tls.SignatureScheme{tls.PKCS1WithSHA256}
	transport.TLSClientConfig.Certificates[0].OCSPStaple = []byte{4, 5, 6}
	transport.TLSClientConfig.Certificates[0].SignedCertificateTimestamps = [][]byte{{7, 8, 9}}
	require.NoError(t, ConfigureTransport(transport, utls.HelloChrome_Auto))

	_, err := transport.DialTLSContext(t.Context(), "tcp", "example.com:443")

	require.Error(t, err)
}
