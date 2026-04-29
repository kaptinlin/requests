package requests

import (
	"net/http"
	"slices"
)

// AsHTTPClient returns a standard-library client that applies this client's defaults.
//
// The returned client preserves the underlying timeout, cookie jar, redirect
// policy, and transport at the time AsHTTPClient is called. Its transport applies
// client headers, cookies, auth, and client-level middleware. RequestBuilder-only
// behavior such as retries, response buffering, streaming callbacks, and decoding
// helpers is not part of the standard-library client.
func (c *Client) AsHTTPClient() *http.Client {
	snap := c.snapshot()
	source := snap.HTTPClient

	client := &http.Client{
		Transport: newClientDefaultsTransport(snap),
	}
	if source == nil {
		return client
	}

	client.Timeout = source.Timeout
	client.Jar = source.Jar
	client.CheckRedirect = source.CheckRedirect
	return client
}

// AsTransport returns a RoundTripper that applies this client's defaults.
//
// Use it when another library owns the *http.Client and only lets callers
// replace http.Client.Transport. The returned transport snapshots client
// headers, cookies, auth, middleware, and the underlying transport at call time.
func (c *Client) AsTransport() http.RoundTripper {
	return newClientDefaultsTransport(c.snapshot())
}

type clientDefaultsTransport struct {
	snap clientSnapshot
	base http.RoundTripper
}

func newClientDefaultsTransport(snap clientSnapshot) http.RoundTripper {
	return &clientDefaultsTransport{
		snap: snap,
		base: baseTransport(snap.HTTPClient),
	}
}

func baseTransport(client *http.Client) http.RoundTripper {
	if client != nil && client.Transport != nil {
		return client.Transport
	}
	return http.DefaultTransport
}

func (t *clientDefaultsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := cloneWithClientDefaults(req, t.snap)

	handler := MiddlewareHandlerFunc(func(req *http.Request) (*http.Response, error) {
		return t.base.RoundTrip(req)
	})
	for _, mw := range slices.Backward(t.snap.Middlewares) {
		handler = mw(handler)
	}

	return handler(cloned)
}

func cloneWithClientDefaults(req *http.Request, snap clientSnapshot) *http.Request {
	cloned := req.Clone(req.Context())
	original := req.Header.Clone()

	cloned.Header = snap.Headers.Clone()
	for _, cookie := range snap.Cookies {
		if cookie != nil {
			cloned.AddCookie(cookie)
		}
	}
	if snap.auth != nil {
		snap.auth.Apply(cloned)
	}
	for key, values := range original {
		cloned.Header[key] = slices.Clone(values)
	}

	return cloned
}
