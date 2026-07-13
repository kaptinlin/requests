package requests

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

type transportErrorObservation struct {
	isCause              bool
	isCanceledCause      bool
	isDeadlineCause      bool
	asURLError           bool
	asDNSError           bool
	asOpError            bool
	asCertificateError   bool
	isCanceled           bool
	isTimeout            bool
	isConnectionError    bool
	defaultRetryEligible bool
}

func observeTransportError(err, cause error) transportErrorObservation {
	var urlErr *url.Error
	var dnsErr *net.DNSError
	var opErr *net.OpError
	var certificateErr *tls.CertificateVerificationError

	return transportErrorObservation{
		isCause:              cause != nil && errors.Is(err, cause),
		isCanceledCause:      errors.Is(err, context.Canceled),
		isDeadlineCause:      errors.Is(err, context.DeadlineExceeded),
		asURLError:           errors.As(err, &urlErr),
		asDNSError:           errors.As(err, &dnsErr),
		asOpError:            errors.As(err, &opErr),
		asCertificateError:   errors.As(err, &certificateErr),
		isCanceled:           IsCanceled(err),
		isTimeout:            IsTimeout(err),
		isConnectionError:    IsConnectionError(err),
		defaultRetryEligible: DefaultRetryIf(nil, nil, err),
	}
}

func TestPublicSendTransportErrorClassification(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T) (error, error)
		want transportErrorObservation
	}{
		{
			name: "DNS resolution",
			run: func(t *testing.T) (error, error) {
				cause := errors.New("local resolver unavailable")
				resolver := &net.Resolver{
					PreferGo: true,
					Dial: func(context.Context, string, string) (net.Conn, error) {
						return nil, cause
					},
				}
				client := newTestClient(t, WithoutProxy(), WithResolver(resolver))
				_, err := client.Get("http://resolver.test/").Send(t.Context())
				return err, cause
			},
			want: transportErrorObservation{
				asURLError:           true,
				asDNSError:           true,
				asOpError:            true,
				isConnectionError:    true,
				defaultRetryEligible: true,
			},
		},
		{
			name: "TCP dial",
			run: func(t *testing.T) (error, error) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				address := listener.Addr().String()
				require.NoError(t, listener.Close())

				client := newTestClient(t, WithoutProxy())
				_, err = client.Get("http://" + address + "/").Send(t.Context())
				return err, nil
			},
			want: transportErrorObservation{
				asURLError:           true,
				asOpError:            true,
				isConnectionError:    true,
				defaultRetryEligible: true,
			},
		},
		{
			name: "TLS verification",
			run: func(t *testing.T) (error, error) {
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				server.Config.ErrorLog = log.New(io.Discard, "", 0)
				server.StartTLS()
				defer server.Close()

				client := newTestClient(t, WithoutProxy())
				_, err := client.Get(server.URL).Send(t.Context())
				return err, nil
			},
			want: transportErrorObservation{
				asURLError:         true,
				asCertificateError: true,
			},
		},
		{
			name: "TLS handshake timeout",
			run: func(t *testing.T) (error, error) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				defer listener.Close() //nolint:errcheck // test cleanup

				done := make(chan struct{})
				defer close(done)
				go func() {
					conn, acceptErr := listener.Accept()
					if acceptErr != nil {
						return
					}
					defer conn.Close() //nolint:errcheck // test cleanup
					<-done
				}()

				client := newTestClient(t,
					WithoutProxy(),
					WithTLSHandshakeTimeout(50*time.Millisecond),
				)
				_, err = client.Get("https://" + listener.Addr().String() + "/").Send(t.Context())
				return err, nil
			},
			want: transportErrorObservation{
				asURLError:           true,
				isTimeout:            true,
				defaultRetryEligible: true,
			},
		},
		{
			name: "response header timeout",
			run: func(t *testing.T) (error, error) {
				server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					<-r.Context().Done()
				}))
				defer server.Close()

				client := newTestClient(t, WithResponseHeaderTimeout(50*time.Millisecond))
				_, err := client.Get(server.URL).Send(t.Context())
				return err, nil
			},
			want: transportErrorObservation{
				isDeadlineCause:      true,
				asURLError:           true,
				isTimeout:            true,
				defaultRetryEligible: true,
			},
		},
		{
			name: "caller cancellation",
			run: func(t *testing.T) (error, error) {
				var reached atomic.Bool
				server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					reached.Store(true)
				}))
				defer server.Close()

				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				client := newTestClient(t)
				_, err := client.Get(server.URL).Send(ctx)
				assert.False(t, reached.Load())
				return err, context.Canceled
			},
			want: transportErrorObservation{
				isCause:         true,
				isCanceledCause: true,
				asURLError:      true,
				isCanceled:      true,
			},
		},
		{
			name: "request deadline",
			run: func(t *testing.T) (error, error) {
				server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					<-r.Context().Done()
				}))
				defer server.Close()

				ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
				defer cancel()
				client := newTestClient(t)
				_, err := client.Get(server.URL).Send(ctx)
				return err, context.DeadlineExceeded
			},
			want: transportErrorObservation{
				isCause:              true,
				isDeadlineCause:      true,
				asURLError:           true,
				isTimeout:            true,
				defaultRetryEligible: true,
			},
		},
		{
			name: "joined retry errors",
			run: func(t *testing.T) (error, error) {
				cause := errors.New("local dial failure")
				var attempts atomic.Int32
				client := newTestClient(t,
					WithoutProxy(),
					WithDialContext(func(_ context.Context, network, _ string) (net.Conn, error) {
						attempts.Add(1)
						return nil, &net.OpError{Op: "dial", Net: network, Err: cause}
					}),
					WithRetry(RetryPolicy{Max: 1, Backoff: DefaultBackoffStrategy(0)}),
				)
				_, err := client.Get("http://dial.test/").Send(t.Context())
				assert.Equal(t, int32(2), attempts.Load())
				return err, cause
			},
			want: transportErrorObservation{
				isCause:              true,
				asURLError:           true,
				asOpError:            true,
				isConnectionError:    true,
				defaultRetryEligible: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err, cause := test.run(t)
			require.Error(t, err)
			got := observeTransportError(err, cause)
			assert.Equalf(t, test.want, got, "error=%T %v", err, err)
		})
	}
}
