package requests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Response struct {
	RawResponse *http.Response
	BodyBytes   []byte
	Context     context.Context
	Client      *Client
}

// NewResponse creates a new wrapped response object, leveraging the buffer pool for efficient memory usage.
func NewResponse(ctx context.Context, resp *http.Response, client *Client) (*Response, error) {
	response := &Response{
		RawResponse: resp,
		Context:     ctx,
		BodyBytes:   nil,
		Client:      client,
	}

	buf := GetBuffer() // Use the buffer pool
	defer PutBuffer(buf)

	_, err := buf.ReadFrom(resp.Body)
	if err != nil {
		return response, fmt.Errorf("%w: %v", ErrResponseReadFailed, err)
	}
	_ = resp.Body.Close()

	resp.Body = io.NopCloser(bytes.NewReader(buf.B))
	response.BodyBytes = buf.B

	return response, nil
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

// Location returns the URL redirected address
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

// Checks if the response Content-Type header matches a given content type.
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

// Body returns the response body as a byte slice.
func (r *Response) Body() []byte {
	return r.BodyBytes
}

// String returns the response body as a string.
func (r *Response) String() string {
	return string(r.BodyBytes)
}

// Scan attempts to unmarshal the response body based on its content type.
func (r *Response) Scan(v interface{}) error {
	if r.IsJSON() {
		return r.ScanJSON(v)
	} else if r.IsXML() {
		return r.ScanXML(v)
	} else if r.IsYAML() {
		return r.ScanYAML(v)
	}
	return fmt.Errorf("%w: %s", ErrUnsupportedContentType, r.ContentType())
}

// ScanJSON unmarshals the response body into a struct via JSON decoding.
func (r *Response) ScanJSON(v interface{}) error {
	if r.BodyBytes == nil {
		return nil
	}
	return r.Client.JSONDecoder.Decode(bytes.NewReader(r.BodyBytes), v)
}

// ScanXML unmarshals the response body into a struct via XML decoding.
func (r *Response) ScanXML(v interface{}) error {
	if r.BodyBytes == nil {
		return nil
	}
	return r.Client.XMLDecoder.Decode(bytes.NewReader(r.BodyBytes), v)
}

// ScanYAML unmarshals the response body into a struct via YAML decoding.
func (r *Response) ScanYAML(v interface{}) error {
	if r.BodyBytes == nil {
		return nil
	}
	return r.Client.YAMLDecoder.Decode(bytes.NewReader(r.BodyBytes), v)
}

// Save saves the response body to a file or io.Writer.
func (r *Response) Save(v any) error {
	switch p := v.(type) {
	case string:
		file := filepath.Clean(p)
		dir := filepath.Dir(file)

		// Create the directory if it doesn't exist
		if _, err := os.Stat(dir); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("failed to check directory: %w", err)
			}

			if err = os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		// Create and open the file for writing
		outFile, err := os.Create(file)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer outFile.Close() // Ensure file is closed after writing

		// Write the response body to the file
		_, err = io.Copy(outFile, bytes.NewReader(r.Body()))
		if err != nil {
			return fmt.Errorf("failed to write response body to file: %w", err)
		}

		return nil
	case io.Writer:
		// Write the response body directly to the provided io.Writer
		_, err := io.Copy(p, bytes.NewReader(r.Body()))
		if err != nil {
			return fmt.Errorf("failed to write response body to io.Writer: %w", err)
		}
		// If the writer can be closed, close it
		if pc, ok := p.(io.WriteCloser); ok {
			defer pc.Close() // Deferred close, ignoring errors as they are not critical here
		}

		return nil
	default:
		// Return an error if the provided type is not supported
		return ErrNotSupportSaveMethod
	}
}

// Close closes the response body.
func (r *Response) Close() error {
	return r.RawResponse.Body.Close()
}
