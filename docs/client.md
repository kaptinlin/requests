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
   - [TLS Configuration](#tls-configuration)
4. [Advanced Features](#advanced-features)
   - [Retry Mechanism](#retry-mechanism)
   - [Proxy Configuration](#proxy-configuration)
   - [Authentication](#authentication)
   - [Using Middleware](#using-middleware)
   - [HTTP Client Customization](#http-client-customization)

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

### TLS Configuration

Custom TLS configurations can be applied for enhanced security measures, such as loading custom certificates:

```go
tlsConfig := &tls.Config{InsecureSkipVerify: true}
client.SetTLSConfig(tlsConfig)
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
