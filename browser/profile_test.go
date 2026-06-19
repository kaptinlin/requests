package browser

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/test-go/testify/require"

	"github.com/kaptinlin/requests"
)

func TestChromeProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.Header.Get("User-Agent"), "Chrome/145.0.0.0")
		require.Equal(t, "gzip, deflate", r.Header.Get("Accept-Encoding"))
		require.NotContains(t, r.Header.Get("Accept-Encoding"), "br")
		require.NotContains(t, r.Header.Get("Accept-Encoding"), "zstd")
		require.Empty(t, r.Header.Get(":authority"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client, err := requests.New(
		requests.WithProfile(Chrome()),
		requests.WithMiddleware(func(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
			return func(req *http.Request) (*http.Response, error) {
				ordered, ok := requests.OrderedHeaders(req)
				require.True(t, ok)
				require.Equal(t, []string{
					":authority",
					":method",
					":path",
					":scheme",
					"Accept-Encoding",
					"Accept-Language",
					"Sec-CH-UA",
					"Sec-CH-UA-Mobile",
					"Sec-CH-UA-Platform",
					"User-Agent",
				}, ordered.Keys())
				return next(req)
			}
		}),
	)
	require.NoError(t, err)

	transport, ok := client.GetHTTPClient().Transport.(*http.Transport)
	require.True(t, ok)
	require.True(t, transport.ForceAttemptHTTP2)
	require.NotNil(t, transport.TLSClientConfig)
	require.True(t, slices.Contains(transport.TLSClientConfig.NextProtos, "h2"))

	resp, err := client.Get(server.URL).Send(t.Context())
	require.NoError(t, err)
	require.NoError(t, resp.Close())
}

func TestChromeProfileUsesExampleHostOverTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "www.example.com", r.Host)
		require.NotNil(t, r.TLS)
		require.Contains(t, r.Header.Get("User-Agent"), "Chrome/145.0.0.0")
		require.Empty(t, r.Header.Get(":authority"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client, err := requests.New(
		requests.WithHTTPClient(server.Client()),
		requests.WithProfile(Chrome()),
	)
	require.NoError(t, err)

	resp, err := client.Get("https://www.example.com").Send(t.Context())
	require.NoError(t, err)
	require.NoError(t, resp.Close())
}

func TestFirefoxProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.Header.Get("User-Agent"), "Firefox/148.0")
		require.Equal(t, "en-US,en;q=0.5", r.Header.Get("Accept-Language"))
		require.Empty(t, r.Header.Get(":authority"))
		require.Contains(t, r.Header.Get("Accept-Encoding"), "gzip")
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	profile := Firefox()
	client, err := requests.New(requests.WithProfile(profile))
	require.NoError(t, err)

	require.Equal(t, "Firefox", profile.Name())
	transport, ok := client.GetHTTPClient().Transport.(*http.Transport)
	require.True(t, ok)
	require.True(t, transport.ForceAttemptHTTP2)

	resp, err := client.Get(server.URL).Send(t.Context())
	require.NoError(t, err)
	require.NoError(t, resp.Close())
}

func TestProfileRequestHeadersOverrideDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "CustomAgent/1.0", r.Header.Get("User-Agent"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client, err := requests.New(
		requests.WithProfile(Chrome()),
		requests.WithMiddleware(func(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
			return func(req *http.Request) (*http.Response, error) {
				ordered, ok := requests.OrderedHeaders(req)
				require.True(t, ok)
				require.NotContains(t, ordered.Keys(), "User-Agent")
				return next(req)
			}
		}),
	)
	require.NoError(t, err)

	resp, err := client.Get(server.URL).UserAgent("CustomAgent/1.0").Send(t.Context())
	require.NoError(t, err)
	require.NoError(t, resp.Close())
}
