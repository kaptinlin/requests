package browser

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/kaptinlin/requests"
	"github.com/test-go/testify/require"
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

	client := requests.New(requests.WithProfile(Chrome()))
	client.AddMiddleware(func(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
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
	})

	transport, ok := client.GetHTTPClient().Transport.(*http.Transport)
	require.True(t, ok)
	require.True(t, transport.ForceAttemptHTTP2)
	require.NotNil(t, transport.TLSClientConfig)
	require.True(t, slices.Contains(transport.TLSClientConfig.NextProtos, "h2"))

	resp, err := client.Get(server.URL).Send(t.Context())
	require.NoError(t, err)
	defer resp.Close() //nolint:errcheck
}

func TestFirefoxProfile(t *testing.T) {
	client := requests.New(requests.WithProfile(Firefox()))

	require.NotNil(t, client.Headers)
	require.Contains(t, client.Headers.Get("User-Agent"), "Firefox/148.0")
	require.Equal(t, "en-US,en;q=0.5", client.Headers.Get("Accept-Language"))
	require.Empty(t, client.Headers.Get(":authority"))
	require.True(t, strings.Contains(client.Headers.Get("Accept-Encoding"), "gzip"))
}

func TestProfileRequestHeadersOverrideDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "CustomAgent/1.0", r.Header.Get("User-Agent"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := requests.New(requests.WithProfile(Chrome()))
	client.AddMiddleware(func(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			ordered, ok := requests.OrderedHeaders(req)
			require.True(t, ok)
			require.NotContains(t, ordered.Keys(), "User-Agent")
			return next(req)
		}
	})

	resp, err := client.Get(server.URL).UserAgent("CustomAgent/1.0").Send(t.Context())
	require.NoError(t, err)
	defer resp.Close() //nolint:errcheck
}
