package requests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

type limitedJSONEncoder struct{}

func (limitedJSONEncoder) Encode(any) (io.Reader, error) {
	payload := `{"message":"hello"}`
	return &io.LimitedReader{R: strings.NewReader(payload), N: int64(len(payload))}, nil
}

func (limitedJSONEncoder) ContentType() string {
	return "application/json"
}

type redirectObservation struct {
	method          string
	body            string
	contentType     string
	contentEncoding string
	trace           string
	referer         string
	cookie          string
}

type redirectResult struct {
	hops       []redirectObservation
	statusCode int
}

func observeRedirect(t *testing.T, status int, send func(string) int) redirectResult {
	t.Helper()
	var hops []redirectObservation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		referer := r.Header.Get("Referer")
		if parsedReferer, err := url.Parse(referer); err == nil && parsedReferer != nil {
			referer = parsedReferer.Path
		}
		hops = append(hops, redirectObservation{
			method:          r.Method,
			body:            string(body),
			contentType:     r.Header.Get("Content-Type"),
			contentEncoding: r.Header.Get("Content-Encoding"),
			trace:           r.Header.Get("X-Trace"),
			referer:         referer,
			cookie:          r.Header.Get("Cookie"),
		})
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", status)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	return redirectResult{
		hops:       hops,
		statusCode: send(server.URL + "/start"),
	}
}

func TestAllowRedirectPolicyMatchesNetHTTPConstruction(t *testing.T) {
	control := observeRedirect(t, http.StatusFound, func(target string) int {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(`{"message":"hello"}`))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("X-Trace", "trace")
		req.AddCookie(&http.Cookie{Name: "session", Value: "cookie"})
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		return resp.StatusCode
	})

	got := observeRedirect(t, http.StatusFound, func(target string) int {
		client := newTestClient(t, WithRedirectPolicy(NewAllowRedirectPolicy(5)))
		resp, err := client.Post(target).
			JSON(map[string]string{"message": "hello"}).
			Header("Content-Encoding", "gzip").
			Header("X-Trace", "trace").
			Cookie("session", "cookie").
			Send(t.Context())
		require.NoError(t, err)
		require.NoError(t, resp.Close())
		return resp.StatusCode()
	})

	assert.Equal(t, control, got)
}

func TestAllowRedirectPolicyMatchesNetHTTPTransitions(t *testing.T) {
	statuses := []int{
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect,
	}
	methods := []string{http.MethodGet, http.MethodPost}
	bodyKinds := []string{"replayable", "one-shot"}

	for _, status := range statuses {
		for _, method := range methods {
			for _, bodyKind := range bodyKinds {
				name := fmt.Sprintf("%d/%s/%s", status, method, bodyKind)
				t.Run(name, func(t *testing.T) {
					control := observeRedirect(t, status, func(target string) int {
						var body io.Reader = strings.NewReader("payload")
						if bodyKind == "one-shot" {
							body = &io.LimitedReader{R: strings.NewReader("payload"), N: int64(len("payload"))}
						}
						req, err := http.NewRequestWithContext(t.Context(), method, target, body)
						require.NoError(t, err)
						req.Header.Set("Content-Type", "text/plain")
						req.Header.Set("Content-Encoding", "identity")
						req.Header.Set("X-Trace", "trace")
						req.AddCookie(&http.Cookie{Name: "session", Value: "cookie"})
						resp, err := http.DefaultClient.Do(req)
						require.NoError(t, err)
						require.NoError(t, resp.Body.Close())
						return resp.StatusCode
					})

					got := observeRedirect(t, status, func(target string) int {
						client := newTestClient(t, WithRedirectPolicy(NewAllowRedirectPolicy(5)))
						builder := client.Request(method, target).
							Header("Content-Encoding", "identity").
							Header("X-Trace", "trace").
							Cookie("session", "cookie")
						if bodyKind == "one-shot" {
							builder.Reader(
								&io.LimitedReader{R: strings.NewReader("payload"), N: int64(len("payload"))},
								"text/plain",
							)
						} else {
							builder.Bytes([]byte("payload")).ContentType("text/plain")
						}
						resp, err := builder.Send(t.Context())
						require.NoError(t, err)
						require.NoError(t, resp.Close())
						return resp.StatusCode()
					})

					assert.Equal(t, control, got)
				})
			}
		}
	}
}

