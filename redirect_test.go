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

	// Test ProhibitRedirectPolicy
	t.Run("ProhibitRedirectPolicy", func(t *testing.T) {
		client := Create(nil)
		client.SetRedirectPolicy(NewProhibitRedirectPolicy())

		_, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())
		if err == nil {
			t.Error("Expected to receive redirect error, but did not")
		}
		if !errors.Is(err, ErrAutoRedirectDisabled) {
			t.Errorf("Expected error is auto redirect disabled,But get: %v", err)
		}
	})

	// Test AllowRedirectPolicy
	t.Run("AllowRedirectPolicy-OK", func(t *testing.T) {
		client := Create(nil)
		client.SetRedirectPolicy(NewAllowRedirectPolicy(3))

		resp, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())
		if err != nil {
			t.Errorf("Expected no errors, but received: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Errorf("Expected status code to be 200, but received: %d", resp.StatusCode())
		}
	})

	// Test AllowRedirectPolicy-EXCEEDS LIMIT
	t.Run("AllowRedirectPolicy-EXCEEDS LIMIT", func(t *testing.T) {
		client := Create(nil)
		client.SetRedirectPolicy(NewAllowRedirectPolicy(1))

		_, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())
		if err == nil {
			t.Error("Expected to receive redirection times exceeding limit error, but not")
		}
	})

	// Test RedirectSpecifiedDomainPolicy
	t.Run("RedirectSpecifiedDomainPolicy", func(t *testing.T) {
		client := Create(&Config{BaseURL: ts.URL})
		host := "127.0.0.1"
		client.SetRedirectPolicy(NewRedirectSpecifiedDomainPolicy(host))

		resp, err := client.Get("/redirect-1").Send(context.Background())
		if err != nil {
			t.Errorf("Expected no errors, but received: %v", err)
		}
		if resp.StatusCode() != http.StatusOK {
			t.Errorf("Expected status code to be 200, but received: %d", resp.StatusCode())
		}
	})

	// Test RedirectSpecifiedDomainPolicy-Prohibit Domain Names
	t.Run("RedirectSpecifiedDomainPolicy-Prohibit Domain Names", func(t *testing.T) {
		client := Create(nil)
		client.SetRedirectPolicy(NewRedirectSpecifiedDomainPolicy("other.domain.com"))

		_, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())
		if err == nil {
			t.Error("Expected domain restriction error, but not")
		}
	})
}
