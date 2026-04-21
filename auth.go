package requests

import "net/http"

// AuthMethod defines the interface for applying authentication strategies to requests.
type AuthMethod interface {
	// Apply applies the authentication strategy to req.
	Apply(req *http.Request)
	// Valid reports whether the authentication strategy is configured.
	Valid() bool
}

// BasicAuth represents HTTP Basic Authentication credentials.
type BasicAuth struct {
	Username string // Username is the HTTP Basic Authentication username.
	Password string // Password is the HTTP Basic Authentication password.
}

// Apply adds the Basic Auth credentials to the request.
func (b BasicAuth) Apply(req *http.Request) {
	req.SetBasicAuth(b.Username, b.Password)
}

// Valid checks if the Basic Auth credentials are present.
func (b BasicAuth) Valid() bool {
	return b.Username != "" && b.Password != ""
}

// BearerAuth represents an OAuth 2.0 Bearer token.
type BearerAuth struct {
	Token string // Token is the bearer token value.
}

// Apply adds the Bearer token to the request's Authorization header.
func (b BearerAuth) Apply(req *http.Request) {
	if b.Valid() {
		req.Header.Set("Authorization", "Bearer "+b.Token)
	}
}

// Valid checks if the Bearer token is present.
func (b BearerAuth) Valid() bool {
	return b.Token != ""
}

// CustomAuth allows for custom Authorization header values.
type CustomAuth struct {
	Header string // Header is the Authorization header value.
}

// Apply sets a custom Authorization header value.
func (c CustomAuth) Apply(req *http.Request) {
	if c.Valid() {
		req.Header.Set("Authorization", c.Header)
	}
}

// Valid checks if the custom Authorization header value is present.
func (c CustomAuth) Valid() bool {
	return c.Header != ""
}
