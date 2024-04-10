# Handling HTTP Responses

Upon performing an HTTP request using the Requests library, a `Response` struct is returned, encapsulating all vital information from the HTTP response. This includes the status code, headers, and the response body.

## Table of Contents
1. [Accessing Basic Response Information](#accessing-basic-response-information)
2. [Examining Headers and Cookies](#examining-headers-and-cookies)
3. [Content Handling](#content-handling)
4. [Parsing Response Body](#parsing-response-body)
5. [Storing Response Content](#storing-response-content)
6. [Evaluating Response Success](#evaluating-response-success)

### Accessing Basic Response Information

The `Response` struct provides several methods to access basic information about the HTTP response:

- **StatusCode** and **Status**: Obtain the HTTP status code and the corresponding status message.

    ```go
    statusCode := response.StatusCode()
    status := response.Status()
    ```

- **Header**: Retrieve headers from the response.

    ```go
    contentType := response.Header().Get("Content-Type")
    ```

- **Cookies**: Access the cookies provided in the response.

    ```go
    cookies := response.Cookies()
    for _, cookie := range cookies {
        fmt.Printf("Cookie: %s Value: %s\n", cookie.Name, cookie.Value)
    }
    ```

- **Body** and **String**: Get the response body either as raw bytes or a string.

    ```go
    bodyBytes := response.Body()
    bodyString := response.String()
    ```

### Examining Headers and Cookies

Headers and cookies are essential for many HTTP interactions, and the `Response` struct simplifies their access:

- Use `Header()` for reading response headers.
- Use `Cookies()` to fetch cookies set by the server.

### Content Handling

The `Response` struct offers methods to facilitate handling of different content types:

- **ContentType**: Determine the `Content-Type` of the response.

    ```go
    if response.ContentType() == "application/json" {
        fmt.Println("Received JSON content")
    }
    ```

- **IsJSON**, **IsXML**, **IsYAML**: Check if the content type matches specific formats.

    ```go
    if response.IsJSON() {
        fmt.Println("Content is of type JSON")
    }
    ```

### Parsing Response Body

By leveraging the `Scan`, `ScanJSON`, `ScanXML`, and `ScanYAML` methods, you can effortlessly decode responses based on the `Content-Type`. 

#### JSON Responses

Given a JSON response, you can unmarshal it directly into a Go struct using either the specific `ScanJSON` method or the generic `Scan` method, which automatically detects the content type:

```go
var jsonData struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

// Unmarshal using ScanJSON
if err := response.ScanJSON(&jsonData); err != nil {
    log.Fatalf("Error unmarshaling JSON: %v", err)
}

// Alternatively, unmarshal using Scan
if err := response.Scan(&jsonData); err != nil {
    log.Fatalf("Error unmarshaling response: %v", err)
}
```

#### XML Responses

For XML responses, use `ScanXML` or `Scan` to decode into a Go struct. Here's an example assuming the response contains XML data:

```go
var xmlData struct {
    Name string `xml:"name"`
    Age  int    `xml:"age"`
}

// Unmarshal using ScanXML
if err := response.ScanXML(&xmlData); err != nil {
    log.Fatalf("Error unmarshaling XML: %v", err)
}

// Alternatively, unmarshal using Scan
if err := response.Scan(&xmlData); err != nil {
    log.Fatalf("Error unmarshaling response: %v", err)
}
```

#### YAML Responses

YAML content is similarly straightforward to handle. The `ScanYAML` or `Scan` method decodes the YAML response into the specified Go struct:

```go
var yamlData struct {
    Name string `yaml:"name"`
    Age  int    `yaml:"age"`
}

// Unmarshal using ScanYAML
if err := response.ScanYAML(&yamlData); err != nil {
    log.Fatalf("Error unmarshaling YAML: %v", err)
}

// Alternatively, unmarshal using Scan
if err := response.Scan(&yamlData); err != nil {
    log.Fatalf("Error unmarshaling response: %v", err)
}
```

### Storing Response Content

For saving the response body to a file or streaming it to an `io.Writer`:

- **Save**: Write the response body to a designated location.

    ```go
    // Save response to a file
    if err := response.Save("downloaded_file.txt"); err != nil {
        log.Fatalf("Failed to save file: %v", err)
    }
    ```

### Evaluating Response Success

To assess whether the HTTP request was successful:

- **IsSuccess**: Check if the status code signifies a successful response.

    ```go
    if response.IsSuccess() {
        fmt.Println("The request succeeded.")
    }
    ```
