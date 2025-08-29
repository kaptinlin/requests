package requests

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			client := Create(&Config{BaseURL: server.URL})
			resp, err := client.Get(tt.url).Send(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, resp.IsContentType(tt.contentType))
		})
	}
}

func TestResponseStatusAndStatusCode(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/test-status-code").Send(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 201, resp.StatusCode())
	assert.Contains(t, resp.Status(), "201 Created")
}

func TestResponseHeaderAndCookies(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})

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

func TestResponseContentLengthAndIsEmpty(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})

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

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/test-status-code").Send(context.Background()) // This endpoint sets status 201
	assert.NoError(t, err)

	assert.True(t, resp.IsSuccess())
}

func TestResponseIsSuccessForFailure(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/test-failure").Send(context.Background()) // This endpoint sets status 500
	assert.NoError(t, err)

	assert.False(t, resp.IsSuccess())
}

func TestResponseAfterRedirect(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/test-redirect").Send(context.Background())
	assert.NoError(t, err)

	bodyStr := resp.String()
	expectedContent := "Redirected\n"
	assert.Contains(t, bodyStr, expectedContent, "The response content should be 'Redirected'")
}

func TestResponseBodyAndString(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/test-body").Send(context.Background())
	assert.NoError(t, err)

	bodyStr := resp.String()
	assert.Contains(t, bodyStr, "This is the response body.")

	bodyBytes := resp.Body()
	assert.Contains(t, string(bodyBytes), "This is the response body.")
}

func TestResponseScanJSON(t *testing.T) {
	type jsonTestResponse struct {
		Message string `json:"message"`
		Status  bool   `json:"status"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, `{"message": "This is a JSON response", "status": true}`)
	}))
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/test-json").Send(context.Background())
	assert.NoError(t, err)

	var jsonResponse jsonTestResponse
	err = resp.Scan(&jsonResponse)
	assert.NoError(t, err)
	assert.Equal(t, "This is a JSON response", jsonResponse.Message)
	assert.True(t, jsonResponse.Status)
}

func TestResponseScanXML(t *testing.T) {
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

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/test-xml").Send(context.Background())
	assert.NoError(t, err)

	var xmlResponse xmlTestResponse
	err = resp.Scan(&xmlResponse)
	assert.NoError(t, err)
	assert.Equal(t, "This is an XML response", xmlResponse.Message)
	assert.True(t, xmlResponse.Status)
}

func TestResponseScanYAML(t *testing.T) {
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

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/test-yaml").Send(context.Background())
	assert.NoError(t, err)

	var yamlResponse yamlTestResponse
	err = resp.Scan(&yamlResponse)
	assert.NoError(t, err)
	assert.Equal(t, "This is a YAML response", yamlResponse.Message)
	assert.True(t, yamlResponse.Status)
}

func TestResponseScanUnsupportedContentType(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/test-pdf").Send(context.Background())
	assert.NoError(t, err)

	var dummyResponse struct{}
	err = resp.Scan(&dummyResponse)
	assert.Error(t, err, "expected an error for unsupported content type")
	assert.ErrorIs(t, err, ErrUnsupportedContentType)
}

func TestResponseClose(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
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
			client := Create(&Config{BaseURL: server.URL})
			resp, err := client.Get(tc.path).Send(context.Background())
			assert.NoError(t, err)

			expectedURL, _ := url.Parse(tc.expected)

			assert.Equal(t, expectedURL.String(), resp.URL().String(), "The response URL should match the expected URL.")
		})
	}
}

func TestResponseSaveToFile(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "Sample response body")
	}))
	defer server.Close()

	// Create client and send request
	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Define the path where to save the response body
	filePath := ".github/testdata/sample_response.txt"
	err = resp.Save(filePath)
	if err != nil {
		t.Fatalf("Failed to save response to file: %v", err)
	}

	// Read the saved file
	savedData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	// Verify the file content
	expected := "Sample response body"
	if string(savedData) != expected {
		t.Errorf("Expected file content %q, got %q", expected, string(savedData))
	}

	// Clean up the saved file
	err = os.Remove(filePath)
	if err != nil {
		t.Fatalf("Failed to remove saved file: %v", err)
	}
}

func TestResponseLines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "Line 1\nLine 2\nLine 3\n")
	}))
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/").Send(context.Background())
	assert.NoError(t, err)

	lines := make([]string, 0)
	for line := range resp.Lines() {
		lines = append(lines, string(line))
	}

	expected := []string{"Line 1", "Line 2", "Line 3"}
	assert.Equal(t, expected, lines)
}

func TestResponseLinesEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		// Empty response
	}))
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/").Send(context.Background())
	assert.NoError(t, err)

	lines := make([]string, 0)
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

	client := Create(&Config{BaseURL: server.URL})
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

func TestResponseSaveToWriter(t *testing.T) {
	// Setup a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "Sample response body")
	}))
	defer server.Close()

	// Create client and send request
	client := Create(&Config{BaseURL: server.URL})
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
