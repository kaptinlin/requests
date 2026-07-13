package requests

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

var errFailingWriter = errors.New("failing writer")

type recordingResponseBody struct {
	data       []byte
	readErr    error
	closeErr   error
	readCount  int
	closeCount int
}

func (b *recordingResponseBody) Read(p []byte) (int, error) {
	b.readCount++
	if len(b.data) > 0 {
		n := copy(p, b.data)
		b.data = b.data[n:]
		return n, nil
	}
	if b.readErr != nil {
		return 0, b.readErr
	}
	return 0, io.EOF
}

func (b *recordingResponseBody) Close() error {
	b.closeCount++
	return b.closeErr
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errFailingWriter
}

type closeTrackingWriter struct {
	bytes.Buffer
	closed bool
}

func (w *closeTrackingWriter) Close() error {
	w.closed = true
	return nil
}

func TestResponseContentType(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	tests := []struct {
		url         string
		contentType string
		expected    bool
	}{
		{"/test-json", "application/json", true},
		{"/test-xml", "application/xml", true},
		{"/test-text", "text/plain", true},
		{"/test-text", "application/json", false},
		{"/test-json", "text/plain", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("ContentType is %s", tt.contentType), func(t *testing.T) {
			client := newTestClient(t, WithBaseURL(server.URL))
			resp, err := client.Get(tt.url).Send(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, resp.IsContentType(tt.contentType))
		})
	}
}

func TestResponseRejectsJSONPMediaType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/jsonp")
		_, _ = fmt.Fprint(w, `{"message":"not json media"}`)
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(t.Context())
	require.NoError(t, err)

	assert.False(t, resp.IsJSON())
	var decoded map[string]string
	err = resp.Decode(&decoded)
	assert.ErrorIs(t, err, ErrUnsupportedContentType)
}

