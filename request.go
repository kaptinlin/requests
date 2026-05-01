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
	"github.com/kaptinlin/orderedobject"
)

// RequestBuilder facilitates building and executing HTTP requests.
type RequestBuilder struct {
	client                *Client
	method                string
	path                  string
	headers               *http.Header
	orderedHeaders        *orderedobject.Object[[]string]
	cookies               []*http.Cookie
	queries               url.Values
	pathParams            map[string]string
	formFields            url.Values
	formFiles             []*File
	multipart             *Multipart
	boundary              string
	bodyData              any
	rawBody               bool
	timeout               time.Duration
	middlewares           []Middleware
	maxRetries            int
	hasMaxRetriesOverride bool
	retryStrategy         BackoffStrategy
	retryIf               RetryIfFunc
	auth                  AuthMethod
	stream                StreamCallback
	streamErr             StreamErrCallback
	streamDone            StreamDoneCallback
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
	if b.pathParams == nil {
		return b
	}
	for _, k := range key {
		delete(b.pathParams, k)
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
		if b.orderedHeaders != nil {
			setOrderedHeaderValues(&b.orderedHeaders, key, values)
		}
	}
	return b
}

// OrderedHeaders sets ordered headers for the request.
func (b *RequestBuilder) OrderedHeaders(headers *orderedobject.Object[[]string]) *RequestBuilder {
	b.orderedHeaders = cloneOrderedHeaders(headers)
	if b.orderedHeaders == nil {
		b.headers = &http.Header{}
		return b
	}
	b.headers = new(headerFromOrderedHeaders(b.orderedHeaders))
	return b
}

// Header sets (or replaces) a header in the request.
func (b *RequestBuilder) Header(key, value string) *RequestBuilder {
	b.headers.Set(key, value)
	if b.orderedHeaders != nil {
		setOrderedHeaderValues(&b.orderedHeaders, key, []string{value})
	}
	return b
}

// AddHeader adds a header to the request.
func (b *RequestBuilder) AddHeader(key, value string) *RequestBuilder {
	b.headers.Add(key, value)
	if b.orderedHeaders != nil {
		addOrderedHeaderValue(&b.orderedHeaders, key, value)
	}
	return b
}

// DelHeader removes one or more headers from the request.
func (b *RequestBuilder) DelHeader(key ...string) *RequestBuilder {
	for _, k := range key {
		b.headers.Del(k)
		if b.orderedHeaders != nil {
			deleteOrderedHeader(b.orderedHeaders, k)
		}
	}
	return b
}

// Cookies adds cookies from a map.
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
	if b.cookies == nil || len(key) == 0 {
		return b
	}

	deleteKeys := make(map[string]struct{}, len(key))
	for _, name := range key {
		deleteKeys[name] = struct{}{}
	}

	b.cookies = slices.DeleteFunc(b.cookies, func(cookie *http.Cookie) bool {
		_, ok := deleteKeys[cookie.Name]
		return ok
	})

	return b
}

// ContentType sets the Content-Type header for the request.
func (b *RequestBuilder) ContentType(contentType string) *RequestBuilder {
	return b.Header("Content-Type", contentType)
}

// Accept sets the Accept header for the request.
func (b *RequestBuilder) Accept(accept string) *RequestBuilder {
	return b.Header("Accept", accept)
}

// UserAgent sets the User-Agent header for the request.
func (b *RequestBuilder) UserAgent(userAgent string) *RequestBuilder {
	return b.Header("User-Agent", userAgent)
}

