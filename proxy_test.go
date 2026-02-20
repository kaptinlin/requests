package requests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// createTestServerForProxy creates a simple HTTP server for testing purposes.
func createTestServerForProxy() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

// TestSetProxyValidProxy tests setting a valid proxy and making a request through it.
func TestSetProxyValidProxy(t *testing.T) {
	server := createTestServerForProxy()
	defer server.Close()

	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Indicate the request passed through the proxy
		w.Header().Set("X-Test-Proxy", "true")
		w.WriteHeader(http.StatusOK)
	}))
	defer proxyServer.Close()

	client := URL(server.URL)

	err := client.SetProxy(proxyServer.URL)
	assert.Nil(t, err, "Setting a valid proxy should not result in an error.")

	resp, err := client.Get("/").Send(context.Background())
	assert.Nil(t, err, "Request through a valid proxy should succeed.")
	assert.NotNil(t, resp, "Response should not be nil.")
	assert.Equal(t, "true", resp.Header().Get("X-Test-Proxy"), "Request should have passed through the proxy.")
}

// TestSetProxyInvalidProxy tests handling of invalid proxy URLs.
func TestSetProxyInvalidProxy(t *testing.T) {
	server := createTestServerForProxy()
	defer server.Close()
	client := URL(server.URL)

	invalidProxyURL := "://invalid_url"
	err := client.SetProxy(invalidProxyURL)
	assert.NotNil(t, err, "Setting an invalid proxy URL should result in an error.")
}

// TestSetProxyRemoveProxy tests removing proxy settings.
func TestSetProxyRemoveProxy(t *testing.T) {
	server := createTestServerForProxy()
	defer server.Close()

	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Proxy server response
		w.WriteHeader(http.StatusOK)
	}))
	defer proxyServer.Close()

	client := URL(server.URL)

	// Set then remove the proxy
	err := client.SetProxy(proxyServer.URL)
	assert.Nil(t, err, "Setting a proxy should not result in an error.")

	client.RemoveProxy()

	// Make a request and check it doesn't go through the proxy
	resp, err := client.Get("/").Send(context.Background())
	assert.Nil(t, err, "Request after removing proxy should succeed.")
	assert.NotNil(t, resp, "Response should not be nil.")
	assert.NotEqual(t, "true", resp.Header().Get("X-Test-Proxy"), "Request should not have passed through the proxy.")
}

func TestNoProxyParsing(t *testing.T) {
	t.Run("Wildcard", func(t *testing.T) {
		np := parseNoProxy("*")
		assert.True(t, np.wildcard)
		assert.True(t, np.matches("anything.com"))
		assert.True(t, np.matches("192.168.1.1"))
	})

	t.Run("DomainExactMatch", func(t *testing.T) {
		np := parseNoProxy("example.com")
		assert.True(t, np.matches("example.com"))
		assert.True(t, np.matches("Example.COM"))
		assert.False(t, np.matches("other.com"))
	})

	t.Run("DomainSubdomainMatch", func(t *testing.T) {
		np := parseNoProxy("example.com")
		assert.True(t, np.matches("sub.example.com"))
		assert.True(t, np.matches("deep.sub.example.com"))
		assert.False(t, np.matches("notexample.com"))
	})

	t.Run("LeadingDotDomain", func(t *testing.T) {
		np := parseNoProxy(".example.com")
		assert.True(t, np.matches("sub.example.com"))
		assert.True(t, np.matches("deep.sub.example.com"))
		assert.False(t, np.matches("example.com"))
	})

	t.Run("IPMatch", func(t *testing.T) {
		np := parseNoProxy("192.168.1.1, 10.0.0.1")
		assert.True(t, np.matches("192.168.1.1"))
		assert.True(t, np.matches("10.0.0.1"))
		assert.False(t, np.matches("192.168.1.2"))
	})

	t.Run("CIDRMatch", func(t *testing.T) {
		np := parseNoProxy("192.168.0.0/16")
		assert.True(t, np.matches("192.168.1.1"))
		assert.True(t, np.matches("192.168.255.255"))
		assert.False(t, np.matches("10.0.0.1"))
	})

	t.Run("MixedRules", func(t *testing.T) {
		np := parseNoProxy("localhost, .local, 10.0.0.0/8, 192.168.1.100")
		assert.True(t, np.matches("localhost"))
		assert.True(t, np.matches("myhost.local"))
		assert.True(t, np.matches("10.1.2.3"))
		assert.True(t, np.matches("192.168.1.100"))
		assert.False(t, np.matches("google.com"))
	})

	t.Run("HostWithPort", func(t *testing.T) {
		np := parseNoProxy("example.com")
		assert.True(t, np.matches("example.com:8080"))
	})

	t.Run("EmptyString", func(t *testing.T) {
		np := parseNoProxy("")
		assert.False(t, np.matches("anything.com"))
	})

	t.Run("NilNoProxy", func(t *testing.T) {
		var np *NoProxy
		assert.False(t, np.matches("anything.com"))
	})
}

func TestSetProxyWithBypass(t *testing.T) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-Proxy", "true")
		w.WriteHeader(http.StatusOK)
	}))
	defer proxyServer.Close()

	server := createTestServerForProxy()
	defer server.Close()

	client := URL(server.URL)
	err := client.SetProxyWithBypass(proxyServer.URL, "127.0.0.1, localhost")
	assert.NoError(t, err)

	resp, err := client.Get("/").Send(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Header().Get("X-Test-Proxy"))
}

func TestSetProxyFromEnv(t *testing.T) {
	client := Create(nil)
	err := client.SetProxyFromEnv()
	assert.NoError(t, err)
}