func TestResponseDecodesStructuredJSONMediaType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{"message":"problem"}`)
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(t.Context())
	require.NoError(t, err)

	assert.True(t, resp.IsJSON())
	var decoded map[string]string
	require.NoError(t, resp.Decode(&decoded))
	assert.Equal(t, "problem", decoded["message"])
}

func TestResponseDecodesStructuredXMLMediaType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = fmt.Fprint(w, `<svg><title>icon</title></svg>`)
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(t.Context())
	require.NoError(t, err)

	assert.True(t, resp.IsXML())
	var decoded struct {
		Title string `xml:"title"`
	}
	require.NoError(t, resp.Decode(&decoded))
	assert.Equal(t, "icon", decoded.Title)
}

func TestResponseDecodesTextXMLMediaType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = fmt.Fprint(w, `<message>hello</message>`)
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(t.Context())
	require.NoError(t, err)

	assert.True(t, resp.IsXML())
	var decoded struct {
		Value string `xml:",chardata"`
	}
	require.NoError(t, resp.Decode(&decoded))
	assert.Equal(t, "hello", decoded.Value)
}

func TestResponseDecodesStructuredYAMLMediaType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.example+yaml")
		_, _ = fmt.Fprint(w, "message: hello\n")
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(t.Context())
	require.NoError(t, err)

	assert.True(t, resp.IsYAML())
	var decoded map[string]string
	require.NoError(t, resp.Decode(&decoded))
	assert.Equal(t, "hello", decoded["message"])
}

func TestResponseMediaTypeClassification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", r.URL.Query().Get("type"))
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer server.Close()
	client := newTestClient(t, WithBaseURL(server.URL))

	tests := []struct {
		name        string
		contentType string
		compare     string
		isJSON      bool
		isXML       bool
		isYAML      bool
		exactMatch  bool
	}{
		{
			name:        "case parameters and whitespace",
			contentType: "Application/JSON ; Charset=UTF-8",
			compare:     "application/json",
			isJSON:      true,
			exactMatch:  true,
		},
		{
			name:        "structured JSON is not exact JSON",
			contentType: "application/problem+json",
			compare:     "application/json",
			isJSON:      true,
		},
		{
			name:        "structured XML",
			contentType: "image/svg+xml",
			compare:     "image/svg+xml; charset=utf-8",
			isXML:       true,
			exactMatch:  true,
		},
		{
			name:        "structured YAML",
			contentType: "application/vnd.example+yaml",
			compare:     "application/vnd.example+yaml",
			isYAML:      true,
			exactMatch:  true,
		},
		{
			name:        "JSONP",
			contentType: "application/jsonp",
			compare:     "application/json",
		},
		{
			name:        "invalid",
			contentType: "not a media type",
			compare:     "not a media type",
		},
		{
			name:        "unsupported",
			contentType: "text/plain",
			compare:     "application/json",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := client.Get("/?type=" + url.QueryEscape(test.contentType)).Send(t.Context())
			require.NoError(t, err)

			assert.Equal(t, test.isJSON, resp.IsJSON())
			assert.Equal(t, test.isXML, resp.IsXML())
			assert.Equal(t, test.isYAML, resp.IsYAML())
			assert.Equal(t, test.exactMatch, resp.IsContentType(test.compare))
			if !test.isJSON && !test.isXML && !test.isYAML {
				var decoded any
				assert.ErrorIs(t, resp.Decode(&decoded), ErrUnsupportedContentType)
			}
		})
	}
}

func TestResponseStatusAndStatusCode(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/test-status-code").Send(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 201, resp.StatusCode())
	assert.Contains(t, resp.Status(), "201 Created")
}

func TestResponseHeaderAndCookies(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))

	t.Run("Test Headers", func(t *testing.T) {
		resp, err := client.Get("/test-headers").Send(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "TestValue", resp.Header().Get("X-Custom-Header"))
	})

	t.Run("Test Cookies", func(t *testing.T) {
		resp, err := client.Get("/test-cookies").Send(context.Background())
		assert.NoError(t, err)
		cookies := resp.Cookies()
		assert.Equal(t, 1, len(cookies))
		assert.Equal(t, "test-cookie", cookies[0].Name)
		assert.Equal(t, "cookie-value", cookies[0].Value)
	})
}

func TestResponseHeaderReturnsSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Snapshot", "raw")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(t.Context())
	require.NoError(t, err)

	headers := resp.Header()
	headers.Set("X-Snapshot", "mutated")
	headers.Set("Content-Type", "application/json")

	assert.Equal(t, "raw", resp.Raw().Header.Get("X-Snapshot"))
	assert.Equal(t, "text/plain", resp.ContentType())
}

func TestResponseLocation(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com/start", nil)
	assert.NoError(t, err)

	resp := &Response{
		rawResponse: &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"/next"}},
			Request:    req,
		},
	}

	location, err := resp.Location()
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com/next", location.String())
}

func TestResponseContentLengthAndIsEmpty(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))

	t.Run("Non-empty response", func(t *testing.T) {
		resp, err := client.Get("/test-content-type?ct=text/plain").Send(context.Background())
		assert.NoError(t, err)
		assert.False(t, resp.IsEmpty())
		assert.Greater(t, resp.ContentLength(), 0)
	})

	t.Run("Empty response", func(t *testing.T) {
		resp, err := client.Get("/test-empty").Send(context.Background())
		assert.NoError(t, err)
		assert.True(t, resp.IsEmpty())
		assert.Equal(t, 0, resp.ContentLength())
	})
}

func TestResponseIsSuccess(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/test-status-code").Send(context.Background()) // This endpoint sets status 201
	assert.NoError(t, err)

	assert.True(t, resp.IsSuccess())
}

func TestResponseIsSuccessForFailure(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/test-failure").Send(context.Background()) // This endpoint sets status 500
	assert.NoError(t, err)

	assert.False(t, resp.IsSuccess())
}

func TestResponseAfterRedirect(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/test-redirect").Send(context.Background())
	assert.NoError(t, err)

	bodyStr := resp.String()
	expectedContent := "Redirected\n"
	assert.Contains(t, bodyStr, expectedContent, "The response content should be 'Redirected'")
}

func TestResponseBytesAndString(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/test-body").Send(context.Background())
	assert.NoError(t, err)

	bodyStr := resp.String()
	assert.Contains(t, bodyStr, "This is the response body.")

	bodyBytes := resp.Bytes()
	assert.Contains(t, string(bodyBytes), "This is the response body.")
	bodyBytes[0] = 'x'
	assert.Contains(t, resp.String(), "This is the response body.")
	assert.Contains(t, string(resp.Bytes()), "This is the response body.")

	var lines []string
	for line := range resp.Lines() {
		lines = append(lines, string(line))
	}
	assert.Contains(t, lines, "This is the response body.")
}

func TestResponseRaw(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Raw", "yes")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, "raw body")
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/raw").Send(t.Context())
	require.NoError(t, err)

	raw := resp.Raw()
	assert.Equal(t, http.StatusCreated, raw.StatusCode)
	assert.Equal(t, "yes", raw.Header.Get("X-Raw"))

	body, err := io.ReadAll(raw.Body)
	require.NoError(t, err)
	assert.Equal(t, "raw body", string(body))
}

func TestResponseDecodeJSON(t *testing.T) {
	type jsonTestResponse struct {
		Message string `json:"message"`
		Status  bool   `json:"status"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, `{"message": "This is a JSON response", "status": true}`)
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/test-json").Send(context.Background())
	assert.NoError(t, err)

	var jsonResponse jsonTestResponse
	err = resp.Decode(&jsonResponse)
	assert.NoError(t, err)
	assert.Equal(t, "This is a JSON response", jsonResponse.Message)
	assert.True(t, jsonResponse.Status)
}

func TestResponseDecodeUsesDispatchSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ignored": true}`)
	}))
	defer server.Close()

	client := newTestClient(t,
		WithBaseURL(server.URL),
		WithJSONDecoder(&JSONDecoder{
			DecodeFunc: func(_ io.Reader, v any) error {
				out := v.(*struct{ Source string })
				out.Source = "dispatch"
				return nil
			},
		}),
	)

	resp, err := client.Get("/").Send(t.Context())
	require.NoError(t, err)

	client.jsonDecoder = &JSONDecoder{
		DecodeFunc: func(_ io.Reader, v any) error {
			out := v.(*struct{ Source string })
			out.Source = "mutated"
			return nil
		},
	}

	var out struct{ Source string }
	err = resp.DecodeJSON(&out)
	require.NoError(t, err)
	assert.Equal(t, "dispatch", out.Source)
}

func TestResponseDecodeXML(t *testing.T) {
	type xmlTestResponse struct {
		XMLName xml.Name `xml:"Response"`
		Message string   `xml:"Message"`
		Status  bool     `xml:"Status"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprintln(w, `<Response><Message>This is an XML response</Message><Status>true</Status></Response>`)
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/test-xml").Send(context.Background())
	assert.NoError(t, err)

	var xmlResponse xmlTestResponse
	err = resp.Decode(&xmlResponse)
	assert.NoError(t, err)
	assert.Equal(t, "This is an XML response", xmlResponse.Message)
	assert.True(t, xmlResponse.Status)
}

func TestResponseDecodeYAML(t *testing.T) {
	type yamlTestResponse struct {
		Message string `yaml:"message"`
		Status  bool   `yaml:"status"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		yml := `---
message: This is a YAML response
status: true
`
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = fmt.Fprint(w, yml)
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/test-yaml").Send(context.Background())
	assert.NoError(t, err)

	var yamlResponse yamlTestResponse
	err = resp.Decode(&yamlResponse)
	assert.NoError(t, err)
	assert.Equal(t, "This is a YAML response", yamlResponse.Message)
	assert.True(t, yamlResponse.Status)
}

func TestResponseDecodeUnsupportedContentType(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/test-pdf").Send(context.Background())
	assert.NoError(t, err)

	var dummyResponse struct{}
	err = resp.Decode(&dummyResponse)
	assert.Error(t, err, "expected an error for unsupported content type")
	assert.ErrorIs(t, err, ErrUnsupportedContentType)
}

func TestResponseClose(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/test-get").Send(context.Background())
	assert.NoError(t, err)

	err = resp.Close()
	assert.NoError(t, err, "expected no error when closing the response")
}

func TestResponseURL(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	tests := []struct {
		name     string
		path     string // Path to append to the base URL
		expected string // Expected final URL (for comparison)
	}{
		{
			name:     "Base URL",
			path:     "",
			expected: server.URL,
		},
		{
			name:     "Path Parameter",
			path:     "/path-param",
			expected: server.URL + "/path-param",
		},
		{
			name:     "Query Parameter",
			path:     "/query?param=value",
			expected: server.URL + "/query?param=value",
		},
		{
			name:     "Hash Fragment",
			path:     "/hash#fragment",
			expected: server.URL + "/hash#fragment",
		},
		{
			name:     "Complex URL",
			path:     "/complex/path?param=value#fragment",
			expected: server.URL + "/complex/path?param=value#fragment",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t, WithBaseURL(server.URL))
			resp, err := client.Get(tc.path).Send(context.Background())
			assert.NoError(t, err)

			expectedURL, _ := url.Parse(tc.expected)

			assert.Equal(t, expectedURL.String(), resp.URL().String(), "The response URL should match the expected URL.")
		})
	}
}

