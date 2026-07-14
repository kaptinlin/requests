package requests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaptinlin/orderedobject"
	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

func TestMetadataPrecedenceClientAuthOverridesClientHeader(t *testing.T) {
	tests := []struct {
		name string
		send func(*testing.T, *Client, string)
	}{
		{
			name: "builder",
			send: func(t *testing.T, client *Client, _ string) {
				resp, err := client.Get("/").Send(t.Context())
				require.NoError(t, err)
				require.NoError(t, resp.Close())
			},
		},
		{
			name: "AsHTTPClient",
			send: func(t *testing.T, client *Client, target string) {
				resp, err := client.AsHTTPClient().Get(target)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
			},
		},
		{
			name: "AsTransport",
			send: func(t *testing.T, client *Client, target string) {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
				require.NoError(t, err)
				resp, err := client.AsTransport().RoundTrip(req)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, []string{"Bearer client-token"}, r.Header.Values("Authorization"))
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := newTestClient(t,
				WithBaseURL(server.URL),
				WithHTTPClient(server.Client()),
				WithHeaders(http.Header{"Authorization": {"client-header"}}),
				WithBearerAuth("client-token"),
			)
			test.send(t, client, server.URL)
		})
	}
}

func TestMetadataPrecedenceRequestHeadersOverrideClientAuth(t *testing.T) {
	tests := []struct {
		name string
		send func(*testing.T, *Client, string)
	}{
		{
			name: "builder",
			send: func(t *testing.T, client *Client, _ string) {
				resp, err := client.Get("/").Headers(http.Header{
					"authorization": {"request-first", "request-second"},
				}).Send(t.Context())
				require.NoError(t, err)
				require.NoError(t, resp.Close())
			},
		},
		{
			name: "AsHTTPClient",
			send: func(t *testing.T, client *Client, target string) {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
				require.NoError(t, err)
				req.Header["authorization"] = []string{"request-first", "request-second"}
				resp, err := client.AsHTTPClient().Do(req)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
			},
		},
		{
			name: "AsTransport",
			send: func(t *testing.T, client *Client, target string) {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
				require.NoError(t, err)
				req.Header["authorization"] = []string{"request-first", "request-second"}
				resp, err := client.AsTransport().RoundTrip(req)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, []string{"request-first", "request-second"}, r.Header.Values("Authorization"))
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := newTestClient(t,
				WithBaseURL(server.URL),
				WithHTTPClient(server.Client()),
				WithHeaders(http.Header{"Authorization": {"client-header"}}),
				WithBearerAuth("client-token"),
			)
			test.send(t, client, server.URL)
		})
	}
}

func TestMetadataPrecedenceRequestAuthSynchronizesOrderedAuthorization(t *testing.T) {
	clientOrdered := orderedobject.New[[]string]().
		Set("Authorization", []string{"client-header"}).
		Set("X-Client", []string{"client"})
	requestOrdered := orderedobject.New[[]string]().
		Set(":authority", []string{"api.example.com"}).
		Set("authorization", []string{"request-header"})
	client := newTestClient(t,
		WithOrderedHeaders(clientOrdered),
		WithBearerAuth("client-token"),
		WithTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, []string{"Bearer request-token"}, req.Header.Values("Authorization"))
			ordered, ok := OrderedHeaders(req)
			require.True(t, ok)
			auth, ok := ordered.Get("authorization")
			require.True(t, ok)
			assert.Equal(t, []string{"Bearer request-token"}, auth)
			authority, ok := ordered.Get(":authority")
			require.True(t, ok)
			assert.Equal(t, []string{"api.example.com"}, authority)
			return &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       http.NoBody,
				Request:    req,
			}, nil
		})),
	)

	resp, err := client.Get("https://example.com").
		OrderedHeaders(requestOrdered).
		Auth(BearerAuth{Token: "request-token"}).
		Send(t.Context())

	require.NoError(t, err)
	require.NoError(t, resp.Close())
}

func TestMetadataPrecedenceAdaptersSynchronizeOrderedAuthorization(t *testing.T) {
	tests := []struct {
		name string
		send func(*testing.T, *Client)
	}{
		{
			name: "AsHTTPClient",
			send: func(t *testing.T, client *Client) {
				resp, err := client.AsHTTPClient().Get("https://example.com")
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
			},
		},
		{
			name: "AsTransport",
			send: func(t *testing.T, client *Client) {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.com", nil)
				require.NoError(t, err)
				resp, err := client.AsTransport().RoundTrip(req)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ordered := orderedobject.New[[]string]().
				Set(":authority", []string{"api.example.com"}).
				Set("Authorization", []string{"client-header"})
			client := newTestClient(t,
				WithOrderedHeaders(ordered),
				WithBearerAuth("client-token"),
				WithTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, []string{"Bearer client-token"}, req.Header.Values("Authorization"))
					gotOrdered, ok := OrderedHeaders(req)
					require.True(t, ok)
					auth, ok := gotOrdered.Get("Authorization")
					require.True(t, ok)
					assert.Equal(t, []string{"Bearer client-token"}, auth)
					authority, ok := gotOrdered.Get(":authority")
					require.True(t, ok)
					assert.Equal(t, []string{"api.example.com"}, authority)
					return &http.Response{
						Status:     "200 OK",
						StatusCode: http.StatusOK,
						Header:     http.Header{},
						Body:       http.NoBody,
						Request:    req,
					}, nil
				})),
			)

			test.send(t, client)
		})
	}
}

