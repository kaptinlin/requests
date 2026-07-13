package http3

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/quic-go/quic-go"
	qhttp3 "github.com/quic-go/quic-go/http3"
	"github.com/test-go/testify/require"

	"github.com/kaptinlin/requests"
)

func TestTransportOptions(t *testing.T) {
	tlsConfig := &tls.Config{ServerName: "example.com", MinVersion: tls.VersionTLS13}
	quicConfig := &quic.Config{}
	settings := map[uint64]uint64{0x21: 1}
	logger := slog.Default()

	transport := Transport(
		WithTLSConfig(tlsConfig),
		WithQUICConfig(quicConfig),
		WithDatagrams(),
		WithAdditionalSettings(settings),
		WithMaxResponseHeaderBytes(1024),
		WithoutCompression(),
		WithLogger(logger),
	)

	require.False(t, tlsConfig == transport.TLSClientConfig)
	require.Equal(t, "example.com", transport.TLSClientConfig.ServerName)
	require.False(t, quicConfig == transport.QUICConfig)
	require.True(t, transport.QUICConfig.EnableDatagrams)
	require.True(t, transport.EnableDatagrams)
	if diff := cmp.Diff(map[uint64]uint64{0x21: 1}, transport.AdditionalSettings); diff != "" {
		t.Errorf("additional settings mismatch (-want +got):\n%s", diff)
	}
	require.Equal(t, 1024, transport.MaxResponseHeaderBytes)
	require.True(t, transport.DisableCompression)
	require.True(t, logger == transport.Logger)

	settings[0x21] = 2
	require.Equal(t, uint64(1), transport.AdditionalSettings[0x21])
}

func TestProfileAppliesHTTP3Transport(t *testing.T) {
	profile := Profile()
	client, err := requests.New(requests.WithProfile(profile))
	require.NoError(t, err)

	require.Equal(t, "HTTP/3", profile.Name())
	_, ok := client.UnsafeHTTPClient().Transport.(*qhttp3.Transport)
	require.True(t, ok)
}

func TestProfileUsesClientTLSConfig(t *testing.T) {
	tlsConfig := &tls.Config{ServerName: "example.com", MinVersion: tls.VersionTLS13}
	client, err := requests.New(
		requests.WithTLSConfig(tlsConfig),
		requests.WithProfile(Profile()),
	)
	require.NoError(t, err)

	transport, ok := client.UnsafeHTTPClient().Transport.(*qhttp3.Transport)
	require.True(t, ok)
	require.False(t, tlsConfig == transport.TLSClientConfig)
	require.Equal(t, "example.com", transport.TLSClientConfig.ServerName)
}

func TestProfileCapturesTLSConfig(t *testing.T) {
	tlsConfig := &tls.Config{ServerName: "initial.example.com", MinVersion: tls.VersionTLS13}
	profile := Profile(WithTLSConfig(tlsConfig))
	tlsConfig.ServerName = "mutated.example.com"

	client, err := requests.New(requests.WithProfile(profile))
	require.NoError(t, err)
	transport, ok := client.UnsafeHTTPClient().Transport.(*qhttp3.Transport)
	require.True(t, ok)
	require.False(t, tlsConfig == transport.TLSClientConfig)
	require.Equal(t, "initial.example.com", transport.TLSClientConfig.ServerName)
}

func TestProfileRejectsTLSConfigAppliedAfterHTTP3(t *testing.T) {
	client, err := requests.New(
		requests.WithProfile(Profile()),
		requests.WithTLSConfig(&tls.Config{ServerName: "example.com"}),
	)

	require.Nil(t, client)
	require.True(t, errors.Is(err, requests.ErrInvalidTransportType))
}

func TestProfileRejectsSessionAppliedAfterHTTP3(t *testing.T) {
	client, err := requests.New(
		requests.WithProfile(Profile()),
		requests.WithSession(),
	)

	require.Nil(t, client)
	require.True(t, errors.Is(err, requests.ErrInvalidTransportType))
}

func TestProfileRejectsTLSMutationAppliedAfterHTTP3(t *testing.T) {
	client, err := requests.New(
		requests.WithProfile(Profile()),
		requests.WithInsecureSkipVerify(),
	)

	require.Nil(t, client)
	require.True(t, errors.Is(err, requests.ErrInvalidTransportType))
}

func TestProfileRejectsUnsupportedOptionsAppliedAfterHTTP3(t *testing.T) {
	tests := []struct {
		name   string
		option requests.Option
	}{
		{name: "certificates", option: requests.WithCertificates(tls.Certificate{})},
		{name: "TLS server name", option: requests.WithTLSServerName("example.com")},
		{name: "root certificate", option: requests.WithRootCertificate("../.github/testdata/cert.pem")},
		{name: "dial timeout", option: requests.WithDialTimeout(time.Second)},
		{name: "proxy", option: requests.WithProxy("http://proxy.example.com:8080")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := requests.New(requests.WithProfile(Profile()), test.option)

			require.Nil(t, client)
			require.True(t, errors.Is(err, requests.ErrInvalidTransportType))
		})
	}
}

func TestProfileUsesSessionConfiguredBeforeHTTP3(t *testing.T) {
	client, err := requests.New(requests.WithSession(), requests.WithProfile(Profile()))
	require.NoError(t, err)

	transport, ok := client.UnsafeHTTPClient().Transport.(*qhttp3.Transport)
	require.True(t, ok)
	require.NotNil(t, client.UnsafeHTTPClient().Jar)
	require.NotNil(t, transport.TLSClientConfig)
	require.NotNil(t, transport.TLSClientConfig.ClientSessionCache)
}

func TestProfileSendsHTTP3Request(t *testing.T) {
	source := httptest.NewTLSServer(http.NotFoundHandler())
	cert := source.TLS.Certificates[0]
	source.Close()

	packetConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &qhttp3.Server{
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{qhttp3.NextProtoH3},
		},
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "HTTP/3.0", r.Proto)
			_, _ = w.Write([]byte("h3"))
		}),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(packetConn)
	}()
	t.Cleanup(func() {
		require.NoError(t, server.Close())
		require.NoError(t, packetConn.Close())
		require.True(t, errors.Is(<-errCh, http.ErrServerClosed))
	})

	client, err := requests.New(requests.WithProfile(Profile(WithTLSConfig(&tls.Config{
		InsecureSkipVerify: true,
	}))))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	resp, err := client.Get("https://" + packetConn.LocalAddr().String()).Send(ctx)
	require.NoError(t, err)
	defer resp.Close() //nolint:errcheck // test cleanup closes response body
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.Equal(t, "HTTP/3.0", resp.Protocol())
	require.Equal(t, "h3", resp.String())
}
