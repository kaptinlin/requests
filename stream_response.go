package requests

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"iter"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// StreamResponse is an unbuffered HTTP response whose body is owned by the caller.
type StreamResponse struct {
	elapsed     time.Duration
	attempts    int
	cancel      context.CancelFunc
	closeOnce   sync.Once
	closeErr    error
	rawResponse *http.Response
}

func newStreamResponse(
	resp *http.Response,
	cancel context.CancelFunc,
) *StreamResponse {
	return &StreamResponse{
		rawResponse: resp,
		cancel:      cancel,
	}
}

// Raw returns the underlying HTTP response for callers that need net/http details.
func (r *StreamResponse) Raw() *http.Response {
	return r.rawResponse
}

// StatusCode returns the HTTP status code of the response.
func (r *StreamResponse) StatusCode() int {
	return r.rawResponse.StatusCode
}

// Status returns the status string of the response.
func (r *StreamResponse) Status() string {
	return r.rawResponse.Status
}

// Header returns the response headers.
func (r *StreamResponse) Header() http.Header {
	return r.rawResponse.Header
}

// URL returns the request URL that elicited the response.
func (r *StreamResponse) URL() *url.URL {
	return r.rawResponse.Request.URL
}

// Elapsed returns the duration from request dispatch through response setup.
func (r *StreamResponse) Elapsed() time.Duration {
	return r.elapsed
}

// Attempts returns the total number of transport attempts, including the first request.
func (r *StreamResponse) Attempts() int {
	return r.attempts
}

// Body returns the live response body. The caller must close the response.
func (r *StreamResponse) Body() io.ReadCloser {
	return r.rawResponse.Body
}

// Lines returns an iterator over streamed response body lines.
func (r *StreamResponse) Lines() iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		scanner := bufio.NewScanner(r.rawResponse.Body)
		scanBuf := make([]byte, 0, maxStreamBufferSize)
		scanner.Buffer(scanBuf, maxStreamBufferSize)

		for scanner.Scan() {
			if !yield(bytes.Clone(scanner.Bytes()), nil) {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			yield(nil, err)
		}
	}
}

// Close closes the response body and releases the request context.
func (r *StreamResponse) Close() error {
	r.closeOnce.Do(func() {
		if r.rawResponse != nil && r.rawResponse.Body != nil {
			r.closeErr = r.rawResponse.Body.Close()
		}
		if r.cancel != nil {
			r.cancel()
		}
	})
	return r.closeErr
}