func TestSetProxyWithBypassInvalidProxy(t *testing.T) {
	client := Create(nil)
	err := client.SetProxyWithBypass("://invalid", "localhost")
	assert.Error(t, err)
}

func TestRoundRobinProxies(t *testing.T) {
	p1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Proxy-ID", "1")
		w.WriteHeader(http.StatusOK)
	}))
	defer p1.Close()

	p2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Proxy-ID", "2")
		w.WriteHeader(http.StatusOK)
	}))
	defer p2.Close()

	server := createTestServerForProxy()
	defer server.Close()

	client := URL(server.URL)
	err := client.SetProxies(p1.URL, p2.URL)
	assert.NoError(t, err)

	// Send 4 requests and verify round-robin ordering
	ids := make([]string, 4)
	for i := range 4 {
		resp, err := client.Get("/").Send(context.Background())
		assert.NoError(t, err)
		ids[i] = resp.Header().Get("X-Proxy-ID")
	}

	assert.Equal(t, "1", ids[0])
	assert.Equal(t, "2", ids[1])
	assert.Equal(t, "1", ids[2])
	assert.Equal(t, "2", ids[3])
}

func TestRandomProxies(t *testing.T) {
	p1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Proxy-ID", "1")
		w.WriteHeader(http.StatusOK)
	}))
	defer p1.Close()

	p2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Proxy-ID", "2")
		w.WriteHeader(http.StatusOK)
	}))
	defer p2.Close()

	server := createTestServerForProxy()
	defer server.Close()

	selector, err := RandomProxies(p1.URL, p2.URL)
	assert.NoError(t, err)

	client := URL(server.URL)
	err = client.SetProxySelector(selector)
	assert.NoError(t, err)

	seen := map[string]bool{}
	for range 20 {
		resp, err := client.Get("/").Send(context.Background())
		assert.NoError(t, err)
		seen[resp.Header().Get("X-Proxy-ID")] = true
	}

	assert.True(t, seen["1"])
	assert.True(t, seen["2"])
}

func TestSetProxiesValidation(t *testing.T) {
	t.Run("NoProxies", func(t *testing.T) {
		client := Create(nil)
		err := client.SetProxies()
		assert.ErrorIs(t, err, ErrNoProxies)
	})

	t.Run("InvalidProxy", func(t *testing.T) {
		client := Create(nil)
		err := client.SetProxies("http://good:8080", "ftp://bad:21")
		assert.ErrorIs(t, err, ErrUnsupportedScheme)
	})

	t.Run("InvalidTransport", func(t *testing.T) {
		client := Create(nil)
		client.HTTPClient.Transport = testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, nil
		})
		err := client.SetProxies("http://proxy:8080")
		assert.ErrorIs(t, err, ErrInvalidTransportType)
	})
}

func TestRoundRobinProxiesFactory(t *testing.T) {
	t.Run("NoURLs", func(t *testing.T) {
		_, err := RoundRobinProxies()
		assert.ErrorIs(t, err, ErrNoProxies)
	})

	t.Run("InvalidURL", func(t *testing.T) {
		_, err := RoundRobinProxies("ftp://bad:21")
		assert.ErrorIs(t, err, ErrUnsupportedScheme)
	})

	t.Run("CyclesCorrectly", func(t *testing.T) {
		selector, err := RoundRobinProxies("http://a:1", "http://b:2", "http://c:3")
		assert.NoError(t, err)

		for range 2 {
			u, err := selector(nil)
			assert.NoError(t, err)
			assert.Equal(t, "a:1", u.Host)

			u, err = selector(nil)
			assert.NoError(t, err)
			assert.Equal(t, "b:2", u.Host)

			u, err = selector(nil)
			assert.NoError(t, err)
			assert.Equal(t, "c:3", u.Host)
		}
	})
}

func TestRandomProxiesFactory(t *testing.T) {
	t.Run("NoURLs", func(t *testing.T) {
		_, err := RandomProxies()
		assert.ErrorIs(t, err, ErrNoProxies)
	})

	t.Run("ReturnsValidProxy", func(t *testing.T) {
		selector, err := RandomProxies("http://a:1", "http://b:2")
		assert.NoError(t, err)

		valid := map[string]bool{"a:1": true, "b:2": true}
		for range 20 {
			u, err := selector(nil)
			assert.NoError(t, err)
			assert.True(t, valid[u.Host])
		}
	})
}

func TestRetryRotatesProxy(t *testing.T) {
	var proxyIDs []string

	// Proxy 1: returns 503
	p1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyIDs = append(proxyIDs, "1")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer p1.Close()

	// Proxy 2: returns 200
	p2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyIDs = append(proxyIDs, "2")
		w.WriteHeader(http.StatusOK)
	}))
	defer p2.Close()

	server := createTestServerForProxy()
	defer server.Close()

	client := URL(server.URL)
	err := client.SetProxies(p1.URL, p2.URL)
	assert.NoError(t, err)

	client.SetMaxRetries(1)
	client.SetRetryStrategy(DefaultBackoffStrategy(0))

	resp, err := client.Get("/").Send(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())

	// Retry should have rotated: attempt 1 → proxy 1 (503), attempt 2 → proxy 2 (200)
	assert.Equal(t, []string{"1", "2"}, proxyIDs)
}

func TestEnsureTransportInvalidType(t *testing.T) {
	client := Create(nil)
	client.HTTPClient.Transport = testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, nil
	})

	err := client.SetProxy("http://proxy.example.com")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidTransportType)
}
