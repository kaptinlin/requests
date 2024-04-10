package requests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
)

// RequestBuilder facilitates building and executing HTTP requests
type RequestBuilder struct {
	client        *Client
	method        string
	path          string
	headers       *http.Header
	cookies       []*http.Cookie
	queries       url.Values
	pathParams    map[string]string
	formFields    url.Values
	formFiles     []*File
	boundary      string
	bodyData      interface{}
	timeout       time.Duration
	middlewares   []Middleware
	maxRetries    int
	retryStrategy BackoffStrategy
	retryIf       RetryIfFunc
	auth          AuthMethod
}

// NewRequestBuilder creates a new RequestBuilder with default settings
func (c *Client) NewRequestBuilder(method, path string) *RequestBuilder {
	return &RequestBuilder{
		client:  c,
		method:  method,
		path:    path,
		queries: url.Values{},
		headers: &http.Header{},
	}
}

// AddMiddleware adds a middleware to the request.
func (b *RequestBuilder) AddMiddleware(middlewares ...Middleware) {
	if b.middlewares == nil {
		b.middlewares = []Middleware{}
	}
	b.middlewares = append(b.middlewares, middlewares...)
}

// Method sets the HTTP method for the request.
func (b *RequestBuilder) Method(method string) *RequestBuilder {
	b.method = method
	return b
}

// Path sets the URL path for the request.
func (b *RequestBuilder) Path(path string) *RequestBuilder {
	b.path = path
	return b
}

// PathParams sets multiple path params fields and their values at one go in the RequestBuilder instance.
func (b *RequestBuilder) PathParams(params map[string]string) *RequestBuilder {
	if b.pathParams == nil {
		b.pathParams = map[string]string{}
	}
	for key, value := range params {
		b.pathParams[key] = value
	}
	return b
}

// PathParam sets a single path param field and its value in the RequestBuilder instance.
func (b *RequestBuilder) PathParam(key, value string) *RequestBuilder {
	if b.pathParams == nil {
		b.pathParams = map[string]string{}
	}
	b.pathParams[key] = value
	return b
}

// DelPathParam removes one or more path params fields from the RequestBuilder instance.
func (b *RequestBuilder) DelPathParam(key ...string) *RequestBuilder {
	if b.pathParams != nil {
		for _, k := range key {
			delete(b.pathParams, k)
		}
	}
	return b
}

// preparePath replaces path parameters in the URL path.
func (b *RequestBuilder) preparePath() string {
	if b.pathParams == nil {
		return b.path
	}

	preparedPath := b.path
	for key, value := range b.pathParams {
		placeholder := "{" + key + "}"
		preparedPath = strings.Replace(preparedPath, placeholder, url.PathEscape(value), -1)
	}
	return preparedPath
}

// Queries adds query parameters to the request
func (b *RequestBuilder) Queries(params url.Values) *RequestBuilder {
	for key, values := range params {
		for _, value := range values {
			b.queries.Add(key, value)
		}
	}
	return b
}

// Query adds a single query parameter to the request
func (b *RequestBuilder) Query(key, value string) *RequestBuilder {
	b.queries.Add(key, value)
	return b
}

// DelQuery removes one or more query parameters from the request.
func (b *RequestBuilder) DelQuery(key ...string) *RequestBuilder {
	for _, k := range key {
		b.queries.Del(k)
	}
	return b
}

// QueriesStruct adds query parameters to the request based on a struct tagged with url tags.
func (b *RequestBuilder) QueriesStruct(queryStruct interface{}) *RequestBuilder {
	values, _ := query.Values(queryStruct) // Safely ignore error for simplicity
	for key, value := range values {
		for _, v := range value {
			b.queries.Add(key, v)
		}
	}
	return b
}

// Headers set headers to the request
func (b *RequestBuilder) Headers(headers http.Header) *RequestBuilder {
	for key, values := range headers {
		for _, value := range values {
			b.headers.Set(key, value)
		}
	}
	return b
}

// Header sets (or replaces) a header in the request.
func (b *RequestBuilder) Header(key, value string) *RequestBuilder {
	b.headers.Set(key, value)
	return b
}

// AddHeader adds a header to the request.
func (b *RequestBuilder) AddHeader(key, value string) *RequestBuilder {
	b.headers.Add(key, value)
	return b
}

// DelHeader removes one or more headers from the request.
func (b *RequestBuilder) DelHeader(key ...string) *RequestBuilder {
	for _, k := range key {
		b.headers.Del(k)
	}
	return b
}

// Cookies method for map
func (b *RequestBuilder) Cookies(cookies map[string]string) *RequestBuilder {
	for key, value := range cookies {
		b.Cookie(key, value)
	}
	return b
}

