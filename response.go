package requests

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Response represents an HTTP response.
type Response struct {
	stream      StreamCallback
	streamErr   StreamErrCallback
	streamDone  StreamDoneCallback
	RawResponse *http.Response
	BodyBytes   []byte
	Context     context.Context
	Client      *Client
}

// NewResponse creates a new wrapped response object, leveraging the buffer pool for efficient memory usage.
func NewResponse(
	ctx context.Context,
	resp *http.Response,
	client *Client,
	stream StreamCallback,
	streamErr StreamErrCallback,
	streamDone StreamDoneCallback,
) (*Response, error) {
	response := &Response{
		RawResponse: resp,
		Context:     ctx,
		BodyBytes:   nil,
		stream:      stream,
		streamErr:   streamErr,
		streamDone:  streamDone,
		Client:      client,
	}

	if response.stream != nil {
		go response.handleStream()
	} else {
		if err := response.handleNonStream(); err != nil {
			return nil, err
		}
	}

	return response, nil
}

// handleStream processes the HTTP response as a stream.
func (r *Response) handleStream() {
	defer func() {
		if err := r.RawResponse.Body.Close(); err != nil {
			if r.Client.Logger != nil {
				r.Client.Logger.Errorf("failed to close response body: %v", err)
			}
		}
	}()

	scanner := bufio.NewScanner(r.RawResponse.Body)
	scanBuf := make([]byte, 0, MaxStreamBufferSize)
	scanner.Buffer(scanBuf, MaxStreamBufferSize)

	for scanner.Scan() {
		if err := r.stream(scanner.Bytes()); err != nil {
			break
		}
	}

	if err := scanner.Err(); err != nil && r.streamErr != nil {
		r.streamErr(err)
	}

	if r.streamDone != nil {
		r.streamDone()
	}
}

// handleNonStream reads the HTTP response body into a buffer for non-streaming responses.
func (r *Response) handleNonStream() error {
	buf := GetBuffer() // Use the buffer pool
	defer PutBuffer(buf)

	_, err := buf.ReadFrom(r.RawResponse.Body)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrResponseReadFailed, err)
	}
	_ = r.RawResponse.Body.Close()

	// Copy data before returning buffer to pool to prevent data race.
	// Without this, concurrent goroutines could get the same pooled buffer,
	// causing one goroutine's response data to be overwritten by another.
	r.BodyBytes = slices.Clone(buf.B)
	r.RawResponse.Body = io.NopCloser(bytes.NewReader(r.BodyBytes))
	return nil
}

// StatusCode returns the HTTP status code of the response.
func (r *Response) StatusCode() int {
	return r.RawResponse.StatusCode
}

// Status returns the status string of the response (e.g., "200 OK").
func (r *Response) Status() string {
	return r.RawResponse.Status
}

// Header returns the response headers.
func (r *Response) Header() http.Header {
	return r.RawResponse.Header
}

// Cookies parses and returns the cookies set in the response.
func (r *Response) Cookies() []*http.Cookie {
	return r.RawResponse.Cookies()
}

// Location returns the URL redirected address.
func (r *Response) Location() (*url.URL, error) {
	return r.RawResponse.Location()
}

// URL returns the request URL that elicited the response.
func (r *Response) URL() *url.URL {
	return r.RawResponse.Request.URL
}

// ContentType returns the value of the "Content-Type" header.
func (r *Response) ContentType() string {
	return r.Header().Get("Content-Type")
}

// IsContentType checks if the response Content-Type header matches a given content type.
func (r *Response) IsContentType(contentType string) bool {
	return strings.Contains(r.ContentType(), contentType)
}

// IsJSON checks if the response Content-Type indicates JSON.
func (r *Response) IsJSON() bool {
	return r.IsContentType("application/json")
}

// IsXML checks if the response Content-Type indicates XML.
func (r *Response) IsXML() bool {
	return r.IsContentType("application/xml")
}

// IsYAML checks if the response Content-Type indicates YAML.
func (r *Response) IsYAML() bool {
	return r.IsContentType("application/yaml")
}

// ContentLength returns the length of the response body.
func (r *Response) ContentLength() int {
	if r.BodyBytes == nil {
		return 0
	}
	return len(r.BodyBytes)
}

// IsEmpty checks if the response body is empty.
func (r *Response) IsEmpty() bool {
	return r.ContentLength() == 0
}