func TestResponseDiagnostics(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/test-get").Send(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, 1, resp.Attempts())
	assert.Greater(t, resp.Elapsed(), time.Duration(0))
	assert.Equal(t, resp.Raw().Proto, resp.Protocol())
}

func TestResponseTLSReturnsCopy(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	require.NoError(t, client.insecureSkipVerify())
	resp, err := client.Get("/").Send(context.Background())
	assert.NoError(t, err)

	state := resp.TLS()
	if assert.NotNil(t, state) && assert.NotEmpty(t, state.PeerCertificates) {
		state.PeerCertificates = nil
		assert.NotEmpty(t, resp.Raw().TLS.PeerCertificates)
	}
}

func TestResponseSaveToFileCreatesParentDirectories(t *testing.T) {
	t.Parallel()

	resp := &Response{
		body: []byte("Sample response body"),
	}
	filePath := filepath.Join(t.TempDir(), "nested", "sample_response.txt")

	require.NoError(t, resp.Save(filePath))

	savedData, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "Sample response body", string(savedData))
}

func TestResponseLines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "Line 1\nLine 2\nLine 3\n")
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(context.Background())
	assert.NoError(t, err)

	lines := make([]string, 0, 3)
	for line := range resp.Lines() {
		lines = append(lines, string(line))
	}

	expected := []string{"Line 1", "Line 2", "Line 3"}
	assert.Equal(t, expected, lines)
}

func TestResponseLinesReturnsLongBufferedLine(t *testing.T) {
	want := bytes.Repeat([]byte("x"), 600<<10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(want)
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(t.Context())
	require.NoError(t, err)

	var lines [][]byte
	for line := range resp.Lines() {
		lines = append(lines, line)
	}
	require.Len(t, lines, 1)
	assert.Equal(t, want, lines[0])
}

func TestResponseLinesReturnsCallerOwnedBytes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "first\nsecond")
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(t.Context())
	require.NoError(t, err)

	var first []byte
	for line := range resp.Lines() {
		first = line
		break
	}
	require.NotEmpty(t, first)
	first[0] = 'X'

	assert.Equal(t, "first\nsecond", resp.String())
	var lines []string
	for line := range resp.Lines() {
		lines = append(lines, string(line))
	}
	assert.Equal(t, []string{"first", "second"}, lines)
}

