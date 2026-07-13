package requests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/kaptinlin/orderedobject"
)

type requestBodyKind uint8

const (
	requestBodyNone requestBodyKind = iota
	requestBodyJSON
	requestBodyXML
	requestBodyYAML
	requestBodyText
	requestBodyBytes
	requestBodyReader
	requestBodyForm
	requestBodyMultipart
)

type requestBodySelection struct {
	kind                 requestBodyKind
	value                any
	form                 url.Values
	multipart            *Multipart
	contentType          string
	generatedContentType bool
}

type preparedRequestBody struct {
	body          io.Reader
	getBody       func() (io.ReadCloser, error)
	contentLength int64
	contentType   string
}

// RequestBuilder facilitates building and executing HTTP requests.
type RequestBuilder struct {
	client         *Client
	method         string
	path           string
	headers        *http.Header
	orderedHeaders *orderedobject.Object[[]string]
	cookies        []*http.Cookie
	queries        url.Values
	pathParams     map[string]string
	body           requestBodySelection
	timeout        time.Duration
	middlewares    []Middleware
	retryPolicy    RetryPolicy
	hasRetryPolicy bool
	auth           AuthMethod
	preparationErr error
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
	if err := validateMiddlewares(middlewares); err != nil {
		b.setPreparationError(err)
		return
	}
	b.middlewares = append(b.middlewares, middlewares...)
}

