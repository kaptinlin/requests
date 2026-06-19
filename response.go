package requests

import (
	"bufio"
	"bytes"
	"crypto/tls"
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
	"time"
)

// Response represents an HTTP response.
type Response struct {
	elapsed     time.Duration
	attempts    int
	jsonDecoder Decoder
	xmlDecoder  Decoder
	yamlDecoder Decoder
	logger      Logger
	rawResponse *http.Response
	body        []byte
}

func newResponse(
	resp *http.Response,
	snap *clientSnapshot,
) (*Response, error) {
	response := &Response{
		rawResponse: resp,
	}
	if snap != nil {
		response.jsonDecoder = snap.jsonDecoder
		response.xmlDecoder = snap.xmlDecoder
		response.yamlDecoder = snap.yamlDecoder
		response.logger = snap.logger
	}

	if err := response.handleNonStream(); err != nil {
		return nil, err
	}
	return response, nil
}

// Raw returns the underlying HTTP response for callers that need net/http details.
func (r *Response) Raw() *http.Response {
	return r.rawResponse
}

func (r *Response) handleNonStream() error {
	buf := getBuffer()
	defer putBuffer(buf)

	_, err := buf.ReadFrom(r.rawResponse.Body)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrResponseReadFailed, err)
	}
	_ = r.rawResponse.Body.Close()

	// Copy data before returning buffer to pool to prevent data race.
	// Without this, concurrent goroutines could get the same pooled buffer,
	// causing one goroutine's response data to be overwritten by another.
	r.body = bytes.Clone(buf.B)
	r.rawResponse.Body = io.NopCloser(bytes.NewReader(r.body))
	return nil
}

// StatusCode returns the HTTP status code of the response.
func (r *Response) StatusCode() int {
	return r.rawResponse.StatusCode
}

// Status returns the status string of the response (e.g., "200 OK").
func (r *Response) Status() string {
	return r.rawResponse.Status
}

// Header returns the response headers.
func (r *Response) Header() http.Header {
	return r.rawResponse.Header
}

// Cookies parses and returns the cookies set in the response.
func (r *Response) Cookies() []*http.Cookie {
	return r.rawResponse.Cookies()
}

// Location returns the URL redirected address.
func (r *Response) Location() (*url.URL, error) {
	return r.rawResponse.Location()
}

// URL returns the request URL that elicited the response.
func (r *Response) URL() *url.URL {
	return r.rawResponse.Request.URL
}

// Elapsed returns the duration from request dispatch through response setup.
func (r *Response) Elapsed() time.Duration {
	return r.elapsed
}

// Attempts returns the total number of transport attempts, including the first request.
func (r *Response) Attempts() int {
	return r.attempts
}

// Protocol returns the response protocol, such as "HTTP/1.1" or "HTTP/2.0".
func (r *Response) Protocol() string {
	if r.rawResponse == nil {
		return ""
	}
	return r.rawResponse.Proto
}

// TLS returns a copy of the response TLS connection state, if any.
func (r *Response) TLS() *tls.ConnectionState {
	if r.rawResponse == nil || r.rawResponse.TLS == nil {
		return nil
	}
	state := new(*r.rawResponse.TLS)
	state.PeerCertificates = slices.Clone(state.PeerCertificates)
	state.VerifiedChains = slices.Clone(state.VerifiedChains)
	for i, chain := range state.VerifiedChains {
		state.VerifiedChains[i] = slices.Clone(chain)
	}
	state.SignedCertificateTimestamps = cloneByteSlices(state.SignedCertificateTimestamps)
	state.OCSPResponse = slices.Clone(state.OCSPResponse)
	state.TLSUnique = slices.Clone(state.TLSUnique)
	return state
}

func cloneByteSlices(values [][]byte) [][]byte {
	if values == nil {
		return nil
	}
	cloned := make([][]byte, len(values))
	for i, value := range values {
		cloned[i] = slices.Clone(value)
	}
	return cloned
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
	return len(r.body)
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
	return r.body
}

// String returns the response body as a string.
func (r *Response) String() string {
	return string(r.body)
}

// Decode decodes the response body based on its content type.
func (r *Response) Decode(v any) error {
	switch {
	case r.IsJSON():
		return r.DecodeJSON(v)
	case r.IsXML():
		return r.DecodeXML(v)
	case r.IsYAML():
		return r.DecodeYAML(v)
	}

	return fmt.Errorf("%w: %s", ErrUnsupportedContentType, r.ContentType())
}

// DecodeJSON decodes the response body as JSON.
func (r *Response) DecodeJSON(v any) error {
	return r.decodeWith(r.jsonDecoder, DefaultJSONDecoder, v)
}

// DecodeXML decodes the response body as XML.
func (r *Response) DecodeXML(v any) error {
	return r.decodeWith(r.xmlDecoder, DefaultXMLDecoder, v)
}

// DecodeYAML decodes the response body as YAML.
func (r *Response) DecodeYAML(v any) error {
	return r.decodeWith(r.yamlDecoder, DefaultYAMLDecoder, v)
}

func (r *Response) decodeWith(decoder, fallback Decoder, v any) error {
	if r.body == nil {
		return nil
	}
	if decoder == nil {
		decoder = fallback
	}
	return decoder.Decode(bytes.NewReader(r.body), v)
}

const dirPermissions = 0o750

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
		if err = os.MkdirAll(dir, dirPermissions); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	outFile, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			if r.logger != nil {
				r.logger.Errorf("failed to close file: %v", err)
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
	return nil
}

// Lines returns an iterator over the buffered response body lines.
func (r *Response) Lines() iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		if r.body == nil {
			return
		}

		scanner := bufio.NewScanner(bytes.NewReader(r.body))
		for scanner.Scan() {
			if !yield(scanner.Bytes()) {
				break
			}
		}
	}
}

// Close closes the response body.
func (r *Response) Close() error {
	return r.rawResponse.Body.Close()
}