func TestRedirectReplaysBuiltInBodies(t *testing.T) {
	tests := []struct {
		name   string
		status int
		build  func(*Client) *RequestBuilder
	}{
		{
			name:   "JSON across 307",
			status: http.StatusTemporaryRedirect,
			build: func(client *Client) *RequestBuilder {
				return client.Post("/start").JSON(map[string]string{"message": "hello"})
			},
		},
		{
			name:   "form across 308",
			status: http.StatusPermanentRedirect,
			build: func(client *Client) *RequestBuilder {
				return client.Post("/start").Form(url.Values{"message": {"hello"}})
			},
		},
		{
			name:   "bytes across 307",
			status: http.StatusTemporaryRedirect,
			build: func(client *Client) *RequestBuilder {
				return client.Post("/start").Bytes([]byte("hello"))
			},
		},
		{
			name:   "replayable multipart across 308",
			status: http.StatusPermanentRedirect,
			build: func(client *Client) *RequestBuilder {
				return client.Post("/start").Multipart(
					NewMultipart().FileString("upload", "hello.txt", "hello").Replayable(1024),
				)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var bodies []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				bodies = append(bodies, string(body))
				if r.URL.Path == "/start" {
					http.Redirect(w, r, "/final", test.status)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := newTestClient(t, WithBaseURL(server.URL))
			resp, err := test.build(client).Send(t.Context())

			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			require.Len(t, bodies, 2)
			assert.NotEmpty(t, bodies[0])
			assert.Equal(t, bodies[0], bodies[1])
		})
	}
}

func TestRedirectReplaysCustomEncodedBody(t *testing.T) {
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		bodies = append(bodies, string(body))
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/final", http.StatusTemporaryRedirect)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t,
		WithBaseURL(server.URL),
		WithJSONEncoder(limitedJSONEncoder{}),
	)
	resp, err := client.Post("/start").JSON(struct{}{}).Send(t.Context())

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
	require.Len(t, bodies, 2)
	assert.Equal(t, bodies[0], bodies[1])
}

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
		client := newTestClient(t)
		client.setRedirectPolicy(NewProhibitRedirectPolicy())

		_, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())

		assert.Error(t, err, "Expected to receive redirect error")
		assert.ErrorIs(t, err, ErrAutoRedirectDisabled, "Expected auto redirect disabled error")
	})

	t.Run("AllowRedirectPolicy", func(t *testing.T) {
		client := newTestClient(t)
		client.setRedirectPolicy(NewAllowRedirectPolicy(3))

		resp, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())

		assert.NoError(t, err, "Expected no errors")
		assert.Equal(t, http.StatusOK, resp.StatusCode(), "Expected status code to be 200")
		defer resp.Close() //nolint:errcheck // test cleanup closes response body
	})

	t.Run("AllowRedirectPolicy-ExceedsLimit", func(t *testing.T) {
		client := newTestClient(t)
		client.setRedirectPolicy(NewAllowRedirectPolicy(1))

		_, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())

		assert.Error(t, err, "Expected to receive redirection limit error")
		assert.EqualError(t, err, "Get \"/redirect-2\": stopped after 1 redirects: too many redirects")
	})

	t.Run("RedirectSpecifiedDomainPolicy", func(t *testing.T) {
		client := newTestClient(t, WithBaseURL(ts.URL))
		host := "127.0.0.1"
		client.setRedirectPolicy(NewRedirectSpecifiedDomainPolicy(host))

		resp, err := client.Get("/redirect-1").Send(context.Background())

		assert.NoError(t, err, "Expected no errors")
		assert.Equal(t, http.StatusOK, resp.StatusCode(), "Expected status code to be 200")
		defer resp.Close() //nolint:errcheck // test cleanup closes response body
	})

	t.Run("RedirectSpecifiedDomainPolicy-ProhibitDomain", func(t *testing.T) {
		client := newTestClient(t)
		client.setRedirectPolicy(NewRedirectSpecifiedDomainPolicy("other.domain.com"))

		_, err := client.Get(ts.URL + "/redirect-1").Send(context.Background())

		assert.Error(t, err, "Expected domain restriction error")
		assert.EqualError(t, err, "Get \"/redirect-2\": redirect not allowed", "Expected domain not allowed error")
	})
}

