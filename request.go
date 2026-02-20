package requests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
)

// RequestBuilder facilitates building and executing HTTP requests.
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
	bodyData      any
	timeout       time.Duration
	middlewares   []Middleware
	maxRetries    int
	retryStrategy BackoffStrategy
	retryIf       RetryIfFunc
	auth          AuthMethod
	stream        StreamCallback
	streamErr     StreamErrCallback
	streamDone    StreamDoneCallback
}

// NewRequestBuilder creates a new RequestBuilder with default settings.
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
		// Pre-allocate with expected size for better performance (Go 1.24+ Swiss Tables)
		b.pathParams = make(map[string]string, len(params))
	}
	maps.Copy(b.pathParams, params)
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
		preparedPath = strings.ReplaceAll(preparedPath, placeholder, url.PathEscape(value))
	}
	return preparedPath
}

// Queries adds query parameters to the request.
func (b *RequestBuilder) Queries(params url.Values) *RequestBuilder {
	for key, values := range params {
		for _, value := range values {
			b.queries.Add(key, value)
		}
	}
	return b
}

// Query adds a single query parameter to the request.
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
func (b *RequestBuilder) QueriesStruct(queryStruct any) *RequestBuilder {
	values, err := query.Values(queryStruct)
	if err != nil {
		if b.client.Logger != nil {
			b.client.Logger.Errorf("Error encoding query struct: %v", err)
		}
		return b
	}
	for key, value := range values {
		for _, v := range value {
			b.queries.Add(key, v)
		}
	}
	return b
}

// Headers set headers to the request.
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

// Cookies method for map.
func (b *RequestBuilder) Cookies(cookies map[string]string) *RequestBuilder {
	for key, value := range cookies {
		b.Cookie(key, value)
	}
	return b
}

// Cookie adds a cookie to the request.
func (b *RequestBuilder) Cookie(key, value string) *RequestBuilder {
	b.cookies = append(b.cookies, &http.Cookie{Name: key, Value: value})
	return b
}

// DelCookie removes one or more cookies from the request.
func (b *RequestBuilder) DelCookie(key ...string) *RequestBuilder {
	if b.cookies == nil {
		return b
	}

	b.cookies = slices.DeleteFunc(b.cookies, func(cookie *http.Cookie) bool {
		return slices.Contains(key, cookie.Name)
	})

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

// Form sets form fields and files for the request.
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

// FormFields sets multiple form fields at once.
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

// FormField adds or updates a form field.
func (b *RequestBuilder) FormField(key, val string) *RequestBuilder {
	if b.formFields == nil {
		b.formFields = url.Values{}
	}
	b.formFields.Add(key, val)
	return b
}

// DelFormField removes one or more form fields.
func (b *RequestBuilder) DelFormField(key ...string) *RequestBuilder {
	if b.formFields != nil {
		for _, k := range key {
			b.formFields.Del(k)
		}
	}
	return b
}

// Files sets multiple files at once.
func (b *RequestBuilder) Files(files ...*File) *RequestBuilder {
	b.formFiles = append(b.formFiles, files...)
	return b
}

// File adds a file to the request.
func (b *RequestBuilder) File(key, filename string, content io.ReadCloser) *RequestBuilder {
	b.formFiles = append(b.formFiles, &File{
		Name:     key,
		FileName: filename,
		Content:  content,
	})
	return b
}

// DelFile removes one or more files from the request.
func (b *RequestBuilder) DelFile(key ...string) *RequestBuilder {
	if b.formFiles == nil {
		return b
	}

	b.formFiles = slices.DeleteFunc(b.formFiles, func(file *File) bool {
		return slices.Contains(key, file.Name)
	})

	return b
}

// Body sets the request body.
func (b *RequestBuilder) Body(body any) *RequestBuilder {
	b.bodyData = body
	return b
}

// JSONBody sets the request body as JSON.
func (b *RequestBuilder) JSONBody(v any) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "application/json")
	return b
}

// XMLBody sets the request body as XML.
func (b *RequestBuilder) XMLBody(v any) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "application/xml")
	return b
}

// YAMLBody sets the request body as YAML.
func (b *RequestBuilder) YAMLBody(v any) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "application/yaml")
	return b
}

// TextBody sets the request body as plain text.
func (b *RequestBuilder) TextBody(v string) *RequestBuilder {
	b.bodyData = v
	b.headers.Set("Content-Type", "text/plain")
	return b
}

// RawBody sets the request body as raw bytes.
func (b *RequestBuilder) RawBody(v []byte) *RequestBuilder {
	b.bodyData = v
	return b
}

// Timeout sets the request timeout.
func (b *RequestBuilder) Timeout(timeout time.Duration) *RequestBuilder {
	b.timeout = timeout
	return b
}

// MaxRetries sets the maximum number of retry attempts.
func (b *RequestBuilder) MaxRetries(maxRetries int) *RequestBuilder {
	b.maxRetries = maxRetries
	return b
}

// RetryStrategy sets the backoff strategy for retries.
func (b *RequestBuilder) RetryStrategy(strategy BackoffStrategy) *RequestBuilder {
	b.retryStrategy = strategy
	return b
}

