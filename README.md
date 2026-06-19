# requests

[![Go Version](https://img.shields.io/badge/go-1.26.4-00ADD8?style=flat-square&logo=go)](go.mod)
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
- **Composable middleware**: Attach header, cookie, or cache middleware at the client or request level.

## Installation

```bash
go get github.com/kaptinlin/requests
```

Requires **Go 1.26.4**.

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
	defer resp.Close()

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
	requests.WithBearerAuth("token"),
	requests.WithRetry(requests.RetryPolicy{Max: 3}),
)
if err != nil {
	log.Fatal(err)
}
```

### Optional profiles

Browser-like defaults:

```go
import (
	"github.com/kaptinlin/requests"
	"github.com/kaptinlin/requests/browser"
)

client, err := requests.New(
	requests.WithProfile(browser.Chrome()),
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

### Forms and files

```go
file, err := os.Open("avatar.png")
if err != nil {
	log.Fatal(err)
}
defer file.Close()

resp, err := client.Post("/upload").
	FormField("user", "alice").
	File("avatar", "avatar.png", file).
	Send(context.Background())
```

For larger multipart bodies, use the streaming multipart builder:

```go
body := requests.NewMultipart().
	Field("user", "alice").
	File("avatar", "avatar.png", file)

resp, err := client.Post("/upload").
	Multipart(body).
	Send(context.Background())
```

Use `Replayable(maxBytes)` when a multipart request must be replayable for retries:

```go
body := requests.NewMultipart().
	Field("user", "alice").
	FileString("note", "note.txt", "hello").
	Replayable(1 << 20)
```

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

### Redirect policies

```go
client, err := requests.New(
	requests.WithRedirectPolicy(requests.NewSmartRedirectPolicy(10)),
)
```

Use `NewAllowRedirectPolicy`, `NewProhibitRedirectPolicy`, or `NewRedirectSpecifiedDomainPolicy` when you need a different redirect strategy.

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

### Save to disk

```go
if err := resp.Save("downloads/report.json"); err != nil {
	log.Fatal(err)
}
```

### Iterate line-oriented responses

```go
for line := range resp.Lines() {
	fmt.Printf("%s\n", line)
}
```

### Classify failures

```go
_, err := client.Get("/health").Send(ctx)
switch {
case requests.IsCanceled(err):
	log.Println("caller canceled the request")
case requests.IsTimeout(err):
	log.Println("request hit a deadline")
case requests.IsConnectionError(err):
	log.Println("dial, DNS, or TLS failed")
}
```

`IsCanceled` matches `context.Canceled` only; `IsTimeout` matches `context.DeadlineExceeded` and `net.Error` timeouts. The two are orthogonal so caller-driven cancel can be told apart from a hit deadline. Sentinel errors in [`errors.go`](errors.go) cover non-transport causes (body replay, redirects, decoding, config) — match them with `errors.Is`.

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

For response caching, use `middlewares.CacheMiddleware` with a `middlewares.Cacher` implementation such as `middlewares.NewMemoryCache()`.

## Documentation

- Development guidance: [CLAUDE.md](CLAUDE.md)
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