// IsSuccess checks if the response status code indicates success (200 - 299).
func (r *Response) IsSuccess() bool {
	code := r.StatusCode()
	return code >= 200 && code <= 299
}

// IsError checks if the response status code indicates an error (>= 400).
func (r *Response) IsError() bool {
	return r.StatusCode() >= 400
}

// IsClientError checks if the response status code indicates a client error (400 - 499).
func (r *Response) IsClientError() bool {
	code := r.StatusCode()
	return code >= 400 && code < 500
}

// IsServerError checks if the response status code indicates a server error (>= 500).
func (r *Response) IsServerError() bool {
	return r.StatusCode() >= 500
}

// IsRedirect checks if the response status code indicates a redirect (300 - 399).
func (r *Response) IsRedirect() bool {
	code := r.StatusCode()
	return code >= 300 && code < 400
}

// Body returns the response body as a byte slice.
func (r *Response) Body() []byte {
	return r.BodyBytes
}

// String returns the response body as a string.
func (r *Response) String() string {
	return string(r.BodyBytes)
}

// Scan attempts to unmarshal the response body based on its content type.
func (r *Response) Scan(v any) error {
	switch {
	case r.IsJSON():
		return r.ScanJSON(v)
	case r.IsXML():
		return r.ScanXML(v)
	case r.IsYAML():
		return r.ScanYAML(v)
	}

	return fmt.Errorf("%w: %s", ErrUnsupportedContentType, r.ContentType())
}

// ScanJSON unmarshals the response body into a struct via JSON decoding.
func (r *Response) ScanJSON(v any) error {
	if r.BodyBytes == nil {
		return nil
	}
	return r.Client.JSONDecoder.Decode(bytes.NewReader(r.BodyBytes), v)
}

// ScanXML unmarshals the response body into a struct via XML decoding.
func (r *Response) ScanXML(v any) error {
	if r.BodyBytes == nil {
		return nil
	}
	return r.Client.XMLDecoder.Decode(bytes.NewReader(r.BodyBytes), v)
}

// ScanYAML unmarshals the response body into a struct via YAML decoding.
func (r *Response) ScanYAML(v any) error {
	if r.BodyBytes == nil {
		return nil
	}
	return r.Client.YAMLDecoder.Decode(bytes.NewReader(r.BodyBytes), v)
}

const DirPermissions = 0o750

// Save saves the response body to a file or io.Writer.
func (r *Response) Save(v any) error {
	switch p := v.(type) {
	case string:
		return r.saveToFile(p)
	case io.Writer:
		return r.saveToWriter(p)
	default:
		return ErrNotSupportSaveMethod
	}
}

func (r *Response) saveToFile(path string) error {
	file := filepath.Clean(path)
	dir := filepath.Dir(file)

	if _, err := os.Stat(dir); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to check directory: %w", err)
		}
		if err = os.MkdirAll(dir, DirPermissions); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	outFile, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			if r.Client.Logger != nil {
				r.Client.Logger.Errorf("failed to close file: %v", err)
			}
		}
	}()

	if _, err = io.Copy(outFile, bytes.NewReader(r.Body())); err != nil {
		return fmt.Errorf("failed to write response body to file: %w", err)
	}

	return nil
}

func (r *Response) saveToWriter(w io.Writer) error {
	if _, err := io.Copy(w, bytes.NewReader(r.Body())); err != nil {
		return fmt.Errorf("failed to write response body to io.Writer: %w", err)
	}

	if wc, ok := w.(io.WriteCloser); ok {
		if err := wc.Close(); err != nil {
			if r.Client.Logger != nil {
				r.Client.Logger.Errorf("failed to close io.Writer: %v", err)
			}
		}
	}

	return nil
}

// Lines returns an iterator that yields each line of the response body as []byte.
// This method is available in Go 1.23+ and provides a convenient way to iterate
// over response lines without loading the entire body into memory.
// The iterator will automatically handle the scanning and yield each line.
// Note: This method is designed for non-streaming responses and will return
// an empty iterator for streaming responses.
func (r *Response) Lines() iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		if r.BodyBytes == nil {
			return
		}

		scanner := bufio.NewScanner(bytes.NewReader(r.BodyBytes))
		for scanner.Scan() {
			if !yield(scanner.Bytes()) {
				break
			}
		}
	}
}

// Close closes the response body.
func (r *Response) Close() error {
	return r.RawResponse.Body.Close()
}