// RetryIf sets the custom retry condition function.
func (b *RequestBuilder) RetryIf(retryIf RetryIfFunc) *RequestBuilder {
	b.retryIf = retryIf
	return b
}

func (b *RequestBuilder) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	finalHandler := MiddlewareHandlerFunc(func(req *http.Request) (*http.Response, error) {
		maxRetries := b.client.MaxRetries
		if b.maxRetries > 0 {
			maxRetries = b.maxRetries
		}

		retryStrategy := b.client.RetryStrategy
		if b.retryStrategy != nil {
			retryStrategy = b.retryStrategy
		}

		retryIf := b.client.RetryIf
		if b.retryIf != nil {
			retryIf = b.retryIf
		}

		if maxRetries < 1 {
			return b.client.HTTPClient.Do(req)
		}

		var errs []error
		var resp *http.Response
		for attempt := range maxRetries + 1 {
			var err error
			resp, err = b.client.HTTPClient.Do(req)

			if err != nil {
				errs = append(errs, fmt.Errorf("attempt %d/%d: %w", attempt+1, maxRetries+1, err))
			}

			shouldRetry := err != nil || (resp != nil && retryIf != nil && retryIf(req, resp, err))
			if !shouldRetry || attempt == maxRetries {
				if err != nil {
					if b.client.Logger != nil {
						b.client.Logger.Errorf("Error after %d attempts: %v", attempt+1, err)
					}
					if len(errs) > 1 {
						return resp, errors.Join(errs...)
					}
					return resp, err
				}
				break
			}

			if resp != nil {
				if closeErr := resp.Body.Close(); closeErr != nil {
					if b.client.Logger != nil {
						b.client.Logger.Errorf("Error closing response body: %v", closeErr)
					}
				}
			}

			if b.client.Logger != nil {
				b.client.Logger.Infof("Retrying request (attempt %d) after backoff", attempt+1)
			}

			timer := time.NewTimer(retryStrategy(attempt))
			select {
			case <-ctx.Done():
				timer.Stop()
				if b.client.Logger != nil {
					b.client.Logger.Errorf("Request canceled or timed out: %v", ctx.Err())
				}
				return nil, ctx.Err()
			case <-timer.C:
			}
		}

		return resp, nil
	})

	// Wrap middlewares: request-level first (inner), then client-level (outer).
	for _, mw := range slices.Backward(b.middlewares) {
		finalHandler = mw(finalHandler)
	}
	for _, mw := range slices.Backward(b.client.Middlewares) {
		finalHandler = mw(finalHandler)
	}

	return finalHandler(req)
}

// Stream sets the stream callback for the request.
func (b *RequestBuilder) Stream(callback StreamCallback) *RequestBuilder {
	b.stream = callback
	return b
}

// StreamErr sets the error callback for the request.
func (b *RequestBuilder) StreamErr(callback StreamErrCallback) *RequestBuilder {
	b.streamErr = callback
	return b
}

// StreamDone sets the done callback for the request.
func (b *RequestBuilder) StreamDone(callback StreamDoneCallback) *RequestBuilder {
	b.streamDone = callback
	return b
}

// Clone creates a deep copy of the RequestBuilder. The clone shares the same client
// reference (shallow copy) but has independent copies of headers, cookies, queries,
// pathParams, and formFields (deep copy). This means configuration changes to the
// client will affect both the original and clone.
//
// Body data, form files, stream callbacks, middlewares, and retry config are not copied
// as they are not safe to share or clone. Set these on the cloned builder if needed.
func (b *RequestBuilder) Clone() *RequestBuilder {
	clone := &RequestBuilder{
		client:   b.client,
		method:   b.method,
		path:     b.path,
		timeout:  b.timeout,
		boundary: b.boundary,
	}

	if b.headers != nil {
		h := b.headers.Clone()
		clone.headers = &h
	}

	if b.cookies != nil {
		clone.cookies = slices.Clone(b.cookies)
	}

	if b.queries != nil {
		clone.queries = url.Values{}
		maps.Copy(clone.queries, b.queries)
	}

	if b.pathParams != nil {
		clone.pathParams = maps.Clone(b.pathParams)
	}

	if b.formFields != nil {
		clone.formFields = url.Values{}
		maps.Copy(clone.formFields, b.formFields)
	}

	return clone
}

