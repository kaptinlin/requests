# Golang Requests Library

The Requests library simplifies the way you make HTTP requests in Go. It provides an easy-to-use interface for sending requests and handling responses, reducing the boilerplate code typically associated with the `net/http` package.

## Quick Start

Begin by installing the Requests library:

```bash
go get github.com/kaptinlin/requests
```

Creating a new HTTP client and making a request is straightforward:

```go
package main

import (
    "github.com/kaptinlin/requests"
    "log"
)

func main() {
    // Create a client using a base URL
    client := requests.URL("http://example.com")

    // Alternatively, create a client with custom configuration
    client = requests.Create(&requests.Config{
        BaseURL: "http://example.com",
        Timeout: 30 * time.Second,
    })

    // Perform a GET request
    resp, err := client.Get("/resource")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Close()

    log.Println(resp.String())
}
```

## Overview

### Client

The `Client` struct is your gateway to making HTTP requests. You can configure it to your needs, setting default headers, cookies, timeout durations, and more.

#### Usage Example:

```go
client := requests.URL("http://example.com")

// Or, with full configuration
client = requests.Create(&requests.Config{
    BaseURL: "http://example.com",
    Timeout: 5 * time.Second,
    Headers: &http.Header{
        "Content-Type": []string{"application/json"},
    },
})
```

For more details, see [docs/client.md](docs/client.md).


### Request

The library provides a `RequestBuilder` to construct and dispatch HTTP requests. Here are examples of performing various types of requests, including adding query parameters, setting headers, and attaching a body to your requests.

#### GET Request

To retrieve data from a specific resource:

```go
resp, err := client.Get("/path").
    Query("search", "query").
    Header("Accept", "application/json").
    Send(context.Background())
```

#### POST Request

To submit data to be processed to a specific resource:

```go
resp, err := client.Post("/path").
    Header("Content-Type", "application/json").
    JSONBody(map[string]any{"key": "value"}).
    Send(context.Background())
```

#### PUT Request

To replace all current representations of the target resource with the request payload:

```go
resp, err := client.Put("/articles/{article_id}").
    PathParam("article_id", "123456").
    JSONBody(map[string]any{"updatedKey": "newValue"}).
    Send(context.Background())
```

#### DELETE Request

To remove all current representations of the target resource:

```go
resp, err := client.Delete("/articles/{article_id}").
    PathParam("article_id", "123456").
    Send(context.Background())
```

For more details, visit [docs/request.md](docs/request.md).

### Response

Handling responses is crucial in determining the outcome of your HTTP requests. The Requests library makes it easy to check status codes, read headers, and parse the body content.

#### Example

Parsing JSON response into a Go struct:

```go
type APIResponse struct {
    Data string `json:"data"`
}

var apiResp APIResponse
if err := resp.ScanJSON(&apiResp); err != nil {
    log.Fatal(err)
}

log.Printf("Status Code: %d\n", resp.StatusCode())
log.Printf("Response Data: %s\n", apiResp.Data)
```

This example demonstrates how to unmarshal a JSON response and check the HTTP status code.

Additional status helpers: `IsSuccess()`, `IsError()`, `IsClientError()`, `IsServerError()`, `IsRedirect()`.

For more on handling responses, see [docs/response.md](docs/response.md).

### Proxy

Configure proxy settings with optional bypass rules:

```go
// Single proxy
client.SetProxy("http://proxy:8080")

// Multiple proxies with round-robin rotation (retries auto-rotate)
client.SetProxies("http://proxy1:8080", "http://proxy2:8080", "socks5://proxy3:1080")

// Proxy with NO_PROXY bypass list
client.SetProxyWithBypass("http://proxy:8080", "localhost, .internal.com, 10.0.0.0/8")

// Use environment variables (HTTP_PROXY, HTTPS_PROXY, NO_PROXY)
client.SetProxyFromEnv()
```

### Redirect Policies

Control redirect behavior including browser-like method downgrade:

```go
// Smart redirect: downgrades POST→GET on 301/302/303, strips sensitive headers cross-host
client.SetRedirectPolicy(requests.NewSmartRedirectPolicy(10))
```

For more details, see [docs/client.md](docs/client.md).

### Transport Timeouts & Connection Pool

Fine-grained control over request phases and connection pooling:

```go
client := requests.Create(&requests.Config{
    BaseURL:               "https://api.example.com",
    Timeout:               60 * time.Second,  // overall deadline
    DialTimeout:           5 * time.Second,   // TCP connect
    TLSHandshakeTimeout:   5 * time.Second,   // TLS negotiation
    ResponseHeaderTimeout: 10 * time.Second,  // time to first byte
    MaxIdleConnsPerHost:   10,                // connection pool tuning
})
```

### Error Introspection

Classify errors without manual type assertion chains:

```go
_, err := client.Get("/resource").Send(ctx)
if requests.IsTimeout(err) {
    // Handle timeout (context deadline, net timeout)
}
if requests.IsConnectionError(err) {
    // Handle connection failure (DNS, TCP, TLS)
}
```

## Additional Resources

- **Logging:** Learn how to configure logging for your requests. See [docs/logging.md](docs/logging.md).
- **Middleware:** Extend functionality with custom middleware. See [docs/middleware.md](docs/middleware.md).
- **Retry Mechanism:** Implement retry strategies for transient errors. See [docs/retry.md](docs/retry.md).
- **Stream Support:** Utilize streaming for real-time data processing. See [docs/stream.md](docs/stream.md).

## Credits

This library was inspired by and built upon the work of several other HTTP client libraries:

- [Monaco-io/request](https://github.com/monaco-io/request)
- [Go-resty/resty](https://github.com/go-resty/resty)
- [Dghubble/sling](https://github.com/dghubble/sling)
- [Henomis/restclientgo](https://github.com/henomis/restclientgo)
- [Fiber Client](https://github.com/gofiber/fiber)

## How to Contribute

Contributions to the `requests` package are welcome. If you'd like to contribute, please follow the [contribution guidelines](CONTRIBUTING.md).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
