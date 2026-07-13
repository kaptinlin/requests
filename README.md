# requests

[![Go Version](https://img.shields.io/badge/go-1.26.5-00ADD8?style=flat-square&logo=go)](go.mod)
[![License](https://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)](LICENSE)

A fluent HTTP client library for Go with middleware, retries, proxy and redirect controls, caller-owned streaming, ordered-header intent, optional client profiles, and JSON/XML/YAML helpers.

## Features

- **Fluent request builder**: Chain path params, query params, headers, cookies, auth, body encoding, and per-request retry settings.
- **Validated construction**: Build clients with `New(...)`; malformed options return errors before any request is sent.
- **Retry-aware delivery**: Combine retry counts, backoff strategies, and `Retry-After` handling without wrapping `net/http` yourself.
- **Transport controls**: Configure TLS, mTLS, HTTP/2, redirect policies, proxies, bypass rules, resolver/dialer hooks, and connection pooling.
- **Ordered headers**: Express header order as request intent with `orderedobject`, while preserving `net/http` header semantics.
- **Optional profiles**: Apply browser-like headers, TLS ClientHello fingerprints, or HTTP/3 through separate extension modules.
- **`net/http` adapters**: Use configured `requests` clients as `*http.Client` or `http.RoundTripper` in other SDKs.
- **Response helpers**: Decode JSON, XML, or YAML, check status helpers, inspect diagnostics, iterate buffered response lines, or save to disk.
- **Composable middleware**: Attach header or cookie middleware at the client or request level.

## Installation

```bash
go get github.com/kaptinlin/requests
```

Requires **Go 1.26.5**.

Optional extension modules:

```bash
go get github.com/kaptinlin/requests/browser
go get github.com/kaptinlin/requests/fingerprint
go get github.com/kaptinlin/requests/http3
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kaptinlin/requests"
)

type Post struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func main() {
	client, err := requests.New(
		requests.WithBaseURL("https://api.example.com"),
		requests.WithTimeout(10*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Get("/posts/{id}").PathParam("id", "1").Send(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	var post Post
	if err := resp.DecodeJSON(&post); err != nil {
		log.Fatal(err)
	}

	fmt.Println(post.ID, post.Title)
}
```

## Client Construction

### Functional Options

```go
client, err := requests.New(
	requests.WithBaseURL("https://api.example.com"),
	requests.WithTimeout(30*time.Second),
	requests.WithHeaders(http.Header{"X-Client": {"requests"}}),
	requests.WithBearerAuth("token"),
	requests.WithRetry(requests.RetryPolicy{Max: 3}),
)
if err != nil {
	log.Fatal(err)
}
```

`WithHeaders` captures a copy during construction. Later changes to the input
header do not affect the client; a nil header means no default headers.

### Optional profiles

Browser-like defaults:

```go
import (
	"github.com/kaptinlin/requests"
	"github.com/kaptinlin/requests/browser"
)

client, err := requests.New(
	requests.WithProfile(browser.Chrome145()),
)
if err != nil {
	log.Fatal(err)
}
```

Profiles apply client-level defaults. Request-local headers still override profile headers.

TLS fingerprint profile:

```go
import (
	"github.com/kaptinlin/requests"
	"github.com/kaptinlin/requests/fingerprint"
)

client, err := requests.New(
	requests.WithProfile(fingerprint.Chrome()),
)
if err != nil {
	log.Fatal(err)
}
```

HTTP/3 profile:

```go
import (
	"crypto/tls"

	"github.com/kaptinlin/requests"
	"github.com/kaptinlin/requests/http3"
)

client, err := requests.New(
	requests.WithProfile(http3.Profile(http3.WithTLSConfig(&tls.Config{
		MinVersion: tls.VersionTLS13,
	}))),
)
if err != nil {
	log.Fatal(err)
}
```

Optional profile packages keep heavier dependencies out of the core module:

- `github.com/kaptinlin/requests/browser` applies browser-like headers, ordered header metadata, and HTTP/2 preference.
- `github.com/kaptinlin/requests/fingerprint` applies uTLS ClientHello fingerprints.
- `github.com/kaptinlin/requests/http3` applies a QUIC HTTP/3 transport.

### Transport Options

```go
client, err := requests.New(
	requests.WithBaseURL("https://api.example.com"),
	requests.WithTimeout(30*time.Second),
	requests.WithHTTP2(),
	requests.WithTLSServerName("api.example.com"),
	requests.WithDialTimeout(5*time.Second),
	requests.WithTLSHandshakeTimeout(5*time.Second),
	requests.WithResponseHeaderTimeout(10*time.Second),
	requests.WithMaxIdleConnsPerHost(10),
)
if err != nil {
	log.Fatal(err)
}
```

## Making Requests

Builder helpers remain fluent even when encoding can fail. `QueriesStruct`,
`Form`, `FormFields`, request-local `Auth`, and `AddMiddleware` retain the first
preparation error; `Send` or `SendStream` returns it before encoding a body or
dispatching a request. Logs are never the only error channel.

The resolved URL, method, and context are validated by `net/http` before a
streaming body is opened. Invalid request shape returns
`ErrRequestCreationFailed` without reading or closing the caller's body source.

### Ordered headers

```go
import "github.com/kaptinlin/orderedobject"

headers := orderedobject.NewObject[[]string]().
	Set("Accept", []string{"application/json"}).
	Set("User-Agent", []string{"requests-example/1.0"})

resp, err := client.Get("/articles").
	OrderedHeaders(headers).
	Send(context.Background())
```

Default `net/http` transports preserve header semantics. Transports that explicitly read `requests.OrderedHeaders(req)` can use the metadata for wire-order delivery.

Dispatch metadata has one precedence rule across builders, `AsHTTPClient`, and
`AsTransport`: client headers < client auth < request headers < request auth.
Request cookies replace same-name client cookies while preserving other client
defaults. Existing ordered entries are synchronized to the final semantic
header values; pseudo-header intent is preserved separately.

### JSON request body

```go
resp, err := client.Post("/articles").
	Header("X-Trace-ID", "trace-123").
	JSON(map[string]any{"title": "hello"}).
	Send(context.Background())
```

### Path and query parameters

```go
resp, err := client.Get("/articles/{id}").
	PathParam("id", "42").
	Query("include", "comments").
	Send(context.Background())
```

Repeated query keys are preserved:

```go
resp, err := client.Get("/search").
	Query("tag", "go").
	Query("tag", "http").
	Send(context.Background())
```

For non-standard methods, use the method-first request entry:

```go
resp, err := client.Request("PROPFIND", "/files/{id}").
	PathParam("id", "report").
	Send(context.Background())
```

Each request has one body selection. A later `JSON`, `XML`, `YAML`, `Text`,
`Bytes`, `Reader`, `Form`, `FormField`, `FormFields`, or `Multipart` call
replaces the earlier body kind; repeated form-field calls remain additive.
Typed bodies set their media type. `Bytes` keeps a caller-set `Content-Type`
but removes a media type generated by the body it replaces.

### Forms and files

```go
resp, err := client.Post("/upload").
	FormField("user", "alice").
	Send(context.Background())
```

All multipart uploads use the streaming multipart builder:

```go
file, err := os.Open("avatar.png")
if err != nil {
	log.Fatal(err)
}
defer file.Close()

body := requests.NewMultipart().
	Field("user", "alice").
	Part(requests.FilePart{
		Field:       "avatar",
		Filename:    "avatar.png",
		ContentType: "image/png",
		Body:        file,
	})

resp, err := client.Post("/upload").
	Multipart(body).
	Send(context.Background())
```

`FilePart.Body` is borrowed. The caller closes files and other owned sources;
`requests` never closes them by dynamic type. The HTTP transport still owns and
closes the outer request body for each attempt.

Use `Replayable(maxBytes)` when a multipart request must be replayable for
retries or 307/308 redirects:

```go
body := requests.NewMultipart().
	Field("user", "alice").
	FileString("note", "note.txt", "hello").
	Replayable(1 << 20)
```

Buffered built-in bodies (`JSON`, `XML`, `YAML`, `Text`, `Bytes`, and form)
always populate the standard `http.Request.GetBody` contract, even when retry
is disabled. Each retry or body-preserving redirect receives a fresh reader.
`Reader` remains one-shot unless its source is seekable and sized.

## Retries and Delivery

### Client-level retries

```go
client, err := requests.New(
	requests.WithBaseURL("https://api.example.com"),
	requests.WithRetry(requests.RetryPolicy{
		Max: 3,
		Backoff: requests.JitterBackoffStrategy(
			requests.ExponentialBackoffStrategy(250*time.Millisecond, 2, 5*time.Second),
			0.2,
		),
	}),
)
if err != nil {
	log.Fatal(err)
}
```

### Request-level overrides

```go
resp, err := client.Get("/jobs/{id}").
	PathParam("id", "job-1").
	Retry(requests.RetryPolicy{
		Max:     5,
		Backoff: requests.LinearBackoffStrategy(500 * time.Millisecond),
	}).
	Send(context.Background())
```

Use `NoRetry()` on a request to disable a positive client default. Replayable request bodies are restored before retry attempts; non-replayable streaming bodies are attempted once.

The retry logic automatically honors `Retry-After` on `429` and `503` responses.

## `net/http` Integration

Use `AsHTTPClient()` when another SDK accepts `*http.Client`:

```go
httpClient := client.AsHTTPClient()
resp, err := httpClient.Get("https://api.example.com/resource")
```

Use `AsTransport()` when the caller owns the `http.Client`:

```go
httpClient := &http.Client{
	Transport: client.AsTransport(),
}
```

The adapter applies client headers, cookies, auth, and client middleware. It does not run request-builder retries, response buffering, stream responses, or decoding helpers.

`UnsafeHTTPClient()` exposes the live underlying `*http.Client` only for advanced integrations that must inspect or mutate raw transport state. Prefer construction options or adapters for ordinary use.

## Session and Dialing

```go
client, err := requests.New(
	requests.WithSession(),
	requests.WithHTTP2(),
	requests.WithResolver(net.DefaultResolver),
)
if err != nil {
	log.Fatal(err)
}
```

`WithSession()` creates a cookie jar and TLS session cache when missing. `WithDialContext` and `WithLocalAddr` are available for custom gateway and network binding setups.

Root TLS, session, proxy, dial, and pool options configure the standard `*http.Transport`. When a transport profile such as HTTP/3 is active, unsupported later options return `ErrInvalidTransportType`; they never replace the selected protocol silently. Put TLS/session options before an HTTP/3 profile when the profile should consume them.

`WithTLSConfig(nil)` explicitly clears TLS state established by an earlier root
TLS option and restores the active standard transport to Go's TLS defaults. It
still requires an active `*http.Transport`; it does not silently mutate or
replace a custom transport.

Construction rejects invalid auth, nil middleware or redirect policies, and malformed root-certificate PEM. A non-nil client has passed these setup checks; failures are returned from `New` or `Clone` rather than deferred until the first request.

Construction ownership is explicit:

| Input | Captured by `requests` | Still caller-owned |
| --- | --- | --- |
| `WithHeaders` | Header map and value slices | Nothing |
| `WithTLSConfig` and profile TLS options | Standard shallow `tls.Config.Clone` | Referenced slices/maps, callbacks, certificate pools, session cache, key logger, private keys, and parsed leaves |
| `WithCertificates` | Certificate slice, DER bytes, signature algorithms, OCSP staple, and SCT bytes | Private keys and parsed leaves |
| `WithHTTPClient`, `WithTransport`, `WithCookieJar`, custom auth/logger/codecs | Nothing | The supplied collaborator and its concurrency safety |

After construction, do not mutate values listed as caller-owned while the client
may use them. `GetTLSConfig` also returns the standard shallow clone rather than
claiming a full deep copy.

## Proxies and Redirects

### Proxy configuration

```go
client, err := requests.New(
	requests.WithProxyBypass("http://proxy.internal:8080", "localhost,.svc.cluster.local,10.0.0.0/8"),
)
if err != nil {
	log.Fatal(err)
}

rotating, err := requests.New(
	requests.WithProxies("http://proxy1:8080", "http://proxy2:8080"),
)
if err != nil {
	log.Fatal(err)
}
```

Proxy URLs support `http`, `https`, and `socks5` schemes.
Without an explicit proxy option, delivery follows `net/http` and its environment proxy rules. Use `WithoutProxy()` when the client must connect directly regardless of `HTTP_PROXY`, `HTTPS_PROXY`, or `NO_PROXY`.

### Redirect policies

```go
client, err := requests.New(
	requests.WithRedirectPolicy(requests.NewAllowRedirectPolicy(10)),
)
```

Use `NewProhibitRedirectPolicy` or `NewRedirectSpecifiedDomainPolicy` when you
need to reject all redirects or restrict destination hosts. `net/http` owns
redirect method, body, Referer, Cookie, and payload-header transitions.
`NewAllowRedirectPolicy` additionally keeps initial sensitive credentials only
when the redirect target has the same scheme, canonical hostname, and effective
port. Authorization, proxy authorization, and explicit cookie headers are
removed across origins; cookies supplied by the client's Jar still follow
`net/http` domain, path, and secure rules.

## Responses

### Decode structured payloads

```go
var out struct {
	Message string `json:"message"`
}
if err := resp.DecodeJSON(&out); err != nil {
	log.Fatal(err)
}
```

`Decode` parses `Content-Type` with the standard MIME parser. It recognizes
`application/json` and `+json`, `application/xml` / `text/xml` and `+xml`, and
`application/yaml` and `+yaml`. Parameters and case are normalized; malformed
or unsupported types return `ErrUnsupportedContentType` instead of guessing a
codec. `IsContentType` compares parsed base media types exactly.

### Save to disk

```go
if err := resp.Save("downloads/report.json"); err != nil {
	log.Fatal(err)
}
```

`Send` fully reads and closes the transport response body before returning,
including when the read fails. A read failure wraps `ErrResponseReadFailed`;
body-close errors remain best-effort. On success, `resp.Raw().Body` is a new
reader over the owned buffered bytes. `resp.Bytes()` returns a caller-owned copy
and `resp.Header()` returns a header snapshot; mutate `resp.Raw()` only when you
intentionally need the underlying `net/http` response.

Buffered responses require no `Close` call for connection reuse.
`Response.Close` only closes the replacement reader over the buffered bytes.

### Iterate line-oriented responses

```go
for line := range resp.Lines() {
	fmt.Printf("%s\n", line)
}
```

Buffered `Lines` iterates the already-owned response bytes without a scanner
token limit. Each yielded line is a caller-owned copy; CRLF and trailing-newline
handling match `bufio.ScanLines`. `StreamResponse.Lines` remains bounded and
reports oversized-line errors through its two-value iterator.

### Classify failures

```go
_, err := client.Get("/health").Send(ctx)
switch {
case requests.IsCanceled(err):
	log.Println("caller canceled the request")
case requests.IsTimeout(err):
	log.Println("request hit a deadline")
case requests.IsConnectionError(err):
	log.Println("DNS resolution or TCP dial failed")
}
```

`IsCanceled` matches `context.Canceled` only; `IsTimeout` matches
`context.DeadlineExceeded` and `net.Error` timeouts. `IsConnectionError` means
the error chain contains `*net.OpError`, which the standard transport uses for
DNS and TCP dial failures. TLS certificate verification errors remain available
through `errors.As` and are not retried by `DefaultRetryIf`; TLS handshake and
response-header timeouts match `IsTimeout`. Joined retry errors preserve these
`errors.Is` / `errors.As` classifications. Sentinel errors in
[`errors.go`](errors.go) cover non-transport causes (body replay, redirects,
decoding, config).

### Inspect diagnostics

```go
fmt.Println(resp.Elapsed())
fmt.Println(resp.Attempts())
fmt.Println(resp.Protocol())
fmt.Println(resp.TLS() != nil)
```

## Streaming

```go
stream, err := client.Get("/events").SendStream(context.Background())
if err != nil {
	log.Fatal(err)
}
defer stream.Close()

for line, err := range stream.Lines() {
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("event: %s\n", line)
}
```

## Middleware

```go
headers := http.Header{}
headers.Set("X-Client", "requests")

client, err := requests.New(
	requests.WithMiddleware(
		middlewares.HeaderMiddleware(headers),
		middlewares.CookieMiddleware([]*http.Cookie{{Name: "session", Value: "abc"}}),
	),
)
if err != nil {
	log.Fatal(err)
}
```

## Logging

`Logger` contains only the four formatted emission methods used by the request
pipeline. The built-in logger keeps level configuration on its concrete type:

```go
logger := requests.NewDefaultLogger(os.Stderr, requests.LevelInfo)
logger.SetLevel(requests.LevelDebug)

client, err := requests.New(requests.WithLogger(logger))
```

Without `WithLogger`, clients remain silent. Custom loggers do not implement
`SetLevel` unless their own API needs it.

## Documentation

- Development guidance: [AGENTS.md](AGENTS.md)
- API and contract details: [SPECS/](SPECS/)
- Release handoff: [RELEASE.md](RELEASE.md)
- Package docs: [pkg.go.dev/github.com/kaptinlin/requests](https://pkg.go.dev/github.com/kaptinlin/requests)
- Browser profile docs: [pkg.go.dev/github.com/kaptinlin/requests/browser](https://pkg.go.dev/github.com/kaptinlin/requests/browser)
- TLS fingerprint profile docs: [pkg.go.dev/github.com/kaptinlin/requests/fingerprint](https://pkg.go.dev/github.com/kaptinlin/requests/fingerprint)
- HTTP/3 profile docs: [pkg.go.dev/github.com/kaptinlin/requests/http3](https://pkg.go.dev/github.com/kaptinlin/requests/http3)

## Development

```bash
task test            # Run root tests with race detection
task test:all        # Run root and extension tests with race detection
task test:published  # Verify extensions outside go.work after a root release
task lint            # Run root golangci-lint and tidy checks
task lint:all        # Run root and extension linters
task tidy:all        # Tidy root and extension modules
task vuln:all        # Run root and extension vulnerability checks
task verify          # Run deps, fmt, vet, lint, test, and vuln checks for root
task verify:all      # Run full root and extension verification
```

## Contributing

Contributions are welcome. Keep changes focused. Run `task test` plus
`task lint` for root-only changes, and `task test:all` plus `task lint:all`
when a change touches extension modules or shared contracts.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