// Referer sets the Referer header for the request.
func (b *RequestBuilder) Referer(referer string) *RequestBuilder {
	return b.Header("Referer", referer)
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

// Multipart sets a multipart/form-data body built by Multipart.
func (b *RequestBuilder) Multipart(m *Multipart) *RequestBuilder {
	b.multipart = m
	return b
}

// DelFile removes one or more files from the request.
func (b *RequestBuilder) DelFile(key ...string) *RequestBuilder {
	if b.formFiles == nil || len(key) == 0 {
		return b
	}

	deleteKeys := make(map[string]struct{}, len(key))
	for _, name := range key {
		deleteKeys[name] = struct{}{}
	}

	b.formFiles = slices.DeleteFunc(b.formFiles, func(file *File) bool {
		_, ok := deleteKeys[file.Name]
		return ok
	})

	return b
}

// Body sets the request body.
func (b *RequestBuilder) Body(body any) *RequestBuilder {
	b.bodyData = body
	b.rawBody = false
	return b
}

// JSONBody sets the request body as JSON.
func (b *RequestBuilder) JSONBody(v any) *RequestBuilder {
	b.bodyData = v
	b.rawBody = false
	return b.ContentType("application/json")
}

// XMLBody sets the request body as XML.
func (b *RequestBuilder) XMLBody(v any) *RequestBuilder {
	b.bodyData = v
	b.rawBody = false
	return b.ContentType("application/xml")
}

// YAMLBody sets the request body as YAML.
func (b *RequestBuilder) YAMLBody(v any) *RequestBuilder {
	b.bodyData = v
	b.rawBody = false
	return b.ContentType("application/yaml")
}

// TextBody sets the request body as plain text.
func (b *RequestBuilder) TextBody(v string) *RequestBuilder {
	b.bodyData = v
	b.rawBody = false
	return b.ContentType("text/plain")
}

// RawBody sets the request body as raw bytes.
func (b *RequestBuilder) RawBody(v []byte) *RequestBuilder {
	b.bodyData = v
	b.rawBody = true
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
	b.hasMaxRetriesOverride = true
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

func (b *RequestBuilder) do(ctx context.Context, req *http.Request, snap clientSnapshot) (*http.Response, int, error) {
	attempts := 0

	finalHandler := MiddlewareHandlerFunc(func(req *http.Request) (*http.Response, error) {
		maxRetries := b.effectiveMaxRetries(snap)

		retryStrategy := snap.RetryStrategy
		if b.retryStrategy != nil {
			retryStrategy = b.retryStrategy
		}

		retryIf := snap.RetryIf
		if b.retryIf != nil {
			retryIf = b.retryIf
		}

		var errs []error
		var resp *http.Response
		for attempt := range maxRetries + 1 {
			if attempt > 0 {
				if err := resetRequestBody(req); err != nil {
					return resp, err
				}
			}

			var err error
			attempts++
			resp, err = snap.HTTPClient.Do(req)

			if err != nil {
				errs = append(errs, fmt.Errorf("attempt %d/%d: %w", attempt+1, maxRetries+1, err))
			}

			shouldRetry := err != nil || (resp != nil && retryIf != nil && retryIf(req, resp, err))
			if !shouldRetry || attempt == maxRetries {
				if err != nil {
					if snap.Logger != nil {
						snap.Logger.Errorf("Error after %d attempts: %v", attempt+1, err)
					}
					if len(errs) > 1 {
						return resp, errors.Join(errs...)
					}
					return resp, err
				}
				break
			}

			if !canReplayRequestBody(req) {
				if snap.Logger != nil {
					snap.Logger.Warnf("request body cannot be replayed; skipping retry after attempt %d", attempt+1)
				}
				if err != nil {
					return resp, err
				}
				break
			}

			if resp != nil {
				if closeErr := resp.Body.Close(); closeErr != nil {
					if snap.Logger != nil {
						snap.Logger.Errorf("Error closing response body: %v", closeErr)
					}
				}
			}

			if snap.Logger != nil {
				snap.Logger.Infof("Retrying request (attempt %d) after backoff", attempt+1)
			}

			delay := retryAfterDelay(resp, retryStrategy(attempt))
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				if snap.Logger != nil {
					snap.Logger.Errorf("Request canceled or timed out: %v", ctx.Err())
				}
				return nil, ctx.Err()
			case <-timer.C:
			}
		}

		return resp, nil
	})

	for _, mw := range slices.Backward(b.middlewares) {
		finalHandler = mw(finalHandler)
	}
	for _, mw := range slices.Backward(snap.Middlewares) {
		finalHandler = mw(finalHandler)
	}

	resp, err := finalHandler(req)
	return resp, attempts, err
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
// Body data, multipart bodies, form files, stream callbacks, middlewares, and retry config
// are not copied as they are not safe to share or clone. Set these on the cloned builder if needed.
func (b *RequestBuilder) Clone() *RequestBuilder {
	clone := &RequestBuilder{
		client:   b.client,
		method:   b.method,
		path:     b.path,
		timeout:  b.timeout,
		boundary: b.boundary,
	}

	if b.headers != nil {
		clone.headers = new(b.headers.Clone())
	}
	clone.orderedHeaders = cloneOrderedHeaders(b.orderedHeaders)

	if b.cookies != nil {
		clone.cookies = slices.Clone(b.cookies)
	}

	if b.queries != nil {
		clone.queries = maps.Clone(b.queries)
	}

	if b.pathParams != nil {
		clone.pathParams = maps.Clone(b.pathParams)
	}

	if b.formFields != nil {
		clone.formFields = maps.Clone(b.formFields)
	}

	return clone
}

// Send executes the HTTP request.
func (b *RequestBuilder) Send(ctx context.Context) (*Response, error) {
	start := time.Now()
	snap := b.client.snapshot()

	parsedURL, err := url.Parse(snap.BaseURL + b.preparePath())
	if err != nil {
		if snap.Logger != nil {
			snap.Logger.Errorf("Error parsing URL: %v", err)
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

	body, contentType, err := b.prepareBody(snap)
	if err != nil {
		if snap.Logger != nil {
			snap.Logger.Errorf("Error preparing request body: %v", err)
		}
		return nil, err
	}

	if contentType != "" {
		b.Header("Content-Type", contentType)
	}

	body, getBody, contentLength, err := b.prepareReplayableBody(body, snap)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, b.method, parsedURL.String(), body)
	if err != nil {
		if snap.Logger != nil {
			snap.Logger.Errorf("Error creating request: %v", err)
		}
		return nil, fmt.Errorf("%w: %w", ErrRequestCreationFailed, err)
	}
	if getBody != nil {
		req.GetBody = getBody
		req.ContentLength = contentLength
	}

	b.applyAuth(req, snap)
	b.applyHeaders(req, snap)
	b.applyCookies(req, snap)
	req = withOrderedHeaders(req, b.effectiveOrderedHeaders(snap))

	resp, attempts, err := b.do(ctx, req, snap)
	if err != nil {
		if snap.Logger != nil {
			snap.Logger.Errorf("Error executing request: %v", err)
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil, err
	}

	if resp == nil {
		if snap.Logger != nil {
			snap.Logger.Errorf("Response is nil")
		}
		return nil, ErrResponseNil
	}

	response, err := NewResponse(ctx, resp, b.client, b.stream, b.streamErr, b.streamDone)
	if response != nil {
		response.elapsed = time.Since(start)
		response.attempts = attempts
	}
	return response, err
}

func (b *RequestBuilder) prepareBody(snap clientSnapshot) (io.Reader, string, error) {
	if b.multipart != nil {
		return b.multipart.reader()
	}
	if len(b.formFiles) > 0 {
		return b.prepareMultipartBody()
	}
	if len(b.formFields) > 0 {
		return strings.NewReader(b.formFields.Encode()), "application/x-www-form-urlencoded", nil
	}
	if b.bodyData == nil {
		return nil, "", nil
	}

	contentType := b.headers.Get("Content-Type")
	if contentType == "" {
		contentType = b.inferContentType()
		b.Header("Content-Type", contentType)
	}

	body, err := b.encodeBody(contentType, snap)
	return body, contentType, err
}

func (b *RequestBuilder) effectiveMaxRetries(snap clientSnapshot) int {
	maxRetries := snap.MaxRetries
	if b.hasMaxRetriesOverride {
		maxRetries = b.maxRetries
	}
	return max(maxRetries, 0)
}

func (b *RequestBuilder) prepareReplayableBody(
	body io.Reader,
	snap clientSnapshot,
) (io.Reader, func() (io.ReadCloser, error), int64, error) {
	if body == nil || b.effectiveMaxRetries(snap) == 0 {
		return body, nil, 0, nil
	}

	data, ok, err := snapshotBody(body)
	if err != nil {
		return nil, nil, 0, err
	}
	if !ok {
		return body, nil, 0, nil
	}
	if closer, ok := body.(io.Closer); ok {
		_ = closer.Close()
	}

	getBody := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	return bytes.NewReader(data), getBody, int64(len(data)), nil
}

type sizedReadSeekerAt interface {
	ReadAt([]byte, int64) (int, error)
	Seek(int64, int) (int64, error)
	Size() int64
}

func snapshotBody(body io.Reader) ([]byte, bool, error) {
	switch reader := body.(type) {
	case *bytes.Buffer:
		return bytes.Clone(reader.Bytes()), true, nil
	case sizedReadSeekerAt:
		data, err := readSizedReaderAt(reader)
		return data, true, err
	default:
		return nil, false, nil
	}
}

func readSizedReaderAt(reader sizedReadSeekerAt) ([]byte, error) {
	data := make([]byte, reader.Size())
	n, err := reader.ReadAt(data, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read replayable request body: %w", err)
	}
	if n != len(data) {
		return nil, fmt.Errorf("%w: read %d bytes, want %d", ErrRequestBodyReadIncomplete, n, len(data))
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("reset replayable request body: %w", err)
	}
	return data, nil
}

func canReplayRequestBody(req *http.Request) bool {
	return req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
}

func resetRequestBody(req *http.Request) error {
	if req.Body == nil || req.Body == http.NoBody {
		return nil
	}
	if req.GetBody == nil {
		return ErrRequestBodyNotReplayable
	}
	body, err := req.GetBody()
	if err != nil {
		return fmt.Errorf("reset request body: %w", err)
	}
	req.Body = body
	return nil
}

func (b *RequestBuilder) prepareContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok && b.timeout > 0 {
		return context.WithTimeout(ctx, b.timeout)
	}
	return ctx, nil
}

func (b *RequestBuilder) applyAuth(req *http.Request, snap clientSnapshot) {
	if b.auth != nil {
		b.auth.Apply(req)
		return
	}
	if snap.auth != nil {
		snap.auth.Apply(req)
	}
}

func (b *RequestBuilder) applyHeaders(req *http.Request, snap clientSnapshot) {
	addHeaderValues(req.Header, snap.Headers, snap.OrderedHeaders)
	if b.headers != nil {
		overlayHeaderValues(req.Header, *b.headers, b.orderedHeaders)
	}
}

func (b *RequestBuilder) effectiveOrderedHeaders(snap clientSnapshot) *orderedobject.Object[[]string] {
	headers := mergeOrderedHeaders(snap.OrderedHeaders, b.orderedHeaders)
	if headers == nil || b.headers == nil {
		return headers
	}
	for key := range *b.headers {
		if _, ok := orderedHeaderKey(b.orderedHeaders, key); ok {
			continue
		}
		deleteOrderedHeader(headers, key)
	}
	if headers.Len() == 0 {
		return nil
	}
	return headers
}

func (b *RequestBuilder) applyCookies(req *http.Request, snap clientSnapshot) {
	for _, cookie := range snap.Cookies {
		req.AddCookie(cookie)
	}
	for _, cookie := range b.cookies {
		req.AddCookie(cookie)
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
		if err = file.Content.Close(); err != nil {
			return nil, "", fmt.Errorf("closing file content failed: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("closing multipart writer failed: %w", err)
	}

	return &buf, writer.FormDataContentType(), nil
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

func (b *RequestBuilder) encodeBody(contentType string, snap clientSnapshot) (io.Reader, error) {
	if b.rawBody {
		return b.encodeRawBody()
	}

	switch contentType {
	case "application/json":
		return snap.JSONEncoder.Encode(b.bodyData)
	case "application/xml":
		return snap.XMLEncoder.Encode(b.bodyData)
	case "application/yaml":
		return snap.YAMLEncoder.Encode(b.bodyData)
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