func TestResponseLinesMatchesScanLinesSemantics(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "empty"},
		{name: "consecutive blank lines", body: "\n\n", want: []string{"", ""}},
		{name: "CRLF", body: "first\r\nsecond\r\n", want: []string{"first", "second"}},
		{name: "trailing newline", body: "first\n", want: []string{"first"}},
		{name: "no newline", body: "first", want: []string{"first"}},
		{name: "terminal carriage return", body: "first\r", want: []string{"first"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, test.body)
			}))
			defer server.Close()

			client := newTestClient(t, WithBaseURL(server.URL))
			resp, err := client.Get("/").Send(t.Context())
			require.NoError(t, err)

			var got []string
			for line := range resp.Lines() {
				got = append(got, string(line))
			}
			assert.Equal(t, test.want, got)
		})
	}
}

func TestResponseLinesEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		// Empty response
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(context.Background())
	assert.NoError(t, err)

	lines := make([]string, 0, 1)
	for line := range resp.Lines() {
		lines = append(lines, string(line))
	}

	assert.Empty(t, lines)
}

func TestResponseLinesEarlyBreak(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n")
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(context.Background())
	assert.NoError(t, err)

	lines := make([]string, 0)
	for line := range resp.Lines() {
		lines = append(lines, string(line))
		// Break after collecting 3 lines
		if len(lines) >= 3 {
			break
		}
	}

	expected := []string{"Line 1", "Line 2", "Line 3"}
	assert.Equal(t, expected, lines)
}

func TestResponseIsError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"200 OK", 200, false},
		{"301 Redirect", 301, false},
		{"400 Bad Request", 400, true},
		{"404 Not Found", 404, true},
		{"500 Internal Server Error", 500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := newTestClient(t, WithBaseURL(server.URL))
			resp, err := client.Get("/").Send(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, resp.IsError())
		})
	}
}

func TestResponseIsClientError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"200 OK", 200, false},
		{"400 Bad Request", 400, true},
		{"404 Not Found", 404, true},
		{"499 Custom", 499, true},
		{"500 Server Error", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := newTestClient(t, WithBaseURL(server.URL))
			resp, err := client.Get("/").Send(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, resp.IsClientError())
		})
	}
}

func TestResponseIsServerError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"200 OK", 200, false},
		{"404 Not Found", 404, false},
		{"500 Internal Server Error", 500, true},
		{"503 Service Unavailable", 503, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := newTestClient(t, WithBaseURL(server.URL))
			resp, err := client.Get("/").Send(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, resp.IsServerError())
		})
	}
}

func TestResponseIsRedirect(t *testing.T) {
	// Use a server that doesn't actually redirect (we just check the code)
	// We need to use a server that returns a redirect status without Location header
	// to prevent the client from following the redirect.
	client := newTestClient(t)
	client.setRedirectPolicy(NewProhibitRedirectPolicy())

	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"200 OK", 200, false},
		{"301 Moved", 301, true},
		{"302 Found", 302, true},
		{"304 Not Modified", 304, true},
		{"400 Bad Request", 400, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			// For non-redirect status codes, use a normal client
			c := newTestClient(t, WithBaseURL(server.URL))
			resp, err := c.Get("/").Send(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, resp.IsRedirect())
		})
	}
}

func TestResponseSaveToWriter(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "Sample response body")
	}))
	defer server.Close()

	// Create client and send request
	client := newTestClient(t, WithBaseURL(server.URL))
	resp, err := client.Get("/").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Use bytes.Buffer as the writer
	var buffer bytes.Buffer
	err = resp.Save(&buffer)
	if err != nil {
		t.Fatalf("Failed to save response to buffer: %v", err)
	}

	// Verify the buffer content
	expected := "Sample response body"
	if buffer.String() != expected {
		t.Errorf("Expected buffer content %q, got %q", expected, buffer.String())
	}
}

func TestResponseSaveToWriterError(t *testing.T) {
	resp := &Response{
		body: []byte("body"),
	}

	err := resp.Save(failingWriter{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errFailingWriter))
}

func TestResponseSaveToWriterDoesNotCloseCallerWriter(t *testing.T) {
	resp := &Response{
		body: []byte("body"),
	}
	writer := &closeTrackingWriter{}

	err := resp.Save(writer)

	require.NoError(t, err)
	assert.Equal(t, "body", writer.String())
	assert.False(t, writer.closed)
}

