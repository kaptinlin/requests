package requests

import (
	"context"
	"net/http"
	"net/http/httptest"
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
		assert.EqualError(t, err, "Get \"/redirect-2\": redirect is not allowed as per RedirectSpecifiedDomainPolicy", "Expected domain not allowed error")
	})
}
