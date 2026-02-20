# The Requests Client

The Requests library in Go provides a simplified yet powerful interface for making HTTP requests. This documentation is a detailed guide to using the `Client`, including how to configure it for various needs such as handling retries, setting up proxies, authenticating requests, and using middleware for request/response processing.

## Table of Contents

1. [Introduction](#introduction)
2. [Quick Start](#quick-start)
   - [Initializing the Client](#initializing-the-client)
   - [Configuring with Set Methods](#configuring-with-set-methods)
3. [Client Configuration](#client-configuration)
   - [Configuring BaseURL](#configuring-baseurl)
   - [Setting Headers](#setting-headers)
   - [Managing Cookies](#managing-cookies)
   - [Configuring Timeouts](#configuring-timeouts)
     - [Transport-Level Timeouts](#transport-level-timeouts)
     - [Connection Pool Configuration](#connection-pool-configuration)
   - [TLS Configuration](#tls-configuration)
   - [HTTP2 Configuration](#http2-configuration)
4. [Advanced Features](#advanced-features)
   - [Retry Mechanism](#retry-mechanism)
   - [Proxy Configuration](#proxy-configuration)
   - [Authentication](#authentication)
   - [Using Middleware](#using-middleware)
   - [HTTP Client Customization](#http-client-customization)
   - [Redirection Configuration](#redirection-configuration)

## Introduction

`Client` acts as the central component in the Requests library, facilitating streamlined HTTP communication. It abstracts the `net/http` package's complexity, making HTTP requests intuitive and developer-friendly.

## Quick Start

The Requests library offers a straightforward way to make HTTP requests in Go. Here's how to get started with configuring and using the `Client`.

### Initializing the Client

You can start by creating a `Client` with specific configurations using the `Create` method:

```go
client := requests.Create(&requests.Config{
    BaseURL: "https://api.example.com",
    Timeout: 30 * time.Second,
    Headers: &http.Header{
        "Authorization": []string{"Bearer YOUR_ACCESS_TOKEN"},
        "Content-Type": []string{"application/json"},
    },
    Cookies: map[string]string{
        "session_token": "YOUR_SESSION_TOKEN",
    },
    TLSConfig: &tls.Config{
        InsecureSkipVerify: true,
    },
    MaxRetries: 3,
    RetryStrategy: requests.ExponentialBackoffStrategy(1*time.Second, 2, 30*time.Second),
    RetryIf: requests.DefaultRetryIf,
})
```

This setup creates a `Client` tailored for your API communication, including base URL, request timeout, default headers, and cookies.

### Configuring with Set Methods

Alternatively, you can use `Set` methods for a more dynamic configuration approach:

```go
client := requests.URL("https://api.example.com").
    SetDefaultHeader("Authorization", "Bearer YOUR_ACCESS_TOKEN").
    SetDefaultHeader("Content-Type", "application/json").
    SetDefaultCookie("session_token", "YOUR_SESSION_TOKEN").
    SetTLSConfig(&tls.Config{InsecureSkipVerify: true}).
    SetMaxRetries(3).
    SetRetryStrategy(requests.ExponentialBackoffStrategy(1*time.Second, 2, 30*time.Second)).
    SetRetryIf(requests.DefaultRetryIf).
    SetProxy("http://localhost:8080")
```

This method allows for incremental building of your client configuration, providing flexibility to adjust settings as needed.

These examples demonstrate two ways to configure the Requests `Client` for use in your projects, whether you prefer a comprehensive or incremental setup approach.

## Client Configuration
### Configuring BaseURL

Set the base URL for all requests:

```go
client.SetBaseURL("https://api.example.com")
```

### Setting Headers

Set default headers for all requests:

```go
client.SetDefaultHeader("Authorization", "Bearer YOUR_ACCESS_TOKEN")
client.SetDefaultHeader("Content-Type", "application/json")
```

Bulk set default headers:

```go
headers := &http.Header{
    "Authorization": []string{"Bearer YOUR_ACCESS_TOKEN"},
    "Content-Type":  []string{"application/json"},
}
client.SetDefaultHeaders(headers)
```

Add or remove a header:

```go
client.AddDefaultHeader("X-Custom-Header", "Value1")
client.DelDefaultHeader("X-Unneeded-Header")
```

### Managing Cookies

Set default cookies for all requests:

```go
client.SetDefaultCookie("session_id", "123456")
```

Bulk set default cookies:

```go
cookies := map[string]string{
    "session_id": "123456",
    "preferences": "dark_mode=true",
}
client.SetDefaultCookies(cookies)
```

Remove a default cookie:

```go
client.DelDefaultCookie("session_id")
```

This approach simplifies managing base URLs, headers, and cookies across all requests made with the client, ensuring consistency and ease of use.

### Configuring Timeouts

Define a global timeout for all requests to prevent indefinitely hanging operations:

```go
client := requests.Create(&requests.Config{
    Timeout: 15 * time.Second,
})
```

#### Transport-Level Timeouts

For fine-grained control over different phases of a request, configure transport-level timeouts:

```go
client := requests.Create(&requests.Config{
    BaseURL:               "https://api.example.com",
    Timeout:               60 * time.Second,             // overall deadline
    DialTimeout:           5 * time.Second,              // TCP connect
    TLSHandshakeTimeout:   5 * time.Second,              // TLS negotiation
    ResponseHeaderTimeout: 10 * time.Second,             // time to first byte
})
```

Or use Set methods:

```go
client.SetDialTimeout(5 * time.Second)
client.SetTLSHandshakeTimeout(5 * time.Second)
client.SetResponseHeaderTimeout(10 * time.Second)
```

#### Connection Pool Configuration

Tune the connection pool for high-throughput scenarios:

```go
client := requests.Create(&requests.Config{
    MaxIdleConns:        100, // Max idle connections across all hosts (default: 100)
    MaxIdleConnsPerHost: 10,  // Max idle connections per host (default: 2)
    MaxConnsPerHost:     50,  // Max total connections per host (default: no limit)
    IdleConnTimeout:     90 * time.Second, // Idle connection lifetime (default: 90s)
})
```

Or use Set methods:

```go
client.SetMaxIdleConns(100)
client.SetMaxIdleConnsPerHost(10)
client.SetMaxConnsPerHost(50)
client.SetIdleConnTimeout(90 * time.Second)
```

### TLS Configuration

Custom TLS configurations can be applied for enhanced security measures, such as loading custom certificates:

```go
tlsConfig := &tls.Config{InsecureSkipVerify: true}
client.SetTLSConfig(tlsConfig)
```

Load and set client certificates:

```go
client := requests.Create(&requests.Config{
    BaseURL: "https://api.example.com",
    TLSConfig: &tls.Config{
        InsecureSkipVerify: false,
    },
})
// Load client certificate
clientCert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
if err != nil {
    log.Fatal(err)
}

// Set certificates
client.SetCertificates(clientCert)
```

Configure root certificates:

```go
// From file
client.SetRootCertificate("root-cert.pem")

// From string
client.SetRootCertificateFromString
(`-----BEGIN CERTIFICATE-----
... certificate content ...
-----END CERTIFICATE-----`)

// Set client root certificate
client.SetClientRootCertificate("client-root-cert.pem")
```

Complete example with all TLS options:

```go
client := requests.Create(&requests.Config{
    BaseURL: "https://api.example.com",
    TLSConfig: &tls.Config{
        InsecureSkipVerify: false,
    },
})

// Load and set certificates
cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
if err != nil {
    log.Fatal(err)
}
client.SetCertificates(cert)

// Set root certificates
client.SetRootCertificate("root-cert.pem")
client.SetClientRootCertificate("client-root-cert.pem")
```

### HTTP/2 Configuration

Configure HTTP/2 support for the client:

```go
// Enable HTTP/2 support explicitly
client := requests.Create(&requests.Config{
    HTTP2: true,
})
```

## Advanced Features

### Retry Mechanism

Automatically retry requests on failure with customizable strategies:

```go
client.SetMaxRetries(3)
client.SetRetryStrategy(requests.ExponentialBackoffStrategy(1*time.Second, 2, 30*time.Second))
client.SetRetryIf(func(req *http.Request, resp *http.Response, err error) bool {
	// Only retry for 500 Internal Server Error
	return resp.StatusCode == http.StatusInternalServerError
})
```

For more details, visit [retry.md](./retry.md).

### Proxy Configuration

Route requests through a proxy server:

```go
client.SetProxy("http://localhost:8080")
```

#### Proxy with Bypass (NO_PROXY)

Configure a proxy with a bypass list to exclude certain hosts:

```go
// Bypass localhost, internal domains, and a CIDR range
client.SetProxyWithBypass("http://proxy:8080", "localhost, .internal.com, 10.0.0.0/8")
```

Supported bypass formats:
- Domain names: `example.com` (matches exact and subdomains)
- Leading dot domains: `.example.com` (matches subdomains only)
- IP addresses: `192.168.1.1`
- CIDR subnets: `10.0.0.0/8`
- Wildcard: `*` (bypass all)

#### Proxy from Environment

Use standard environment variables (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`):

```go
client.SetProxyFromEnv()
```

#### Proxy Rotation

Rotate through multiple proxies per-request using round-robin:

```go
client.SetProxies("http://proxy1:8080", "http://proxy2:8080", "socks5://proxy3:1080")
```

Each call to `Send()` (including retries) picks the next proxy in order. This means retries automatically hit a different proxy — no special configuration needed.

For random selection or custom logic, use `SetProxySelector`:

```go
// Random selection
selector, _ := requests.RandomProxies("http://proxy1:8080", "http://proxy2:8080")
client.SetProxySelector(selector)

// Custom: geo-routing, weighted, health-aware, etc.
client.SetProxySelector(func(req *http.Request) (*url.URL, error) {
    return pickByRegion(req.URL.Host), nil
})
```

### Authentication

Supports various authentication methods:

- **Basic Auth**:

```go
client.SetAuth(requests.BasicAuth{
  Username: "user",
  Password: "pass",
})
```

- **Bearer Token**:

```go
client.SetAuth(requests.BearerAuth{
    Token: "YOUR_ACCESS_TOKEN",
})
```

### Using Middleware

Middleware can be used for logging, enriching requests, or custom error handling. Here's an example of adding a simple logging middleware:

```go
logMiddleware := func(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
    return func(req *http.Request) (*http.Response, error) {
        log.Printf("Request URL: %s", req.URL)
        return next(req)
    }
}

client := requests.URL("https://api.example.com")
client.AddMiddleware(logMiddleware)
```

For more details, visit [middleware.md](./middleware.md).

### HTTP Client Customization

Directly customize the underlying `http.Client`:

```go
customHTTPClient := &http.Client{Timeout: 20 * time.Second}
client.SetHTTPClient(customHTTPClient)
```

### Redirection Configuration

Configure the redirect behavior of requests using various redirect policies:

```go
// Create a new client
client := requests.Create(&requests.Config{
    BaseURL: "https://api.example.com",
})

// Prohibit all redirects
client.SetRedirectPolicy(requests.NewProhibitRedirectPolicy())

// Allow up to 3 redirects
client.SetRedirectPolicy(requests.NewAllowRedirectPolicy(3))

// Allow redirects only to specified domains
client.SetRedirectPolicy(requests.NewRedirectSpecifiedDomainPolicy(
    "example.com",
    "api.example.com",
))
```

#### Smart Redirect Policy

The `SmartRedirectPolicy` provides browser-like redirect behavior:
- **301/302**: Downgrades POST to GET (per browser convention)
- **303**: Converts any method (except HEAD) to GET
- **307/308**: Preserves the original method and body
- Automatically strips sensitive headers (`Authorization`, `Cookie`, etc.) on cross-host or HTTPS-to-HTTP redirects

```go
client.SetRedirectPolicy(requests.NewSmartRedirectPolicy(10))
```
