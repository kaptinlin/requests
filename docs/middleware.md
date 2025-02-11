# Middleware for HTTP Interactions

Middleware allows for the preprocessing and postprocessing of HTTP requests and responses in the Requests library. This guide covers the implementation of middleware to add functionalities such as authentication, logging, and more, with practical examples for clarity.

## Table of Contents
1. [Understanding Middleware](#understanding-middleware)
2. [Client-Level Middleware](#client-level-middleware)
3. [Request-Level Middleware](#request-level-middleware)
4. [Implementing Custom Middleware](#implementing-custom-middleware)
5. [Integrating OpenTelemetry Middleware](#integrating-opentelemetry-middleware)
6. [Built Middleware](#built-middleware)
    - [Header Middleware](#header-middleware)
    - [Cookie Middleware](#cookie-middleware)
    - [Cache Middleware](#cache-middleware)

### Understanding Middleware

Middleware functions wrap around HTTP requests, allowing pre- and post-processing of requests and responses. They can modify requests before they are sent, examine responses, and decide whether to modify them, retry the request, or take other actions.

### Client-Level Middleware

Client-level middleware is applied to all requests made by a client. It's ideal for cross-cutting concerns like logging, error handling, and metrics collection.

**Adding Middleware to a Client:**

```go
client := requests.Create(&requests.Config{BaseURL: "https://api.example.com"})
client.AddMiddleware(func(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
    return func(req *http.Request) (*http.Response, error) {
        // Pre-request manipulation
        fmt.Println("Request URL:", req.URL)

        // Proceed with the request
        resp, err := next(req)

        // Post-response manipulation
        if err == nil {
            fmt.Println("Response status:", resp.Status)
        }

        return resp, err
    }
})
```

### Request-Level Middleware

Request-level middleware applies only to individual requests. This is useful for request-specific concerns, such as request tracing or modifying the request based on dynamic context.

**Adding Middleware to a Request:**

```go
request := client.NewRequestBuilder("GET", "/path").AddMiddleware(func(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
    return func(req *http.Request) (*http.Response, error) {
        // Modify the request here
        req.Header.Add("X-Request-ID", "12345")

        // Proceed with the modified request
        return next(req)
    }
})
```

### Implementing Custom Middleware

Custom middleware can perform a variety of tasks, such as authentication, logging, and metrics. Here's a simple logging middleware example:

```go
func loggingMiddleware(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
    return func(req *http.Request) (*http.Response, error) {
        log.Printf("Requesting %s %s", req.Method, req.URL)
        resp, err := next(req)
        if err != nil {
            log.Printf("Request to %s failed: %v", req.URL, err)
        } else {
            log.Printf("Received %d response from %s", resp.StatusCode, req.URL)
        }
        return resp, err
    }
}
```

### Integrating OpenTelemetry Middleware

OpenTelemetry middleware can be used to collect tracing and metrics for your requests. Below is an example of how to set up a basic trace for an HTTP request:

**Implementing OpenTelemetry Middleware:**

```go
func openTelemetryMiddleware(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
    return func(req *http.Request) (*http.Response, error) {
        ctx, span := otel.Tracer("requests").Start(req.Context(), req.URL.Path)
        defer span.End()

        // Add trace ID to request headers if needed
        traceID := span.SpanContext().TraceID().String()
        req.Header.Set("X-Trace-ID", traceID)

        resp, err := next(req)

        // Set span attributes based on response
        if err == nil {
            span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
        } else {
            span.RecordError(err)
        }

        return resp, err
    }
}
```

By utilizing middleware, you can enhance the functionality and observability of your HTTP requests within the Requests library. Whether you're logging requests, collecting metrics with OpenTelemetry, or adding custom request headers, middleware offers a flexible solution to enrich your HTTP client's capabilities.


### Built Middleware

The Requests library provides several built-in middleware functions that can be used to enhance the functionality of your HTTP requests. Below are some examples of how to use these built-in middleware functions:

### Header Middleware

Add header middleware to modify request headers:

```go
// Create middleware with specific headers
headerMiddleware := HeaderMiddleware(http.Header{
    "User-Agent": []string{"Custom-Agent"},
    "Accept":     []string{"application/json"},
})

// Apply middleware to client
client := requests.Create(&requests.Config{
    BaseURL:     "https://api.example.com",
    Middlewares: []requests.Middleware{headerMiddleware},
})
```

### Cookie Middleware

Add cookie middleware to manage cookies:

```go
// Create middleware with specific cookies
cookieMiddleware := CookieMiddleware(http.Cookie{
    Name:  "session",
    Value: "12345",
})

// Apply middleware to client
client := requests.Create(&requests.Config{
    BaseURL:     "https://api.example.com",
    Middlewares: []requests.Middleware{cookieMiddleware},
})
```

### Cache Middleware

Add cache middleware to cache responses:

```go
// Create memory cache and logger
cache := NewMemoryCache()
logger := NewDefaultLogger(os.Stdout, LevelDebug)

// Create cache middleware
cacheMiddleware := CacheMiddleware(cache, 5*time.Second, logger)

// Create client with cache middleware
client := requests.Create(&requests.Config{
    BaseURL:     "https://api.example.com",
    Middlewares: []requests.Middleware{cacheMiddleware},
})
```