func TestHandleNonStream_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	const goroutines = 50

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return the requested ID so each goroutine gets unique data
		id := r.URL.Query().Get("id")
		_, _ = fmt.Fprintf(w, "response-%s", id)
	}))
	defer server.Close()

	client := newTestClient(t, WithBaseURL(server.URL))

	results := make([]string, goroutines)

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Go(func() {
			resp, err := client.Get(fmt.Sprintf("/?id=%d", i)).Send(context.Background())
			if err != nil {
				results[i] = fmt.Sprintf("ERROR: %v", err)
				return
			}
			results[i] = string(resp.Bytes())
		})
	}
	wg.Wait()

	for i := range goroutines {
		expected := fmt.Sprintf("response-%d", i)
		assert.Equal(t, expected, results[i], "goroutine %d: body data mismatch (possible data race)", i)
	}
}

func TestBufferedResponseClosesBodyOnReadError(t *testing.T) {
	readErr := errors.New("response read failed")
	body := &recordingResponseBody{readErr: readErr}
	httpClient := &http.Client{Transport: testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       body,
			Request:    req,
		}, nil
	})}
	client := newTestClient(t, WithHTTPClient(httpClient))

	resp, err := client.Get("http://example.test/").Send(t.Context())

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrResponseReadFailed)
	assert.ErrorIs(t, err, readErr)
	assert.Equal(t, 1, body.closeCount)
}

func TestBufferedResponseClosesBodyAfterSuccessfulRead(t *testing.T) {
	payload := []byte(`{"message":"hello"}`)
	closeErr := errors.New("response close failed")
	body := &recordingResponseBody{data: bytes.Clone(payload), closeErr: closeErr}
	httpClient := &http.Client{Transport: testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       body,
			Request:    req,
		}, nil
	})}
	client := newTestClient(t, WithHTTPClient(httpClient))

	resp, err := client.Get("http://example.test/").Send(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, body.closeCount)
	assert.Equal(t, payload, resp.Bytes())
	assert.Equal(t, string(payload), resp.String())

	var decoded map[string]string
	require.NoError(t, resp.Decode(&decoded))
	assert.Equal(t, map[string]string{"message": "hello"}, decoded)
	rawBody, err := io.ReadAll(resp.Raw().Body)
	require.NoError(t, err)
	assert.Equal(t, payload, rawBody)
	require.NoError(t, resp.Close())
	assert.Equal(t, 1, body.closeCount)
}

func TestBufferedResponseReadErrorTakesPrecedenceOverCloseError(t *testing.T) {
	readErr := errors.New("response read failed")
	closeErr := errors.New("response close failed")
	body := &recordingResponseBody{
		data:     []byte("partial"),
		readErr:  readErr,
		closeErr: closeErr,
	}
	httpClient := &http.Client{Transport: testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       body,
			Request:    req,
		}, nil
	})}
	client := newTestClient(t, WithHTTPClient(httpClient))

	resp, err := client.Get("http://example.test/").Send(t.Context())

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrResponseReadFailed)
	assert.ErrorIs(t, err, readErr)
	assert.NotErrorIs(t, err, closeErr)
	assert.Equal(t, 1, body.closeCount)
}

func TestSendStreamDoesNotPreReadAndClosesBodyOnce(t *testing.T) {
	body := &recordingResponseBody{data: []byte("stream")}
	httpClient := &http.Client{Transport: testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       body,
			Request:    req,
		}, nil
	})}
	client := newTestClient(t, WithHTTPClient(httpClient))

	resp, err := client.Get("http://example.test/").SendStream(t.Context())
	require.NoError(t, err)
	assert.Zero(t, body.readCount)
	assert.Zero(t, body.closeCount)
	require.NoError(t, resp.Close())
	require.NoError(t, resp.Close())
	assert.Equal(t, 1, body.closeCount)
}