// Cookie adds a cookie to the request.
func (b *RequestBuilder) Cookie(key, value string) *RequestBuilder {
	if b.cookies == nil {
		b.cookies = []*http.Cookie{}
	}
	b.cookies = append(b.cookies, &http.Cookie{Name: key, Value: value})
	return b
}

// DelCookie removes one or more cookies from the request.
func (b *RequestBuilder) DelCookie(key ...string) *RequestBuilder {
	if b.cookies != nil {
		for i, cookie := range b.cookies {
			if slices.Contains(key, cookie.Name) {
				b.cookies = append(b.cookies[:i], b.cookies[i+1:]...)
			}
		}
	}
	return b
}

// ContentType sets the Content-Type header for the request.
func (b *RequestBuilder) ContentType(contentType string) *RequestBuilder {
	b.headers.Set("Content-Type", contentType)
	return b
}

// Accept sets the Accept header for the request.
func (b *RequestBuilder) Accept(accept string) *RequestBuilder {
	b.headers.Set("Accept", accept)
	return b
}

// UserAgent sets the User-Agent header for the request.
func (b *RequestBuilder) UserAgent(userAgent string) *RequestBuilder {
	b.headers.Set("User-Agent", userAgent)
	return b
}

// Referer sets the Referer header for the request.
func (b *RequestBuilder) Referer(referer string) *RequestBuilder {
	b.headers.Set("Referer", referer)
	return b
}

// Auth applies an authentication method to the request.
func (b *RequestBuilder) Auth(auth AuthMethod) *RequestBuilder {
	if auth.Valid() {
		b.auth = auth
	}
	return b
}

// Form sets form fields and files for the request
func (b *RequestBuilder) Form(v any) *RequestBuilder {
	formFields, formFiles, err := parseForm(v)

	if err != nil {
		if b.client.Logger != nil {
			b.client.Logger.Errorf("Error parsing form: %v", err)
		}
		return b
	}

	if formFields != nil {
		b.formFields = formFields
	}

	if formFiles != nil {
		b.formFiles = formFiles
	}

	return b
}

// FormFields sets multiple form fields at once
func (b *RequestBuilder) FormFields(fields any) *RequestBuilder {
	if b.formFields == nil {
		b.formFields = url.Values{}
	}

	values, err := parseFormFields(fields)
	if err != nil {
		if b.client.Logger != nil {
			b.client.Logger.Errorf("Error parsing form fields: %v", err)
		}
		return b
	}

	for key, value := range values {
		for _, v := range value {
			b.formFields.Add(key, v)
		}
	}
	return b
}

// FormField adds or updates a form field
func (b *RequestBuilder) FormField(key, val string) *RequestBuilder {
	if b.formFields == nil {
		b.formFields = url.Values{}
	}
	b.formFields.Add(key, val)
	return b
}

// DelFormField removes one or more form fields
func (b *RequestBuilder) DelFormField(key ...string) *RequestBuilder {
	if b.formFields != nil {
		for _, k := range key {
			b.formFields.Del(k)
		}
	}
	return b
}

// Files sets multiple files at once
func (b *RequestBuilder) Files(files ...*File) *RequestBuilder {
	if b.formFiles == nil {
		b.formFiles = []*File{}
	}

	b.formFiles = append(b.formFiles, files...)
	return b
}

// File adds a file to the request
func (b *RequestBuilder) File(key, filename string, content io.ReadCloser) *RequestBuilder {
	if b.formFiles == nil {
		b.formFiles = []*File{}
	}

	b.formFiles = append(b.formFiles, &File{
		Name:     key,
		FileName: filename,
		Content:  content,
	})
	return b
}

// DelFile removes one or more files from the request
func (b *RequestBuilder) DelFile(key ...string) *RequestBuilder {
	if b.formFiles != nil {
		for i, file := range b.formFiles {
			if slices.Contains(key, file.Name) {
				b.formFiles = append(b.formFiles[:i], b.formFiles[i+1:]...)
			}
		}
	}
	return b
}

// Body sets the request body
func (b *RequestBuilder) Body(body interface{}) *RequestBuilder {
	b.bodyData = body
	return b
}

func (b *RequestBuilder) JsonBody(v interface{}) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "application/json")
	return b
}

func (b *RequestBuilder) XMLBody(v interface{}) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "application/xml")
	return b
}

func (b *RequestBuilder) YAMLBody(v interface{}) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "application/yaml")
	return b
}

func (b *RequestBuilder) TextBody(v string) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "text/plain")
	return b
}

func (b *RequestBuilder) RawBody(v []byte) *RequestBuilder {
	b.bodyData = v
	return b
}

