package requests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedirectPolicies(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/no-redirect":
			_, _ = w.Write([]byte("no redirect"))
		case "/redirect-1":
			http.Redirect(w, r, "/redirect-2", http.StatusFound)
		case "/redirect-2":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			_, _ = w.Write([]byte("final destination"))
		}
	}))
	defer ts.Close()

	t.Run("ProhibitRedirectPolicy", func(t *testing.T) {
		client := Create(nil)
		client.SetRedirectPolicy(NewProhibitRedirectPolicy())

		_, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())

		assert.Error(t, err, "Expected to receive redirect error")
		assert.ErrorIs(t, err, ErrAutoRedirectDisabled, "Expected auto redirect disabled error")
	})

	t.Run("AllowRedirectPolicy", func(t *testing.T) {
		client := Create(nil)
		client.SetRedirectPolicy(NewAllowRedirectPolicy(3))

		resp, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())

		assert.NoError(t, err, "Expected no errors")
		assert.Equal(t, http.StatusOK, resp.StatusCode(), "Expected status code to be 200")
		defer resp.Close() //nolint:errcheck
	})

	t.Run("AllowRedirectPolicy-ExceedsLimit", func(t *testing.T) {
		client := Create(nil)
		client.SetRedirectPolicy(NewAllowRedirectPolicy(1))

		_, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())

		assert.Error(t, err, "Expected to receive redirection limit error")
		assert.EqualError(t, err, "Get \"/redirect-2\": stopped after 1 redirects: too many redirects")
	})

	t.Run("RedirectSpecifiedDomainPolicy", func(t *testing.T) {
		client := Create(&Config{BaseURL: ts.URL})
		host := "127.0.0.1"
		client.SetRedirectPolicy(NewRedirectSpecifiedDomainPolicy(host))

		resp, err := client.Get("/redirect-1").Send(context.Background())

		assert.NoError(t, err, "Expected no errors")
		assert.Equal(t, http.StatusOK, resp.StatusCode(), "Expected status code to be 200")
		defer resp.Close() //nolint:errcheck
	})

	t.Run("RedirectSpecifiedDomainPolicy-ProhibitDomain", func(t *testing.T) {
		client := Create(nil)
		client.SetRedirectPolicy(NewRedirectSpecifiedDomainPolicy("other.domain.com"))

		_, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())

		assert.Error(t, err, "Expected domain restriction error")
		assert.EqualError(t, err, "Get \"/redirect-2\": redirect not allowed", "Expected domain not allowed error")
	})
}

func TestSensitiveHeaderStripping(t *testing.T) {
	t.Run("CrossHostStripsSensitiveHeaders", func(t *testing.T) {
		cur := &http.Request{
			URL:    &url.URL{Scheme: "https", Host: "other.com"},
			Header: http.Header{"Authorization": []string{"Bearer secret"}, "Cookie": []string{"session=abc"}},
		}
		pre := &http.Request{
			URL:    &url.URL{Scheme: "https", Host: "example.com"},
			Header: http.Header{"X-Custom": []string{"value"}},
		}

		checkHostAndAddHeaders(cur, pre)
		assert.Empty(t, cur.Header.Get("Authorization"))
		assert.Empty(t, cur.Header.Get("Cookie"))
	})

	t.Run("SameHostPreservesHeaders", func(t *testing.T) {
		cur := &http.Request{
			URL:    &url.URL{Scheme: "https", Host: "example.com"},
			Header: http.Header{"Authorization": []string{"Bearer token"}},
		}
		pre := &http.Request{
			URL:    &url.URL{Scheme: "https", Host: "example.com"},
			Header: http.Header{"X-Custom": []string{"value"}},
		}

		checkHostAndAddHeaders(cur, pre)
		assert.Equal(t, "value", cur.Header.Get("X-Custom"))
		assert.Equal(t, "Bearer token", cur.Header.Get("Authorization"))
	})

	t.Run("SchemeDowngradeStripsSensitiveHeaders", func(t *testing.T) {
		cur := &http.Request{
			URL:    &url.URL{Scheme: "http", Host: "example.com"},
			Header: http.Header{"Authorization": []string{"Bearer token"}, "Cookie": []string{"session=abc"}},
		}
		pre := &http.Request{
			URL:    &url.URL{Scheme: "https", Host: "example.com"},
			Header: http.Header{"X-Custom": []string{"value"}},
		}

		checkHostAndAddHeaders(cur, pre)
		assert.Empty(t, cur.Header.Get("Authorization"))
		assert.Empty(t, cur.Header.Get("Cookie"))
	})
}

