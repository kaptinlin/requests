package http3

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kaptinlin/requests"
	"github.com/quic-go/quic-go"
	qhttp3 "github.com/quic-go/quic-go/http3"
	"github.com/test-go/testify/require"
)

func TestTransportOptions(t *testing.T) {
	tlsConfig := &tls.Config{ServerName: "example.com", MinVersion: tls.VersionTLS13}
	quicConfig := &quic.Config{}
	settings := map[uint64]uint64{0x21: 1}

	transport := Transport(
		WithTLSConfig(tlsConfig),
		WithQUICConfig(quicConfig),
		WithDatagrams(),
		WithAdditionalSettings(settings),
		WithMaxResponseHeaderBytes(1024),
		WithoutCompression(),
	)

	require.False(t, tlsConfig == transport.TLSClientConfig)
	require.Equal(t, "example.com", transport.TLSClientConfig.ServerName)
	require.False(t, quicConfig == transport.QUICConfig)
	require.True(t, transport.QUICConfig.EnableDatagrams)
	require.True(t, transport.EnableDatagrams)
	require.Equal(t, map[uint64]uint64{0x21: 1}, transport.AdditionalSettings)
	require.Equal(t, 1024, transport.MaxResponseHeaderBytes)
	require.True(t, transport.DisableCompression)

	settings[0x21] = 2
	require.Equal(t, uint64(1), transport.AdditionalSettings[0x21])
}

func TestProfileAppliesHTTP3Transport(t *testing.T) {
	client := requests.New(requests.WithProfile(Profile()))

	_, ok := client.GetHTTPClient().Transport.(*qhttp3.Transport)
	require.True(t, ok)
}

func TestProfileUsesClientTLSConfig(t *testing.T) {
	tlsConfig := &tls.Config{ServerName: "example.com", MinVersion: tls.VersionTLS13}
	client := requests.New(
		requests.WithTLSConfig(tlsConfig),
		requests.WithProfile(Profile()),
	)

	transport, ok := client.GetHTTPClient().Transport.(*qhttp3.Transport)
	require.True(t, ok)
	require.False(t, tlsConfig == transport.TLSClientConfig)
	require.Equal(t, "example.com", transport.TLSClientConfig.ServerName)
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

	client := requests.New(requests.WithProfile(Profile(WithTLSConfig(&tls.Config{
		InsecureSkipVerify: true, //nolint:gosec
	}))))
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	resp, err := client.Get("https://" + packetConn.LocalAddr().String()).Send(ctx)
	require.NoError(t, err)
	defer resp.Close() //nolint:errcheck
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.Equal(t, "HTTP/3.0", resp.Protocol())
	require.Equal(t, "h3", resp.String())
}