func TestMetadataPrecedenceRequestCookiesOverrideByName(t *testing.T) {
	tests := []struct {
		name string
		send func(*testing.T, *Client, string)
	}{
		{
			name: "builder",
			send: func(t *testing.T, client *Client, _ string) {
				resp, err := client.Get("/").
					Cookie("shared", "request").
					Cookie("local", "request").
					Send(t.Context())
				require.NoError(t, err)
				require.NoError(t, resp.Close())
			},
		},
		{
			name: "AsHTTPClient",
			send: func(t *testing.T, client *Client, target string) {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
				require.NoError(t, err)
				req.AddCookie(&http.Cookie{Name: "shared", Value: "request"})
				req.AddCookie(&http.Cookie{Name: "local", Value: "request"})
				resp, err := client.AsHTTPClient().Do(req)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
			},
		},
		{
			name: "AsTransport",
			send: func(t *testing.T, client *Client, target string) {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
				require.NoError(t, err)
				req.AddCookie(&http.Cookie{Name: "shared", Value: "request"})
				req.AddCookie(&http.Cookie{Name: "local", Value: "request"})
				resp, err := client.AsTransport().RoundTrip(req)
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				values := map[string]string{}
				counts := map[string]int{}
				for _, cookie := range r.Cookies() {
					values[cookie.Name] = cookie.Value
					counts[cookie.Name]++
				}
				assert.Equal(t, map[string]string{
					"default": "client",
					"shared":  "request",
					"local":   "request",
				}, values)
				assert.Equal(t, 1, counts["shared"])
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := newTestClient(t,
				WithBaseURL(server.URL),
				WithHTTPClient(server.Client()),
				WithCookies(map[string]string{
					"default": "client",
					"shared":  "client",
				}),
			)
			test.send(t, client, server.URL)
		})
	}
}

func TestMetadataPrecedenceSynchronizesOrderedCookies(t *testing.T) {
	clientOrdered := orderedobject.New[[]string]().
		Set("Cookie", []string{"default=client; shared=client"})
	requestOrdered := orderedobject.New[[]string]().
		Set(":authority", []string{"api.example.com"}).
		Set("cookie", []string{"shared=request"})
	client := newTestClient(t,
		WithOrderedHeaders(clientOrdered),
		WithTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, []string{"default=client; shared=request; local=request"}, req.Header.Values("Cookie"))
			ordered, ok := OrderedHeaders(req)
			require.True(t, ok)
			cookies, ok := ordered.Get("cookie")
			require.True(t, ok)
			assert.Equal(t, req.Header.Values("Cookie"), cookies)
			authority, ok := ordered.Get(":authority")
			require.True(t, ok)
			assert.Equal(t, []string{"api.example.com"}, authority)
			return &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       http.NoBody,
				Request:    req,
			}, nil
		})),
	)

	resp, err := client.Get("https://example.com").
		OrderedHeaders(requestOrdered).
		Cookie("local", "request").
		Send(t.Context())

	require.NoError(t, err)
	require.NoError(t, resp.Close())
}

func TestMetadataPrecedencePreservesMiddlewareRetryCadence(t *testing.T) {
	var middlewareCalls atomic.Int64
	var transportCalls atomic.Int64
	client := newTestClient(t,
		WithRetry(RetryPolicy{Max: 1, Backoff: DefaultBackoffStrategy(0)}),
		WithTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			status := http.StatusInternalServerError
			if transportCalls.Add(1) == 2 {
				status = http.StatusOK
			}
			return &http.Response{
				Status:     http.StatusText(status),
				StatusCode: status,
				Header:     http.Header{},
				Body:       http.NoBody,
				Request:    req,
			}, nil
		})),
	)
	client.addMiddleware(func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			middlewareCalls.Add(1)
			return next(req)
		}
	})

	resp, err := client.Get("https://example.com").Send(t.Context())

	require.NoError(t, err)
	assert.Equal(t, 2, resp.Attempts())
	assert.Equal(t, int64(1), middlewareCalls.Load())
	assert.Equal(t, int64(2), transportCalls.Load())
}

func TestAsHTTPClientAppliesClientDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "value", r.Header.Get("X-Default"))
		assert.Equal(t, "middleware", r.Header.Get("X-Middleware"))
		assert.Equal(t, "Basic dXNlcjpwYXNz", r.Header.Get("Authorization"))
		cookie, err := r.Cookie("session")
		require.NoError(t, err)
		assert.Equal(t, "abc", cookie.Value)
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	client := newTestClient(t,
		WithTimeout(5*time.Second),
		WithHeader("X-Default", "value"),
		WithBasicAuth("user", "pass"),
		WithCookies(map[string]string{"session": "abc"}),
	)
	client.addMiddleware(func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			req.Header.Set("X-Middleware", "middleware")
			return next(req)
		}
	})

	httpClient := client.AsHTTPClient()
	require.Equal(t, 5*time.Second, httpClient.Timeout)

	resp, err := httpClient.Get(server.URL)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAsTransportDoesNotMutateOriginalRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "value", r.Header.Get("X-Default"))
		defaultCookie, err := r.Cookie("default")
		require.NoError(t, err)
		assert.Equal(t, "client", defaultCookie.Value)
		localCookie, err := r.Cookie("local")
		require.NoError(t, err)
		assert.Equal(t, "request", localCookie.Value)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t,
		WithHeader("X-Default", "value"),
		WithCookies(map[string]string{"default": "client"}),
	)
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "local", Value: "request"})
	originalHeaders := req.Header.Clone()

	resp, err := client.AsTransport().RoundTrip(req)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()

	assert.Empty(t, req.Header.Get("X-Default"))
	assert.Equal(t, originalHeaders, req.Header)
	_, err = req.Cookie("default")
	assert.Error(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAsHTTPClientAppliesDefaultsToExampleHost(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "api.example.com", r.Host)
		assert.Equal(t, "value", r.Header.Get("X-Default"))
		assert.Equal(t, "middleware", r.Header.Get("X-Middleware"))
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
		cookie, err := r.Cookie("session")
		require.NoError(t, err)
		assert.Equal(t, "abc", cookie.Value)
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	client := newTestClient(t,
		WithHTTPClient(server.Client()),
		WithHeader("X-Default", "value"),
		WithBearerAuth("token"),
		WithCookies(map[string]string{"session": "abc"}),
	)
	client.addMiddleware(func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			req.Header.Set("X-Middleware", "middleware")
			return next(req)
		}
	})

	resp, err := client.AsHTTPClient().Get("https://api.example.com/resource")
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAsTransportAppliesDefaultsToExampleHost(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "api.example.com", r.Host)
		assert.Equal(t, "value", r.Header.Get("X-Default"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, WithHTTPClient(server.Client()), WithHeader("X-Default", "value"))
	req, err := http.NewRequest(http.MethodGet, "https://api.example.com/resource", nil)
	require.NoError(t, err)

	resp, err := client.AsTransport().RoundTrip(req)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()

	assert.Empty(t, req.Header.Get("X-Default"))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAsTransportAttachesDefaultOrderedHeaders(t *testing.T) {
	headers := orderedobject.New[[]string]().
		Set("X-First", []string{"1"}).
		Set(":authority", []string{"metadata-only"}).
		Set("X-Second", []string{"2"})

	client := newTestClient(t,
		WithOrderedHeaders(headers),
		WithTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "1", req.Header.Get("X-First"))
			assert.Equal(t, "2", req.Header.Get("X-Second"))
			assert.Empty(t, req.Header.Get(":authority"))

			ordered, ok := OrderedHeaders(req)
			require.True(t, ok)
			assert.Equal(t, []string{"X-First", ":authority", "X-Second"}, ordered.Keys())

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{},
				Body:       http.NoBody,
				Request:    req,
			}, nil
		})),
	)

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	resp, err := client.AsTransport().RoundTrip(req)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAsTransportDropsOrderedMetadataForOriginalHeaderOverrides(t *testing.T) {
	headers := orderedobject.New[[]string]().
		Set("X-First", []string{"default"}).
		Set("X-Keep", []string{"default"})

	client := newTestClient(t,
		WithOrderedHeaders(headers),
		WithTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "request", req.Header.Get("X-First"))
			assert.Equal(t, "default", req.Header.Get("X-Keep"))

			ordered, ok := OrderedHeaders(req)
			require.True(t, ok)
			assert.Equal(t, []string{"X-Keep"}, ordered.Keys())

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{},
				Body:       http.NoBody,
				Request:    req,
			}, nil
		})),
	)

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)
	req.Header.Set("X-First", "request")

	resp, err := client.AsTransport().RoundTrip(req)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAsTransportCanonicalizesOriginalHeaderOverrides(t *testing.T) {
	headers := orderedobject.New[[]string]().
		Set("X-First", []string{"default"}).
		Set("X-Keep", []string{"default"})

	client := newTestClient(t,
		WithOrderedHeaders(headers),
		WithTransport(testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "request", req.Header.Get("X-First"))
			assert.Equal(t, "default", req.Header.Get("X-Keep"))

			ordered, ok := OrderedHeaders(req)
			require.True(t, ok)
			assert.Equal(t, []string{"X-Keep"}, ordered.Keys())

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{},
				Body:       http.NoBody,
				Request:    req,
			}, nil
		})),
	)

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)
	req.Header["x-first"] = []string{"request"}

	resp, err := client.AsTransport().RoundTrip(req)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, resp.Body.Close())
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