func TestRedirectPolicyUsesExampleHost(t *testing.T) {
	var hosts []string
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hosts = append(hosts, r.Host)
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "https://api.example.com/final", http.StatusFound)
		case "/final":
			_, _ = w.Write([]byte("done"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	client := newTestClient(t,
		WithBaseURL("https://api.example.com"),
		WithHTTPClient(ts.Client()),
		WithRedirectPolicy(NewRedirectSpecifiedDomainPolicy("api.example.com")),
	)

	resp, err := client.Get("/redirect").Send(t.Context())
	assert.NoError(t, err)
	defer resp.Close() //nolint:errcheck // test cleanup closes response body
	assert.Equal(t, http.StatusOK, resp.StatusCode())
	assert.Equal(t, []string{"api.example.com", "api.example.com"}, hosts)
}

func TestRedirectPolicyRejectsDifferentExampleHost(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "api.example.com", r.Host)
		http.Redirect(w, r, "https://login.example.com/final", http.StatusFound)
	}))
	defer ts.Close()

	client := newTestClient(t,
		WithBaseURL("https://api.example.com"),
		WithHTTPClient(ts.Client()),
		WithRedirectPolicy(NewRedirectSpecifiedDomainPolicy("api.example.com")),
	)

	_, err := client.Get("/redirect").Send(t.Context())
	assert.ErrorIs(t, err, ErrRedirectNotAllowed)
}

func TestRedirectCredentialPolicyStripsCredentialsAcrossPorts(t *testing.T) {
	type receivedHeaders struct {
		authorization      string
		proxyAuthorization string
		trace              string
	}

	received := make(chan receivedHeaders, 1)
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- receivedHeaders{
			authorization:      r.Header.Get("Authorization"),
			proxyAuthorization: r.Header.Get("Proxy-Authorization"),
			trace:              r.Header.Get("X-Trace"),
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusFound)
	}))
	defer source.Close()

	client := newTestClient(t, WithRedirectPolicy(NewAllowRedirectPolicy(5)))
	resp, err := client.Get(source.URL).
		Header("Authorization", "Bearer secret").
		Header("Proxy-Authorization", "Basic c2VjcmV0").
		Header("X-Trace", "trace").
		Send(t.Context())
	require.NoError(t, err)
	require.NoError(t, resp.Close())

	got := <-received
	assert.Empty(t, got.authorization)
	assert.Empty(t, got.proxyAuthorization)
	assert.Equal(t, "trace", got.trace)
}

func headersAfterRedirect(t *testing.T, sourceURL, destinationURL string) http.Header {
	t.Helper()

	var destinationHeaders http.Header
	requestsSeen := 0
	httpClient := &http.Client{Transport: testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requestsSeen++
		if requestsSeen == 1 {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{destinationURL}},
				Body:       http.NoBody,
				Request:    req,
			}, nil
		}

		destinationHeaders = req.Header.Clone()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       http.NoBody,
			Request:    req,
		}, nil
	})}
	client := newTestClient(t,
		WithHTTPClient(httpClient),
		WithRedirectPolicy(NewAllowRedirectPolicy(5)),
	)

	resp, err := client.Get(sourceURL).
		Header("Authorization", "Bearer secret").
		Header("Proxy-Authorization", "Basic c2VjcmV0").
		Header("X-Trace", "trace").
		Send(t.Context())
	require.NoError(t, err)
	require.NoError(t, resp.Close())
	require.NotNil(t, destinationHeaders)
	return destinationHeaders
}

func TestRedirectCredentialPolicyStripsCredentialsWhenSchemeChanges(t *testing.T) {
	destinationHeaders := headersAfterRedirect(
		t,
		"http://example.test:8443/start",
		"https://example.test:8443/final",
	)

	assert.Empty(t, destinationHeaders.Get("Authorization"))
	assert.Empty(t, destinationHeaders.Get("Proxy-Authorization"))
	assert.Equal(t, "trace", destinationHeaders.Get("X-Trace"))
}