// Timeout sets the request timeout
func (b *RequestBuilder) Timeout(timeout time.Duration) *RequestBuilder {
	b.timeout = timeout
	return b
}

// MaxRetries sets the maximum number of retry attempts
func (b *RequestBuilder) MaxRetries(maxRetries int) *RequestBuilder {
	b.maxRetries = maxRetries
	return b
}

// RetryStrategy sets the backoff strategy for retries
func (b *RequestBuilder) RetryStrategy(strategy BackoffStrategy) *RequestBuilder {
	b.retryStrategy = strategy
	return b
}

// RetryIf sets the custom retry condition function
func (b *RequestBuilder) RetryIf(retryIf RetryIfFunc) *RequestBuilder {
	b.retryIf = retryIf
	return b
}

func (b *RequestBuilder) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	finalHandler := MiddlewareHandlerFunc(func(req *http.Request) (*http.Response, error) {
		var maxRetries = b.client.MaxRetries
		if b.maxRetries > 0 {
			maxRetries = b.maxRetries
		}

		var retryStrategy = b.client.RetryStrategy
		if b.retryStrategy != nil {
			retryStrategy = b.retryStrategy
		}

		var retryIf = b.client.RetryIf
		if b.retryIf != nil {
			retryIf = b.retryIf
		}

		if maxRetries < 1 {
			return b.client.HttpClient.Do(req) // Single request, no retries
		}

		var lastErr error
		var resp *http.Response
		for attempt := 0; attempt <= maxRetries; attempt++ {
			resp, lastErr = b.client.HttpClient.Do(req)

			// Determine if a retry is needed
			shouldRetry := lastErr != nil || (resp != nil && retryIf != nil && retryIf(req, resp, lastErr))
			if !shouldRetry || attempt == maxRetries {
				if lastErr != nil {
					if b.client.Logger != nil {
						b.client.Logger.Errorf("Error after %d attempts: %v", attempt+1, lastErr)
					}
				}
				break
			}

			if resp != nil {
				resp.Body.Close() // Prevent resource leaks
			}

			// Logging retry decision
			if b.client.Logger != nil {
				b.client.Logger.Infof("Retrying request (attempt %d) after backoff", attempt+1)
			}

			// Logging context cancellation as an error condition
			select {
			case <-ctx.Done():
				if b.client.Logger != nil {
					b.client.Logger.Errorf("Request canceled or timed out: %v", ctx.Err())
				}
				return nil, ctx.Err()
			case <-time.After(retryStrategy(attempt)):
				// Backoff before retrying
			}
		}

		return resp, lastErr
	})

	if b.middlewares != nil {
		for i := len(b.middlewares) - 1; i >= 0; i-- {
			finalHandler = b.middlewares[i](finalHandler)
		}
	}

	if b.client.Middlewares != nil {
		for i := len(b.client.Middlewares) - 1; i >= 0; i-- {
			finalHandler = b.client.Middlewares[i](finalHandler)
		}
	}

	return finalHandler(req)
}

