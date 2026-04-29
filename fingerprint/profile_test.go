package fingerprint

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kaptinlin/requests"
	utls "github.com/refraction-networking/utls"
	"github.com/test-go/testify/require"
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

	client := requests.New(
		requests.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}), //nolint:gosec
		requests.WithProfile(Chrome()),
	)

	resp, err := client.Get(server.URL).Send(t.Context())
	require.NoError(t, err)
	defer resp.Close() //nolint:errcheck
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
	client := requests.New(
		requests.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}), //nolint:gosec
		requests.WithProfile(Chrome()),
		requests.WithDialContext(func(ctx context.Context, network, addr string) (net.Conn, error) {
			called.Store(true)
			return dialer.DialContext(ctx, network, addr)
		}),
	)

	resp, err := client.Get(server.URL).Send(t.Context())
	require.NoError(t, err)
	defer resp.Close() //nolint:errcheck
	require.True(t, called.Load())
}

func TestProfileRejectsCustomTransport(t *testing.T) {
	client := requests.New(requests.WithTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errUnusedRoundTrip
	})))

	err := client.ApplyProfile(Chrome())
	require.True(t, errors.Is(err, requests.ErrInvalidTransportType))
}

func TestConfigureTransportRejectsNilTransport(t *testing.T) {
	err := ConfigureTransport(nil, utls.HelloChrome_Auto)

	require.True(t, errors.Is(err, requests.ErrInvalidConfigValue))
}

func TestCustomProfileNameDefaultsToHelloID(t *testing.T) {
	profile := Custom("", utls.HelloChrome_Auto)

	require.True(t, strings.HasPrefix(profile.Name(), "Chrome-"))
}