func TestRedirectCredentialPolicyStripsCredentialsWhenHostnameChanges(t *testing.T) {
	destinationHeaders := headersAfterRedirect(
		t,
		"https://source.example:8443/start",
		"https://destination.example:8443/final",
	)

	assert.Empty(t, destinationHeaders.Get("Authorization"))
	assert.Empty(t, destinationHeaders.Get("Proxy-Authorization"))
	assert.Equal(t, "trace", destinationHeaders.Get("X-Trace"))
}

func TestRedirectCredentialPolicyStripsCredentialsOnHTTPSDowngrade(t *testing.T) {
	destinationHeaders := headersAfterRedirect(
		t,
		"https://example.test:8443/start",
		"http://example.test:8443/final",
	)

	assert.Empty(t, destinationHeaders.Get("Authorization"))
	assert.Empty(t, destinationHeaders.Get("Proxy-Authorization"))
	assert.Equal(t, "trace", destinationHeaders.Get("X-Trace"))
}

func TestRedirectCredentialPolicyUsesEffectiveDefaultPort(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		destination string
	}{
		{
			name:        "DNS hostname",
			source:      "http://example.test/start",
			destination: "http://example.test:80/final",
		},
		{
			name:        "IPv6 literal",
			source:      "http://[2001:db8::1]/start",
			destination: "http://[2001:db8::1]:80/final",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			destinationHeaders := headersAfterRedirect(t, test.source, test.destination)

			assert.Equal(t, "Bearer secret", destinationHeaders.Get("Authorization"))
			assert.Equal(t, "Basic c2VjcmV0", destinationHeaders.Get("Proxy-Authorization"))
			assert.Equal(t, "trace", destinationHeaders.Get("X-Trace"))
		})
	}
}

func TestRedirectCredentialPolicyCanonicalizesIDNAHostnames(t *testing.T) {
	destinationHeaders := headersAfterRedirect(
		t,
		"http://b\u00fccher.example/start",
		"http://xn--bcher-kva.example/final",
	)

	assert.Equal(t, "Bearer secret", destinationHeaders.Get("Authorization"))
	assert.Equal(t, "Basic c2VjcmV0", destinationHeaders.Get("Proxy-Authorization"))
	assert.Equal(t, "trace", destinationHeaders.Get("X-Trace"))
}

func TestRedirectCredentialPolicyLeavesCookieScopeToNetHTTP(t *testing.T) {
	destinationURL, err := url.Parse("https://destination.example/final")
	require.NoError(t, err)
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	jar.SetCookies(destinationURL, []*http.Cookie{
		{Name: "target", Value: "cookie", Path: "/final", Secure: true},
		{Name: "wrong-path", Value: "cookie", Path: "/other", Secure: true},
	})

	var destinationHeaders http.Header
	requestsSeen := 0
	httpClient := &http.Client{
		Jar: jar,
		Transport: testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			requestsSeen++
			if requestsSeen == 1 {
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": []string{destinationURL.String()}},
					Body:       http.NoBody,
					Request:    req,
				}, nil
			}

			destinationHeaders = req.Header.Clone()
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    req,
			}, nil
		}),
	}
	client := newTestClient(t,
		WithHTTPClient(httpClient),
		WithRedirectPolicy(NewAllowRedirectPolicy(5)),
	)

	resp, err := client.Get("https://source.example/start").
		Cookie("manual", "source").
		Send(t.Context())
	require.NoError(t, err)
	require.NoError(t, resp.Close())
	require.NotNil(t, destinationHeaders)
	assert.Equal(t, "target=cookie", destinationHeaders.Get("Cookie"))
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

		stripSensitiveHeadersOnRedirect(cur, pre)
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

		stripSensitiveHeadersOnRedirect(cur, pre)
		assert.Empty(t, cur.Header.Get("X-Custom"))
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

		stripSensitiveHeadersOnRedirect(cur, pre)
		assert.Empty(t, cur.Header.Get("Authorization"))
		assert.Empty(t, cur.Header.Get("Cookie"))
	})
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