func TestSmartRedirectPolicy(t *testing.T) {
	t.Run("POST to GET on 301", func(t *testing.T) {
		var finalMethod string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/start":
				http.Redirect(w, r, "/final", http.StatusMovedPermanently)
			case "/final":
				finalMethod = r.Method
				_, _ = w.Write([]byte("done"))
			}
		}))
		defer ts.Close()

		client := Create(nil)
		client.SetRedirectPolicy(NewSmartRedirectPolicy(5))

		resp, err := client.Post(ts.URL + "/start").Send(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		assert.Equal(t, http.MethodGet, finalMethod, "POST should be downgraded to GET on 301")
	})

	t.Run("POST to GET on 302", func(t *testing.T) {
		var finalMethod string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/start":
				http.Redirect(w, r, "/final", http.StatusFound)
			case "/final":
				finalMethod = r.Method
				_, _ = w.Write([]byte("done"))
			}
		}))
		defer ts.Close()

		client := Create(nil)
		client.SetRedirectPolicy(NewSmartRedirectPolicy(5))

		resp, err := client.Post(ts.URL + "/start").Send(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		assert.Equal(t, http.MethodGet, finalMethod, "POST should be downgraded to GET on 302")
	})

	t.Run("POST to GET on 303", func(t *testing.T) {
		var finalMethod string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/start":
				http.Redirect(w, r, "/final", http.StatusSeeOther)
			case "/final":
				finalMethod = r.Method
				_, _ = w.Write([]byte("done"))
			}
		}))
		defer ts.Close()

		client := Create(nil)
		client.SetRedirectPolicy(NewSmartRedirectPolicy(5))

		resp, err := client.Post(ts.URL + "/start").Send(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		assert.Equal(t, http.MethodGet, finalMethod, "POST should be downgraded to GET on 303")
	})

	t.Run("GET preserved on 307", func(t *testing.T) {
		var finalMethod string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/start":
				http.Redirect(w, r, "/final", http.StatusTemporaryRedirect)
			case "/final":
				finalMethod = r.Method
				_, _ = w.Write([]byte("done"))
			}
		}))
		defer ts.Close()

		client := Create(nil)
		client.SetRedirectPolicy(NewSmartRedirectPolicy(5))

		resp, err := client.Get(ts.URL + "/start").Send(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())
		assert.Equal(t, http.MethodGet, finalMethod, "GET should be preserved on 307")
	})

	t.Run("HEAD preserved on 303", func(t *testing.T) {
		var finalMethod string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/start":
				http.Redirect(w, r, "/final", http.StatusSeeOther)
			case "/final":
				finalMethod = r.Method
			}
		}))
		defer ts.Close()

		client := Create(nil)
		client.SetRedirectPolicy(NewSmartRedirectPolicy(5))

		_, err := client.Head(ts.URL + "/start").Send(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, http.MethodHead, finalMethod, "HEAD should be preserved on 303")
	})

	t.Run("Exceeds redirect limit", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/loop", http.StatusFound)
		}))
		defer ts.Close()

		client := Create(nil)
		client.SetRedirectPolicy(NewSmartRedirectPolicy(2))

		_, err := client.Get(ts.URL + "/loop").Send(context.Background())
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrTooManyRedirects)
	})
}

func TestDropPayloadHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Content-Length", "42")
	h.Set("Content-Encoding", "gzip")
	h.Set("Transfer-Encoding", "chunked")
	h.Set("Accept", "application/json")

	dropPayloadHeaders(h)

	assert.Empty(t, h.Get("Content-Type"))
	assert.Empty(t, h.Get("Content-Length"))
	assert.Empty(t, h.Get("Content-Encoding"))
	assert.Empty(t, h.Get("Transfer-Encoding"))
	assert.Equal(t, "application/json", h.Get("Accept"), "Non-payload headers should be preserved")
}

func TestGetHostname(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com"},
		{"Example.COM", "example.com"},
		{"example.com:8080", "example.com"},
		{"127.0.0.1:8080", "127.0.0.1"},
		{"[::1]:8080", "::1"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("getHostname(%s)", tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, getHostname(tt.input))
		})
	}
}