// Send executes the HTTP request.
func (b *RequestBuilder) Send(ctx context.Context) (*Response, error) {
	var body io.Reader
	var contentType string
	var err error

	// Check if the request includes files, indicating multipart/form-data encoding is required.
	if len(b.formFiles) > 0 {
		body, contentType, err = b.prepareMultipartBody()
	} else if len(b.formFields) > 0 {
		// For form fields without files, use application/x-www-form-urlencoded encoding.
		body, contentType, err = b.prepareFormFieldsBody()
	} else if b.bodyData != nil {
		// Fallback to handling as per original logic for JSON, XML, etc.
		body, contentType, err = b.prepareBodyBasedOnContentType()
	}

	if err != nil {
		if b.client.Logger != nil {
			b.client.Logger.Errorf("Error preparing request body: %v", err)
		}
		return nil, err
	}

	if contentType != "" {
		// Set the Content-Type header based on the determined contentType.
		b.headers.Set("Content-Type", contentType)
	}

	// Parse the complete URL first to handle any modifications needed.
	parsedURL, err := url.Parse(b.client.BaseURL + b.preparePath())
	if err != nil {
		if b.client.Logger != nil {
			b.client.Logger.Errorf("Error parsing URL: %v", err)
		}
		return nil, err
	}

	// Combine query parameters from both the URL and the Query method.
	query := parsedURL.Query()
	for key, values := range b.queries {
		for _, value := range values {
			query.Set(key, value) // Add new values, preserving existing ones.
		}
	}
	parsedURL.RawQuery = query.Encode()

	// Create a context with a timeout if one is not already set.
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		if b.timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, b.timeout)
			defer cancel()
		}
	}

	// Create the HTTP request with the fully prepared URL, including query parameters.
	req, err := http.NewRequestWithContext(ctx, b.method, parsedURL.String(), body)
	if err != nil {
		if b.client.Logger != nil {
			b.client.Logger.Errorf("Error creating request: %v", err)
		}
		return nil, fmt.Errorf("%w: %v", ErrRequestCreationFailed, err)
	}

	if b.auth != nil {
		b.auth.Apply(req)
	} else if b.client.auth != nil {
		b.client.auth.Apply(req)
	}

	// Set the headers from the client and the request builder.
	if b.client.Headers != nil {
		for key := range *b.client.Headers {
			values := (*b.client.Headers)[key]
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
	if b.headers != nil {
		for key := range *b.headers {
			values := (*b.headers)[key]
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// Merge cookies from the client and the request builder.
	if b.client.Cookies != nil {
		for _, cookie := range b.client.Cookies {
			req.AddCookie(cookie)
		}
	}
	if b.cookies != nil {
		for _, cookie := range b.cookies {
			req.AddCookie(cookie)
		}
	}

	// Execute the HTTP request.
	resp, err := b.do(ctx, req)
	if err != nil {
		if b.client.Logger != nil {
			b.client.Logger.Errorf("Error executing request: %v", err)
		}
		return nil, err
	}
	defer resp.Body.Close()

	// Wrap and return the response.
	return NewResponse(ctx, resp, b.client)
}

func (b *RequestBuilder) prepareMultipartBody() (io.Reader, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// if a custom boundary is set, use it
	if b.boundary != "" {
		if err := writer.SetBoundary(b.boundary); err != nil {
			return nil, "", fmt.Errorf("setting custom boundary failed: %w", err)
		}
	}

	// add form fields
	for key, vals := range b.formFields {
		for _, val := range vals {
			if err := writer.WriteField(key, val); err != nil {
				return nil, "", fmt.Errorf("writing form field failed: %w", err)
			}
		}
	}

	// add form files
	for _, file := range b.formFiles {
		// create a new multipart part for the file
		part, err := writer.CreateFormFile(file.Name, file.FileName)
		if err != nil {
			return nil, "", fmt.Errorf("creating form file failed: %w", err)
		}
		// copy the file content to the part
		if _, err = io.Copy(part, file.Content); err != nil {
			return nil, "", fmt.Errorf("copying file content failed: %w", err)
		}

		// close the file content if it's a closer
		if closer, ok := file.Content.(io.Closer); ok {
			if err = closer.Close(); err != nil {
				return nil, "", fmt.Errorf("closing file content failed: %w", err)
			}
		}
	}

	// close the multipart writer
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("closing multipart writer failed: %w", err)
	}

	return &buf, writer.FormDataContentType(), nil
}

func (b *RequestBuilder) prepareFormFieldsBody() (io.Reader, string, error) {
	// Encode formFields as URL-encoded string
	data := b.formFields.Encode()
	return strings.NewReader(data), "application/x-www-form-urlencoded", nil
}

func (b *RequestBuilder) prepareBodyBasedOnContentType() (io.Reader, string, error) {
	// Determine and prepare the body based on the specific Content-Type
	contentType := b.headers.Get("Content-Type")

	if contentType == "" && b.bodyData != nil {
		switch b.bodyData.(type) {
		case url.Values, map[string][]string, map[string]string:
			contentType = "application/x-www-form-urlencoded"
		case map[string]interface{}, []interface{}, struct{}:
			contentType = "application/json"
		case string, []byte:
			contentType = "text/plain"
		}

		// Set the inferred Content-Type
		b.headers.Set("Content-Type", contentType)
	}

	var body io.Reader
	var err error

	switch contentType {
	case "application/json":
		body, err = b.client.JSONEncoder.Encode(b.bodyData)
	case "application/xml":
		body, err = b.client.XMLEncoder.Encode(b.bodyData)
	case "application/yaml":
		body, err = b.client.YAMLEncoder.Encode(b.bodyData)
	case "application/x-www-form-urlencoded":
		body, err = DefaultFormEncoder.Encode(b.bodyData)
	case "text/plain", "application/octet-stream":
		switch data := b.bodyData.(type) {
		case string:
			body = strings.NewReader(data)
		case []byte:
			body = bytes.NewReader(data)
		default:
			err = fmt.Errorf("%w: %s", ErrUnsupportedContentType, contentType)
		}
	default:
		err = fmt.Errorf("%w: %s", ErrUnsupportedContentType, contentType)
	}

	return body, contentType, err
}
