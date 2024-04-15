# The Requests builder

The Request builder simplifies creating HTTP requests, providing methods to customize every aspect of the request, including headers, query parameters, and the body. It seamlessly integrates with the Client, utilizing the configurations and middleware defined at the client level.

## Table of Contents

1. [Building a Request](#building-a-request)
   - [Basic Usage](#basic-usage)
   - [Setting HTTP Method and Path](#setting-http-method-and-path)
2. [Customizing Requests](#customizing-requests)
   - [Query Parameters](#query-parameters)
   - [Headers](#headers)
   - [Cookies](#cookies)
   - [Body Content](#body-content)
   - [Timeout and Retries](#timeout-and-retries)
   - [Sending Requests](#sending-requests)
   - [Handling Cancellation](#handling-cancellation)
3. [Advanced Features](#advanced-features)
   - [Path Parameters](#path-parameters)
   - [Form Data](#form-data)
   - [File Uploads](#file-uploads)
   - [Authentication](#authentication)
   - [Middleware](#middleware)

## Building a Request

### Basic Usage

To start building a request, use the `NewRequestBuilder` method of a Client instance. Specify the HTTP method and the request path:

```go
client := requests.URL("https://api.example.com")
request := client.NewRequestBuilder("GET", "/users")
```

For convenience, the Requests library provides quick methods for common HTTP methods, allowing you to initiate requests with less boilerplate.

```go
// GET
request := client.Get("/path")

// POST
request := client.Post("/path")

// DELETE
request := client.Delete("/path")

// PUT
request := client.Put("/path")

// PATCH
request := client.Patch("/path")

// OPTIONS
request := client.Options("/path")

// HEAD
request := client.Head("/path")

// CONNECT
request := client.CONNECT("/path")

// TRACE
request := client.TRACE("/path")

// Custom Method
request := client.Custom("/path", "METHOD")
```

Each method automatically sets the HTTP method and path for your request, simplifying the initiation process.

### Setting HTTP Method and Path

You can dynamically set or change the HTTP method and URL path for the request:

```go
request.Method("POST").Path("/users/create")
```

## Customizing Requests

### Query Parameters

Add query parameters to your request using `Query`, `Queries`, `QueriesStruct`, or remove them with `DelQuery`.

```go
// Add a single query parameter
request.Query("search", "query")

// Add multiple query parameters
request.Queries(url.Values{"sort": []string{"date"}, "limit": []string{"10"}})

// Add query parameters from a struct
type queryParams struct {
    Sort  string `url:"sort"`
    Limit int    `url:"limit"`
}
request.QueriesStruct(queryParams{Sort: "date", Limit: 10})

// Remove one or more query parameters
request.DelQuery("sort", "limit")
```

### Headers

Set request headers using `Header`, `Headers`, or related methods. The library also offers convenient methods for commonly used headers, simplifying the syntax.

```go
request.Header("Authorization", "Bearer YOUR_ACCESS_TOKEN")
request.Headers(http.Header{"Content-Type": []string{"application/json"}})

// Convenient methods for common headers
request.ContentType("application/json")
request.Accept("application/json")
request.UserAgent("MyCustomClient/1.0")
request.Referer("https://example.com")
```

### Cookies

Add cookies to your request using `Cookie`, `Cookies`, or remove them with `DelCookie`. 

```go
// Add a single cookie
request.Cookie("session_token", "YOUR_SESSION_TOKEN")

// Add multiple cookies at once
request.Cookies(map[string]string{
    "session_token": "YOUR_SESSION_TOKEN",
    "user_id": "12345",
})

// Remove one or more cookies
request.DelCookie("session_token", "user_id")

```

### Body Content

Specify the request body directly with `Body` or use format-specific methods like `JSONBody`, `XMLBody`, `YAMLBody`, `TextBody`, or `RawBody` for appropriate content types.

```go
// Setting JSON body
request.JSONBody(map[string]interface{}{"key": "value"})

// Setting XML body
request.XMLBody(myXmlStruct)

// Setting YAML body
request.YAMLBody(myYamlStruct)

// Setting text body
request.TextBody("plain text content")

// Setting raw body
request.RawBody([]byte("raw data"))
```

### Timeout and Retries

Configure request-specific timeout and retry strategies:

```go
request.Timeout(10 * time.Second).MaxRetries(3)
```

### Sending Requests

The `Send(ctx)` method executes the HTTP request built with the Request builder. It requires a `context.Context` argument, allowing you to control request cancellation and timeouts.

```go
resp, err := request.Send(context.Background())
if err != nil {
    log.Fatalf("Request failed: %v", err)
}
// Process response...
```


### Handling Cancellation

To cancel a request, simply use the context's cancel function. This is particularly useful for long-running requests that you may want to abort if they take too long or if certain conditions are met.

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel() // Ensures resources are freed up after the operation completes or times out

// Cancel the request if it hasn't completed within the timeout
resp, err := request.Send(ctx)
if errors.Is(err, context.Canceled) {
    log.Println("Request was canceled")
}
```

## Advanced Features

### Path Parameters

To insert or modify path parameters in your URL, use `PathParam` for individual parameters or `PathParams` for multiple. For removal, use `DelPathParam`.

```go
// Setting a single path parameter
request.PathParam("userId", "123")

// Setting multiple path parameters at once
request.PathParams(map[string]string{"userId": "123", "postId": "456"})

// Removing path parameters
request.DelPathParam("userId", "postId")
```

When using `client.Get("/users/{userId}/posts/{postId}")`, replace `{userId}` and `{postId}` with actual values by using `PathParams` or `PathParam`.

### Form Data

For `application/x-www-form-urlencoded` content, utilize `FormField` for individual fields or `FormFields` for multiple.

```go
// Adding individual form field
request.FormField("name", "John Doe")

// Setting multiple form fields at once
fields := map[string]interface{}{"name": "John", "age": "30"}
request.FormFields(fields)
```

### File Uploads

To include files in a `multipart/form-data` request, specify each file's form field name, file name, and content using `File` or add multiple files with `Files`.

```go
// Adding a single file
file, _ := os.Open("path/to/file")
request.File("profile_picture", "filename.jpg", file)

// Adding multiple files
request.Files(file1, file2)
```

### Authentication

Apply authentication methods directly to the request:

```go
request.Auth(requests.BasicAuth{
   Username: "user", 
   Password: "pass",
})
```

### Middleware

Add custom middleware to process the request or response:

```go
request.AddMiddleware(func(next requests.MiddlewareHandlerFunc) requests.MiddlewareHandlerFunc {
    return func(req *http.Request) (*http.Response, error) {
        // Custom logic before request
        resp, err := next(req)
        // Custom logic after response
        return resp, err
    }
})
```

For more details, visit [middleware.md](./middleware.md).