// Send executes the HTTP request.
func (b *RequestBuilder) Send(ctx context.Context) (*Response, error) {
	body, contentType, err := b.prepareBody()
	if err != nil {
		if b.client.Logger != nil {
			b.client.Logger.Errorf("Error preparing request body: %v", err)
		}
		return nil, err
	}

	if contentType != "" {
		b.headers.Set("Content-Type", contentType)
	}

	parsedURL, err := url.Parse(b.client.BaseURL + b.preparePath())
	if err != nil {
		if b.client.Logger != nil {
			b.client.Logger.Errorf("Error parsing URL: %v", err)
		}
		return nil, err
	}

	query := parsedURL.Query()
	for key, values := range b.queries {
		for _, value := range values {
			query.Set(key, value)
		}
	}
	parsedURL.RawQuery = query.Encode()

	ctx, cancel := b.prepareContext(ctx)
	if cancel != nil {
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, b.method, parsedURL.String(), body)
	if err != nil {
		if b.client.Logger != nil {
			b.client.Logger.Errorf("Error creating request: %v", err)
		}
		return nil, fmt.Errorf("%w: %w", ErrRequestCreationFailed, err)
	}

	b.applyAuth(req)
	b.applyHeaders(req)
	b.applyCookies(req)

	resp, err := b.do(ctx, req)
	if err != nil {
		if b.client.Logger != nil {
			b.client.Logger.Errorf("Error executing request: %v", err)
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil, err
	}

	if resp == nil {
		if b.client.Logger != nil {
			b.client.Logger.Errorf("Response is nil")
		}
		return nil, fmt.Errorf("%w: %w", ErrResponseNil, err)
	}

	return NewResponse(ctx, resp, b.client, b.stream, b.streamErr, b.streamDone)
}

func (b *RequestBuilder) prepareBody() (io.Reader, string, error) {
	if len(b.formFiles) > 0 {
		return b.prepareMultipartBody()
	}
	if len(b.formFields) > 0 {
		body, contentType := b.prepareFormFieldsBody()
		return body, contentType, nil
	}
	if b.bodyData != nil {
		return b.prepareBodyBasedOnContentType()
	}
	return nil, "", nil
}

func (b *RequestBuilder) prepareContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok && b.timeout > 0 {
		return context.WithTimeout(ctx, b.timeout)
	}
	return ctx, nil
}

func (b *RequestBuilder) applyAuth(req *http.Request) {
	if b.auth != nil {
		b.auth.Apply(req)
	} else if b.client.auth != nil {
		b.client.auth.Apply(req)
	}
}

func (b *RequestBuilder) applyHeaders(req *http.Request) {
	if b.client.Headers != nil {
		for key, values := range *b.client.Headers {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
	if b.headers != nil {
		for key, values := range *b.headers {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
}

func (b *RequestBuilder) applyCookies(req *http.Request) {
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
}

func (b *RequestBuilder) prepareMultipartBody() (io.Reader, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if b.boundary != "" {
		if err := writer.SetBoundary(b.boundary); err != nil {
			return nil, "", fmt.Errorf("setting custom boundary failed: %w", err)
		}
	}

	for key, vals := range b.formFields {
		for _, val := range vals {
			if err := writer.WriteField(key, val); err != nil {
				return nil, "", fmt.Errorf("writing form field failed: %w", err)
			}
		}
	}

	for _, file := range b.formFiles {
		part, err := writer.CreateFormFile(file.Name, file.FileName)
		if err != nil {
			return nil, "", fmt.Errorf("creating form file failed: %w", err)
		}
		if _, err = io.Copy(part, file.Content); err != nil {
			return nil, "", fmt.Errorf("copying file content failed: %w", err)
		}
		if closer, ok := file.Content.(io.Closer); ok {
			if err = closer.Close(); err != nil {
				return nil, "", fmt.Errorf("closing file content failed: %w", err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("closing multipart writer failed: %w", err)
	}

	return &buf, writer.FormDataContentType(), nil
}

func (b *RequestBuilder) prepareFormFieldsBody() (io.Reader, string) {
	return strings.NewReader(b.formFields.Encode()), "application/x-www-form-urlencoded"
}

func (b *RequestBuilder) prepareBodyBasedOnContentType() (io.Reader, string, error) {
	contentType := b.headers.Get("Content-Type")

	if contentType == "" && b.bodyData != nil {
		contentType = b.inferContentType()
		b.headers.Set("Content-Type", contentType)
	}

	body, err := b.encodeBody(contentType)
	return body, contentType, err
}

func (b *RequestBuilder) inferContentType() string {
	switch b.bodyData.(type) {
	case url.Values, map[string][]string, map[string]string:
		return "application/x-www-form-urlencoded"
	case map[string]any, []any, struct{}:
		return "application/json"
	case string, []byte:
		return "text/plain"
	default:
		return ""
	}
}

func (b *RequestBuilder) encodeBody(contentType string) (io.Reader, error) {
	switch contentType {
	case "application/json":
		return b.client.JSONEncoder.Encode(b.bodyData)
	case "application/xml":
		return b.client.XMLEncoder.Encode(b.bodyData)
	case "application/yaml":
		return b.client.YAMLEncoder.Encode(b.bodyData)
	case "application/x-www-form-urlencoded":
		return DefaultFormEncoder.Encode(b.bodyData)
	case "text/plain", "application/octet-stream":
		return b.encodeRawBody()
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedContentType, contentType)
	}
}

func (b *RequestBuilder) encodeRawBody() (io.Reader, error) {
	switch data := b.bodyData.(type) {
	case string:
		return strings.NewReader(data), nil
	case []byte:
		return bytes.NewReader(data), nil
	case io.Reader:
		return data, nil
	default:
		return nil, fmt.Errorf("%w: expected string, []byte, or io.Reader", ErrUnsupportedContentType)
	}
}
