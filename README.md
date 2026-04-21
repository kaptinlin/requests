# requests

[![Go Version](https://img.shields.io/badge/go-1.26+-00ADD8?style=flat-square&logo=go)](go.mod)
[![License](https://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)](LICENSE)

A fluent HTTP client library for Go with middleware, retries, proxy and redirect controls, streaming callbacks, and JSON/XML/YAML helpers

## Features

- **Fluent request builder**: Chain path params, query params, headers, cookies, auth, body encoding, and per-request retry settings.
- **Multiple client entry points**: Start with `New(...)`, `URL(...)`, or `Create(&Config{...})` depending on how much control you need.
- **Retry-aware delivery**: Combine retry counts, backoff strategies, and `Retry-After` handling without wrapping `net/http` yourself.
- **Transport controls**: Configure TLS, mTLS, HTTP/2, redirect policies, proxies, bypass rules, and connection pooling.
- **Response helpers**: Decode JSON, XML, or YAML, iterate line streams, inspect status helpers, or save to disk.
- **Composable middleware**: Attach header, cookie, or cache middleware at the client or request level.

## Installation

```bash
go get github.com/kaptinlin/requests
```

Requires **Go 1.26+**.

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
	client := requests.New(
		requests.WithBaseURL("https://jsonplaceholder.typicode.com"),
		requests.WithTimeout(10*time.Second),
	)

	resp, err := client.Get("/posts/{id}").PathParam("id", "1").Send(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Close()

	var post Post
	if err := resp.ScanJSON(&post); err != nil {
		log.Fatal(err)
	}

	fmt.Println(post.ID, post.Title)
}
```

## Client Construction

### Functional options

```go
client := requests.New(
	requests.WithBaseURL("https://api.example.com"),
	requests.WithTimeout(30*time.Second),
	requests.WithBearerAuth("token"),
	requests.WithMaxRetries(3),
)
```

### URL shortcut

```go
client := requests.URL("https://api.example.com")
```

### Full config

```go
cfg := &requests.Config{
	BaseURL:               "https://api.example.com",
	Timeout:               30 * time.Second,
	HTTP2:                 true,
	TLSServerName:         "api.example.com",
	DialTimeout:           5 * time.Second,
	TLSHandshakeTimeout:   5 * time.Second,
	ResponseHeaderTimeout: 10 * time.Second,
	MaxIdleConnsPerHost:   10,
}
if err := cfg.Validate(); err != nil {
	log.Fatal(err)
}

client := requests.Create(cfg)
```

## Making Requests

### JSON request body

```go
resp, err := client.Post("/articles").
	Header("X-Trace-ID", "trace-123").
	JSONBody(map[string]any{"title": "hello"}).
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

## Retries and Delivery

### Client-level retries

```go
client := requests.New(
	requests.WithBaseURL("https://api.example.com"),
	requests.WithMaxRetries(3),
	requests.WithRetryStrategy(
		requests.JitterBackoffStrategy(
			requests.ExponentialBackoffStrategy(250*time.Millisecond, 2, 5*time.Second),
			0.2,
		),
	),
)
```

### Request-level overrides

```go
resp, err := client.Get("/jobs/{id}").
	PathParam("id", "job-1").
	MaxRetries(5).
	RetryStrategy(requests.LinearBackoffStrategy(500 * time.Millisecond)).
	Send(context.Background())
```

The retry logic automatically honors `Retry-After` on `429` and `503` responses.

## Proxies and Redirects

### Proxy configuration

```go
if err := client.SetProxyWithBypass("http://proxy.internal:8080", "localhost,.svc.cluster.local,10.0.0.0/8"); err != nil {
	log.Fatal(err)
}

if err := client.SetProxies("http://proxy1:8080", "http://proxy2:8080"); err != nil {
	log.Fatal(err)
}
```

### Redirect policies

```go
client.SetRedirectPolicy(requests.NewSmartRedirectPolicy(10))
```

Use `NewAllowRedirectPolicy`, `NewProhibitRedirectPolicy`, or `NewRedirectSpecifiedDomainPolicy` when you need a different redirect strategy.

## Responses

### Decode structured payloads

```go
var out struct {
	Message string `json:"message"`
}
if err := resp.ScanJSON(&out); err != nil {
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
_, err := client.Get("/health").Send(context.Background())
if requests.IsTimeout(err) {
	log.Println("request timed out")
}
if requests.IsConnectionError(err) {
	log.Println("connection failed")
}
```

## Streaming

```go
_, err := client.Get("/events").
	Stream(func(line []byte) error {
		fmt.Printf("event: %s\n", line)
		return nil
	}).
	StreamErr(func(err error) {
		log.Printf("stream error: %v", err)
	}).
	StreamDone(func() {
		log.Println("stream closed")
	}).
	Send(context.Background())
```

## Middleware

```go
headers := http.Header{}
headers.Set("X-Client", "requests")

client.AddMiddleware(
	middlewares.HeaderMiddleware(headers),
	middlewares.CookieMiddleware([]*http.Cookie{{Name: "session", Value: "abc"}}),
)
```

For response caching, use `middlewares.CacheMiddleware` with a `middlewares.Cacher` implementation such as `middlewares.NewMemoryCache()`.

## Documentation

- Development guidance: [CLAUDE.md](CLAUDE.md)
- API and contract details: [SPECS/](SPECS/)
- Package docs: [pkg.go.dev/github.com/kaptinlin/requests](https://pkg.go.dev/github.com/kaptinlin/requests)

## Development

```bash
task test      # Run all tests with race detection
task lint      # Run golangci-lint and tidy checks
task verify    # Run deps, fmt, vet, lint, test, and vuln checks
```

## Contributing

Contributions are welcome. Open an issue or pull request with a focused change, and run `task test` plus `task lint` before submitting.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