func (b *RequestBuilder) setPreparationError(err error) {
	if err != nil && b.preparationErr == nil {
		b.preparationErr = err
	}
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

func resolveRequestURL(baseURL, requestPath string, queryValues url.Values) (*url.URL, error) {
	requestURL, err := url.Parse(requestPath)
	if err != nil {
		return nil, err
	}
	if requestURL.IsAbs() || baseURL == "" {
		addQueryValues(requestURL, queryValues)
		return requestURL, nil
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	resolved := *base
	resolved.RawQuery = requestURL.RawQuery
	resolved.ForceQuery = requestURL.ForceQuery
	resolved.Fragment = requestURL.Fragment

	path, rawPath, err := joinEscapedURLPath(base.EscapedPath(), requestURL.EscapedPath())
	if err != nil {
		return nil, err
	}
	resolved.Path = path
	resolved.RawPath = rawPath
	addQueryValues(&resolved, queryValues)
	return &resolved, nil
}

func joinEscapedURLPath(basePath, requestPath string) (string, string, error) {
	escaped := basePath
	if requestPath != "" {
		switch escaped {
		case "", "/":
			escaped = "/" + strings.TrimLeft(requestPath, "/")
		default:
			escaped = strings.TrimRight(escaped, "/") + "/" + strings.TrimLeft(requestPath, "/")
		}
	}
	path, err := url.PathUnescape(escaped)
	if err != nil {
		return "", "", err
	}
	if path == escaped {
		return path, "", nil
	}
	return path, escaped, nil
}

func addQueryValues(requestURL *url.URL, queryValues url.Values) {
	if len(queryValues) == 0 {
		return
	}
	query := requestURL.Query()
	for key, values := range queryValues {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	requestURL.RawQuery = query.Encode()
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
		b.setPreparationError(err)
		if b.client.logger != nil {
			b.client.logger.Errorf("Error encoding query struct: %v", err)
		}
		return b
	}
	return b.Queries(values)
}

// Headers set headers to the request.
func (b *RequestBuilder) Headers(headers http.Header) *RequestBuilder {
	for key, values := range headers {
		b.headers.Del(key)
		for _, value := range values {
			b.headers.Add(key, value)
		}
		if b.orderedHeaders != nil {
			setOrderedHeaderValues(&b.orderedHeaders, key, values)
		}
		if strings.EqualFold(key, "Content-Type") {
			b.body.generatedContentType = false
		}
	}
	return b
}

// OrderedHeaders sets ordered headers for the request.
func (b *RequestBuilder) OrderedHeaders(headers *orderedobject.Object[[]string]) *RequestBuilder {
	b.body.generatedContentType = false
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
	b.setHeader(key, value)
	if strings.EqualFold(key, "Content-Type") {
		b.body.generatedContentType = false
	}
	return b
}

func (b *RequestBuilder) setHeader(key, value string) {
	b.headers.Set(key, value)
	if b.orderedHeaders != nil {
		setOrderedHeaderValues(&b.orderedHeaders, key, []string{value})
	}
}

// AddHeader adds a header to the request.
func (b *RequestBuilder) AddHeader(key, value string) *RequestBuilder {
	b.headers.Add(key, value)
	if b.orderedHeaders != nil {
		addOrderedHeaderValue(&b.orderedHeaders, key, value)
	}
	if strings.EqualFold(key, "Content-Type") {
		b.body.generatedContentType = false
	}
	return b
}

// DelHeader removes one or more headers from the request.
func (b *RequestBuilder) DelHeader(key ...string) *RequestBuilder {
	b.delHeader(key...)
	for _, k := range key {
		if strings.EqualFold(k, "Content-Type") {
			b.body.generatedContentType = false
		}
	}
	return b
}

func (b *RequestBuilder) delHeader(key ...string) {
	for _, k := range key {
		b.headers.Del(k)
		if b.orderedHeaders != nil {
			deleteOrderedHeader(b.orderedHeaders, k)
		}
	}
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
	b.cookies = append(b.cookies, &http.Cookie{Name: key, Value: value}) //nolint:gosec // callers control request cookie attributes
	return b
}

// DelCookie removes one or more cookies from the request.
func (b *RequestBuilder) DelCookie(key ...string) *RequestBuilder {
	if b.cookies == nil || len(key) == 0 {
		return b
	}

	deleteKeys := stringSet(key)
	b.cookies = slices.DeleteFunc(b.cookies, func(cookie *http.Cookie) bool {
		_, ok := deleteKeys[cookie.Name]
		return ok
	})

	return b
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
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
	if err := validateAuthOption(auth); err != nil {
		b.setPreparationError(err)
		return b
	}
	b.auth = auth
	return b
}

// Form sets URL-encoded form fields from a struct, map, or url.Values.
// The resulting body is buffered and is safe to replay for retries.
func (b *RequestBuilder) Form(v any) *RequestBuilder {
	formFields, err := parseFormFields(v)

	if err != nil {
		b.setPreparationError(err)
		if b.client.logger != nil {
			b.client.logger.Errorf("Error parsing form: %v", err)
		}
		return b
	}

	if formFields == nil {
		formFields = url.Values{}
	}
	b.selectBody(requestBodySelection{
		kind:                 requestBodyForm,
		form:                 formFields,
		contentType:          "application/x-www-form-urlencoded",
		generatedContentType: true,
	})

	return b
}

// FormFields sets multiple form fields at once.
// The resulting body is buffered and is safe to replay for retries.
func (b *RequestBuilder) FormFields(fields any) *RequestBuilder {
	values, err := parseFormFields(fields)
	if err != nil {
		b.setPreparationError(err)
		if b.client.logger != nil {
			b.client.logger.Errorf("Error parsing form fields: %v", err)
		}
		return b
	}
	formFields := b.activateForm()

	for key, value := range values {
		for _, v := range value {
			formFields.Add(key, v)
		}
	}
	return b
}

// FormField adds or updates a form field.
// Without files, the resulting form body is buffered and safe to replay for retries.
func (b *RequestBuilder) FormField(key, val string) *RequestBuilder {
	b.activateForm().Add(key, val)
	return b
}

func (b *RequestBuilder) activateForm() url.Values {
	if b.body.kind != requestBodyForm {
		b.selectBody(requestBodySelection{
			kind:                 requestBodyForm,
			form:                 url.Values{},
			contentType:          "application/x-www-form-urlencoded",
			generatedContentType: true,
		})
	}
	return b.body.form
}

// DelFormField removes one or more form fields.
func (b *RequestBuilder) DelFormField(key ...string) *RequestBuilder {
	if b.body.kind == requestBodyForm {
		for _, k := range key {
			b.body.form.Del(k)
		}
	}
	return b
}

// Multipart sets a multipart/form-data body built by [Multipart].
//
// By default the body is streamed once via an [io.Pipe] and is not replayable;
// a retry that needs to resend the body returns [ErrRequestBodyNotReplayable].
// Call m.Replayable(maxBytes) before passing the builder if retries must
// or 307/308 redirects may resend the body.
func (b *RequestBuilder) Multipart(m *Multipart) *RequestBuilder {
	if m == nil {
		b.setPreparationError(fmt.Errorf("%w: multipart body", ErrInvalidConfigValue))
		return b
	}
	b.selectBody(requestBodySelection{
		kind:                 requestBodyMultipart,
		multipart:            m,
		generatedContentType: true,
	})
	return b
}

// JSON sets the request body as JSON and Content-Type to application/json.
// The encoded body is buffered and is safe to replay for retries.
func (b *RequestBuilder) JSON(v any) *RequestBuilder {
	b.selectBody(requestBodySelection{
		kind:                 requestBodyJSON,
		value:                v,
		contentType:          "application/json",
		generatedContentType: true,
	})
	return b
}

// XML sets the request body as XML and Content-Type to application/xml.
// The encoded body is buffered and is safe to replay for retries.
func (b *RequestBuilder) XML(v any) *RequestBuilder {
	b.selectBody(requestBodySelection{
		kind:                 requestBodyXML,
		value:                v,
		contentType:          "application/xml",
		generatedContentType: true,
	})
	return b
}

// YAML sets the request body as YAML and Content-Type to application/yaml.
// The encoded body is buffered and is safe to replay for retries.
func (b *RequestBuilder) YAML(v any) *RequestBuilder {
	b.selectBody(requestBodySelection{
		kind:                 requestBodyYAML,
		value:                v,
		contentType:          "application/yaml",
		generatedContentType: true,
	})
	return b
}

// Text sets the request body as plain text and Content-Type to text/plain.
// The body is buffered and is safe to replay for retries.
func (b *RequestBuilder) Text(v string) *RequestBuilder {
	b.selectBody(requestBodySelection{
		kind:                 requestBodyText,
		value:                v,
		contentType:          "text/plain",
		generatedContentType: true,
	})
	return b
}

// Bytes sets the request body as raw bytes without changing Content-Type.
// The body is buffered and is safe to replay for retries.
func (b *RequestBuilder) Bytes(v []byte) *RequestBuilder {
	b.selectBody(requestBodySelection{kind: requestBodyBytes, value: v})
	return b
}

// Reader sets a one-shot raw request body and optional Content-Type.
// The body is not replayable unless r itself is seekable and sized.
func (b *RequestBuilder) Reader(r io.Reader, contentType string) *RequestBuilder {
	b.selectBody(requestBodySelection{
		kind:                 requestBodyReader,
		value:                r,
		contentType:          contentType,
		generatedContentType: contentType != "",
	})
	return b
}

func (b *RequestBuilder) selectBody(body requestBodySelection) {
	if b.body.generatedContentType {
		b.delHeader("Content-Type")
	}
	b.body = body
	if body.contentType != "" {
		b.setHeader("Content-Type", body.contentType)
	}
}

// Timeout sets the request timeout.
func (b *RequestBuilder) Timeout(timeout time.Duration) *RequestBuilder {
	b.timeout = timeout
	return b
}

// Retry sets the request-local retry policy, replacing the client policy.
func (b *RequestBuilder) Retry(policy RetryPolicy) *RequestBuilder {
	b.retryPolicy = policy
	b.hasRetryPolicy = true
	return b
}

// NoRetry disables retries for this request.
func (b *RequestBuilder) NoRetry() *RequestBuilder {
	return b.Retry(RetryPolicy{})
}

func (b *RequestBuilder) effectiveRetryPolicy(snap *clientSnapshot) RetryPolicy {
	policy := snap.retry
	if b.hasRetryPolicy {
		policy = b.retryPolicy
	}
	return policy.normalize()
}

func (b *RequestBuilder) do(ctx context.Context, req *http.Request, snap *clientSnapshot) (*http.Response, int, error) {
	attempts := 0

	finalHandler := MiddlewareHandlerFunc(func(req *http.Request) (*http.Response, error) {
		retry := b.effectiveRetryPolicy(snap)

		var errs []error
		var resp *http.Response
		for attempt := range retry.Max + 1 {
			if attempt > 0 {
				if err := resetRequestBody(req); err != nil {
					return resp, err
				}
			}

			var err error
			attempts++
			resp, err = snap.httpClient.Do(req)

			if err != nil {
				errs = append(errs, fmt.Errorf("attempt %d/%d: %w", attempt+1, retry.Max+1, err))
			}

			shouldRetry := retry.ShouldRetry(req, resp, err)
			if !shouldRetry || attempt == retry.Max {
				if err != nil {
					if snap.logger != nil {
						snap.logger.Errorf("Error after %d attempts: %v", attempt+1, err)
					}
					if len(errs) > 1 {
						return resp, errors.Join(errs...)
					}
					return resp, err
				}
				break
			}

			if !canReplayRequestBody(req) {
				if snap.logger != nil {
					snap.logger.Warnf("request body cannot be replayed; failing retry after attempt %d", attempt+1)
				}
				if err != nil {
					return resp, errors.Join(err, ErrRequestBodyNotReplayable)
				}
				return resp, ErrRequestBodyNotReplayable
			}

			if resp != nil {
				drainAndCloseBody(resp.Body, snap.logger)
			}

			if snap.logger != nil {
				snap.logger.Infof("Retrying request (attempt %d) after backoff", attempt+1)
			}

			delay := retry.delay(attempt, resp)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				if snap.logger != nil {
					snap.logger.Errorf("Request canceled or timed out: %v", ctx.Err())
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
	for _, mw := range slices.Backward(snap.middlewares) {
		finalHandler = mw(finalHandler)
	}

	resp, err := finalHandler(req)
	return resp, attempts, err
}

const maxRetryDrainBytes = 64 << 10

func drainAndCloseBody(body io.ReadCloser, logger Logger) {
	if body == nil {
		return
	}
	if _, err := io.Copy(io.Discard, io.LimitReader(body, maxRetryDrainBytes)); err != nil && logger != nil {
		logger.Errorf("Error draining response body: %v", err)
	}
	if err := body.Close(); err != nil && logger != nil {
		logger.Errorf("Error closing response body: %v", err)
	}
}

// Send executes the HTTP request.
//
// Invalid fluent preparation input is returned here before client snapshot,
// body preparation, middleware, or transport dispatch.
//
// Send takes a snapshot of the client at call time; later mutations on the
// client do not affect this in-flight request.
//
// Cancellation: ctx propagates through dial, TLS handshake, request header
// read, body read, retry backoff, and stream callbacks. When ctx is canceled
// before the response arrives, Send returns ctx.Err() and any partial response
// is closed internally. On success, Send fully reads and closes the transport
// body before returning the buffered Response; caller cleanup is not required
// for connection reuse.
//
// Retries: if the request body cannot be replayed, retries that would need to
// resend the body return [ErrRequestBodyNotReplayable] instead of silently
// re-sending or silently skipping.
func (b *RequestBuilder) Send(ctx context.Context) (*Response, error) {
	req, snap, cancel, start, err := b.prepareRequest(ctx)
	if err != nil {
		return nil, err
	}
	if cancel != nil {
		defer cancel()
	}

	resp, attempts, err := b.do(req.Context(), req, &snap)
	if err != nil {
		if snap.logger != nil {
			snap.logger.Errorf("Error executing request: %v", err)
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil, err
	}

	if resp == nil {
		if snap.logger != nil {
			snap.logger.Errorf("Response is nil")
		}
		return nil, ErrResponseNil
	}

	response, err := newResponse(resp, &snap)
	if response != nil {
		response.elapsed = time.Since(start)
		response.attempts = attempts
	}
	return response, err
}

// SendStream sends the request and returns an unbuffered streaming response.
// Invalid fluent preparation input is returned before any body or transport work.
func (b *RequestBuilder) SendStream(ctx context.Context) (*StreamResponse, error) {
	req, snap, cancel, start, err := b.prepareRequest(ctx)
	if err != nil {
		return nil, err
	}

	resp, attempts, err := b.do(req.Context(), req, &snap)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		if snap.logger != nil {
			snap.logger.Errorf("Error executing request: %v", err)
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil, err
	}

	if resp == nil {
		if cancel != nil {
			cancel()
		}
		if snap.logger != nil {
			snap.logger.Errorf("Response is nil")
		}
		return nil, ErrResponseNil
	}

	response := newStreamResponse(resp, cancel)
	response.elapsed = time.Since(start)
	response.attempts = attempts
	return response, nil
}

func (b *RequestBuilder) prepareRequest(ctx context.Context) (*http.Request, clientSnapshot, context.CancelFunc, time.Time, error) {
	start := time.Now()
	if b.preparationErr != nil {
		return nil, clientSnapshot{}, nil, start, b.preparationErr
	}
	snap := b.client.snapshot()
	var cancel context.CancelFunc
	cancelOnError := func() {
		if cancel != nil {
			cancel()
		}
	}

	parsedURL, err := resolveRequestURL(snap.baseURL, b.preparePath(), b.queries)
	if err != nil {
		if snap.logger != nil {
			snap.logger.Errorf("Error parsing URL: %v", err)
		}
		return nil, snap, nil, start, err
	}

	if _, err := http.NewRequestWithContext(ctx, b.method, parsedURL.String(), nil); err != nil {
		if snap.logger != nil {
			snap.logger.Errorf("Error creating request: %v", err)
		}
		return nil, snap, nil, start, fmt.Errorf("%w: %w", ErrRequestCreationFailed, err)
	}

	ctx, cancel = b.prepareContext(ctx)

	preparedBody, err := b.prepareBody(&snap)
	if err != nil {
		cancelOnError()
		if snap.logger != nil {
			snap.logger.Errorf("Error preparing request body: %v", err)
		}
		return nil, snap, nil, start, err
	}

	if preparedBody.contentType != "" {
		b.setHeader("Content-Type", preparedBody.contentType)
	}

	req, err := http.NewRequestWithContext(ctx, b.method, parsedURL.String(), preparedBody.body)
	if err != nil {
		cancelOnError()
		if snap.logger != nil {
			snap.logger.Errorf("Error creating request: %v", err)
		}
		return nil, snap, nil, start, fmt.Errorf("%w: %w", ErrRequestCreationFailed, err)
	}
	if preparedBody.getBody != nil {
		req.GetBody = preparedBody.getBody
		req.ContentLength = preparedBody.contentLength
	}

	b.applyAuthAndHeaders(req, &snap)
	orderedHeaders := b.effectiveOrderedHeaders(&snap)
	syncOrderedHeaderValues(orderedHeaders, req.Header)
	req = withOrderedHeaders(req, orderedHeaders)

	return req, snap, cancel, start, nil
}

func (b *RequestBuilder) prepareBody(snap *clientSnapshot) (preparedRequestBody, error) {
	contentType := b.headers.Get("Content-Type")
	switch b.body.kind {
	case requestBodyNone:
		return preparedRequestBody{}, nil
	case requestBodyJSON:
		return b.prepareEncodedBody(contentType, snap.jsonEncoder.Encode)
	case requestBodyXML:
		return b.prepareEncodedBody(contentType, snap.xmlEncoder.Encode)
	case requestBodyYAML:
		return b.prepareEncodedBody(contentType, snap.yamlEncoder.Encode)
	case requestBodyText:
		return replayableRequestBody([]byte(b.body.value.(string)), contentType), nil
	case requestBodyBytes:
		return replayableRequestBody(b.body.value.([]byte), contentType), nil
	case requestBodyReader:
		body, err := encodeRawBody(b.body.value)
		if err != nil {
			return preparedRequestBody{}, err
		}
		return prepareReaderBody(body, contentType)
	case requestBodyForm:
		return replayableRequestBody([]byte(b.body.form.Encode()), contentType), nil
	case requestBodyMultipart:
		body, generatedContentType, err := b.body.multipart.reader()
		if err != nil {
			return preparedRequestBody{}, err
		}
		if !b.body.generatedContentType {
			generatedContentType = ""
		}
		if !b.body.multipart.canReplay {
			return preparedRequestBody{body: body, contentType: generatedContentType}, nil
		}
		data, err := io.ReadAll(body)
		if err != nil {
			return preparedRequestBody{}, fmt.Errorf("read replayable multipart body: %w", err)
		}
		return replayableRequestBody(data, generatedContentType), nil
	default:
		return preparedRequestBody{}, fmt.Errorf("%w: unknown body selection", ErrInvalidConfigValue)
	}
}

func (b *RequestBuilder) prepareEncodedBody(
	contentType string,
	encode func(any) (io.Reader, error),
) (preparedRequestBody, error) {
	if contentType == "" {
		return preparedRequestBody{}, fmt.Errorf("%w: missing Content-Type", ErrUnsupportedContentType)
	}
	body, err := encode(b.body.value)
	if err != nil {
		return preparedRequestBody{}, err
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return preparedRequestBody{}, fmt.Errorf("read encoded request body: %w", err)
	}
	return replayableRequestBody(data, contentType), nil
}

func prepareReaderBody(body io.Reader, contentType string) (preparedRequestBody, error) {
	prepared := preparedRequestBody{body: body, contentType: contentType}
	data, ok, err := snapshotReaderBody(body)
	if err != nil {
		return preparedRequestBody{}, err
	}
	if !ok {
		return prepared, nil
	}

	prepared.getBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	prepared.contentLength = int64(len(data))
	return prepared, nil
}

func replayableRequestBody(data []byte, contentType string) preparedRequestBody {
	data = bytes.Clone(data)
	return preparedRequestBody{
		body:          bytes.NewReader(data),
		contentLength: int64(len(data)),
		contentType:   contentType,
		getBody: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}
}

type sizedReadSeekerAt interface {
	ReadAt([]byte, int64) (int, error)
	Seek(int64, int) (int64, error)
	Size() int64
}

func snapshotReaderBody(body io.Reader) ([]byte, bool, error) {
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
	offset, err := reader.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("read replayable request body position: %w", err)
	}
	size := reader.Size()
	if offset < 0 || offset > size {
		return nil, fmt.Errorf("%w: request body offset %d outside size %d", ErrRequestBodyReadIncomplete, offset, size)
	}
	data := make([]byte, size-offset)
	n, err := reader.ReadAt(data, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read replayable request body: %w", err)
	}
	if n != len(data) {
		return nil, fmt.Errorf("%w: read %d bytes, want %d", ErrRequestBodyReadIncomplete, n, len(data))
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

func (b *RequestBuilder) applyAuthAndHeaders(req *http.Request, snap *clientSnapshot) {
	addHeaderValues(req.Header, snap.headers, snap.orderedHeaders)
	for _, cookie := range snap.cookies {
		if cookie != nil {
			req.AddCookie(cookie)
		}
	}
	clientCookies := req.Cookies()
	if snap.auth != nil {
		snap.auth.Apply(req)
	}
	var requestCookies []*http.Cookie
	if b.headers != nil {
		overlayHeaderValues(req.Header, *b.headers, b.orderedHeaders)
		requestCookies = headerCookies(*b.headers)
	}
	requestCookies = append(requestCookies, b.cookies...)
	applyCookiePrecedence(req, clientCookies, requestCookies)
	if b.auth != nil {
		b.auth.Apply(req)
	}
}

func (b *RequestBuilder) effectiveOrderedHeaders(snap *clientSnapshot) *orderedobject.Object[[]string] {
	headers := mergeOrderedHeaders(snap.orderedHeaders, b.orderedHeaders)
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

func encodeRawBody(value any) (io.Reader, error) {
	switch data := value.(type) {
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
