package requests

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRedirectPolicies is a test function for testing the redirect policies
func TestRedirectPolicies(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/no-redirect":
			w.Write([]byte("no redirect"))
		case "/redirect-1":
			http.Redirect(w, r, "/redirect-2", http.StatusFound)
		case "/redirect-2":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			w.Write([]byte("final destination"))
		}
	}))
	defer ts.Close()

	t.Run("NoRedirectPolicy", func(t *testing.T) {
		client := Create(nil)
		client.SetRedirectPolicy(NewNoRedirectPolicy())

		_, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())
		if err == nil {
			t.Error("Expected to receive redirect error, but did not")
		}
		if !errors.Is(err, ErrAutoRedirectDisabled) {
			t.Errorf("Expected error is auto redirect disabled,But get: %v", err)
		}
	})

	t.Run("FlexibleRedirectPolicy-OK", func(t *testing.T) {
		client := Create(nil)
		client.SetRedirectPolicy(NewFlexibleRedirectPolicy(3))

		resp, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())
		if err != nil {
			t.Errorf("Expected no errors, but received: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Errorf("Expected status code to be 200, but received: %d", resp.StatusCode())
		}
	})

	t.Run("FlexibleRedirectPolicy-EXCEEDS LIMIT", func(t *testing.T) {
		client := Create(nil)
		client.SetRedirectPolicy(NewFlexibleRedirectPolicy(1))

		_, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())
		if err == nil {
			t.Error("Expected to receive redirection times exceeding limit error, but not")
		}
	})

	t.Run("DomainCheckRedirectPolicy", func(t *testing.T) {
		client := Create(&Config{BaseURL: ts.URL})
		host := "127.0.0.1"
		client.SetRedirectPolicy(NewDomainCheckRedirectPolicy(host))

		resp, err := client.Get("/redirect-1").Send(context.Background())
		if err != nil {
			t.Errorf("Expected no errors, but received: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Errorf("Expected status code to be 200, but received: %d", resp.StatusCode())
		}
	})

	t.Run("DomainCheckRedirectPolicy-Prohibit Domain Names", func(t *testing.T) {
		client := Create(nil)
		client.SetRedirectPolicy(NewDomainCheckRedirectPolicy("other.domain.com"))

		_, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())
		if err == nil {
			t.Error("Expected domain restriction error, but not")
		}
	})
}
