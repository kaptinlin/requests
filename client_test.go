package requests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	json2 "github.com/go-json-experiment/json"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

// startTestHTTPServer starts a test HTTP server that responds to various endpoints for testing purposes.
func startTestHTTPServer() *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/test-get", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "GET response")
	})

	handler.HandleFunc("/test-post", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "POST response")
	})

	handler.HandleFunc("/test-put", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "PUT response")
	})

	handler.HandleFunc("/test-delete", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "DELETE response")
	})

	handler.HandleFunc("/test-patch", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "PATCH response")
	})

	handler.HandleFunc("/test-status-code", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated) // 201
		_, _ = fmt.Fprintln(w, `Created`)
	})

	handler.HandleFunc("/test-headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "TestValue")
		_, _ = fmt.Fprintln(w, `Headers test`)
	})

	handler.HandleFunc("/test-cookies", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "test-cookie", Value: "cookie-value"})
		_, _ = fmt.Fprintln(w, `Cookies test`)
	})

	handler.HandleFunc("/test-body", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "This is the response body.")
	})

	handler.HandleFunc("/test-empty", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // Send a 200 OK status
		// Don't write any body to ensure it's empty
	})

	handler.HandleFunc("/test-json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, `{"message": "This is a JSON response", "status": true}`)
	})

	handler.HandleFunc("/test-xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprintln(w, `<Response><Message>This is an XML response</Message><Status>true</Status></Response>`)
	})

	handler.HandleFunc("/test-text", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprintln(w, `This is a text response`)
	})

	handler.HandleFunc("/test-pdf", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = fmt.Fprintln(w, `This is a PDF response`)
	})

	handler.HandleFunc("/test-redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/test-redirected", http.StatusFound)
	})

	handler.HandleFunc("/test-redirected", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "Redirected")
	})

	handler.HandleFunc("/test-failure", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	})

	return httptest.NewServer(handler)
}

// testRoundTripperFunc type is an adapter to allow the use of ordinary functions as http.RoundTrippers.
type testRoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip executes a single HTTP transaction.
func (f testRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// TestSetHTTPClient verifies that SetHTTPClient correctly sets a custom http.Client
// and uses it for subsequent requests, specifically checking for cookie modifications.
func TestSetHTTPClient(t *testing.T) {
	// Create a mock server that inspects incoming requests for a specific cookie.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for the presence of a specific cookie.
		cookie, err := r.Cookie("X-Custom-Test-Cookie")
		if err != nil || cookie.Value != "true" {
			// If the cookie is missing or not as expected, respond with a 400 Bad Request.
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// If the cookie is present and correct, respond with a 200 OK.
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Create a new instance of your Client.
	client := Create(&Config{
		BaseURL: mockServer.URL, // Use the mock server URL in the client configuration.
	})

	// Define a custom transport that adds a custom cookie to all outgoing requests.
	customTransport := testRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		// Add the custom cookie to the request.
		req.AddCookie(&http.Cookie{Name: "X-Custom-Test-Cookie", Value: "true"})
		// Proceed with the default transport after adding the cookie.
		return http.DefaultTransport.RoundTrip(req)
	})

	// Set the custom http.Client with the custom transport to your Client.
	client.SetHTTPClient(&http.Client{
		Transport: customTransport,
	})

	// Send a request using the custom http.Client.
	resp, err := client.Get("/test").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Close() //nolint: errcheck

	// Verify that the server responded with a 200 OK, indicating the custom cookie was successfully added.
	if resp.StatusCode() != http.StatusOK {
		t.Errorf("Expected status code 200, got %d. Indicates custom cookie was not recognized by the server.", resp.StatusCode())
	}
}

func TestClientURL(t *testing.T) {
	client := URL("http://localhost:8080")
	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8080", client.BaseURL)
}

func TestClientGetRequest(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/test-get").Send(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "GET response\n", resp.String())
}

func TestClientPostRequest(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Post("/test-post").Body(map[string]any{"key": "value"}).Send(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "POST response\n", resp.String())
}

func TestClientPutRequest(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Put("/test-put").Body(map[string]any{"key": "value"}).Send(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "PUT response\n", resp.String())
}

func TestClientDeleteRequest(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Delete("/test-delete").Send(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "DELETE response\n", resp.String())
}

func TestClientPatchRequest(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Patch("/test-patch").Body(map[string]any{"key": "value"}).Send(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "PATCH response\n", resp.String())
}

func TestClientOptionsRequest(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Options("/test-get").Send(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.RawResponse.StatusCode)
}

func TestClientHeadRequest(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Head("/test-get").Send(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.RawResponse.StatusCode)
}

func TestClientTraceRequest(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.TRACE("/test-get").Send(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.RawResponse.StatusCode)
}

func TestClientCustomMethodRequest(t *testing.T) {
	server := startTestHTTPServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Custom("/test-get", "OPTIONS").Send(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.RawResponse.StatusCode)
}

// testSchema represents the JSON structure for testing.
type testSchema struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// TestSetJSONMarshal tests custom JSON marshal functionality.
func TestSetJSONMarshal(t *testing.T) {
	// Start a mock HTTP server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body from the request
		var received testSchema
		err := json.NewDecoder(r.Body).Decode(&received)
		assert.NoError(t, err)
		assert.Equal(t, "John Doe", received.Name)
		assert.Equal(t, 30, received.Age)
	}))
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})

	// Set the custom JSON marshal function using JSON v2
	client.SetJSONMarshal(func(v any) ([]byte, error) {
		return json2.Marshal(v)
	})

	// Create a test data instance.
	data := testSchema{
		Name: "John Doe",
		Age:  30,
	}

	// Send a request with the custom marshaled body.
	resp, err := client.Post("/").JSONBody(&data).Send(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

// TestSetJSONUnmarshal tests custom JSON unmarshal functionality.
func TestSetJSONUnmarshal(t *testing.T) {
	// Mock response data.
	mockResponse := `{"name":"Jane Doe","age":25}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, mockResponse)
	}))
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})

	// Set the custom JSON unmarshal function using JSON v2
	client.SetJSONUnmarshal(func(data []byte, v any) error {
		return json2.Unmarshal(data, v)
	})

	// Fetch and unmarshal the response.
	resp, err := client.Get("/").Send(context.Background())
	assert.NoError(t, err)

	var result testSchema
	err = resp.Scan(&result)
	assert.NoError(t, err)
	assert.Equal(t, "Jane Doe", result.Name)
	assert.Equal(t, 25, result.Age)
}

type xmlTestSchema struct {
	XMLName xml.Name `xml:"Test"`
	Message string   `xml:"Message"`
	Status  bool     `xml:"Status"`
}

func TestSetXMLMarshal(t *testing.T) {
	// Mock server to check the received XML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var received xmlTestSchema
		err := xml.NewDecoder(r.Body).Decode(&received)
		assert.NoError(t, err)
		assert.Equal(t, "Test message", received.Message)
		assert.True(t, received.Status)
	}))
	defer server.Close()

	// Create your client and set the XML marshal function to use Go's default
	client := Create(&Config{BaseURL: server.URL})
	client.SetXMLMarshal(xml.Marshal)

	// Data to marshal and send
	data := xmlTestSchema{
		Message: "Test message",
		Status:  true,
	}

	// Marshal and send the data
	resp, err := client.Post("/").XMLBody(&data).Send(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestSetXMLUnmarshal(t *testing.T) {
	// Mock server to send XML data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprintln(w, `<Test><Message>Response message</Message><Status>true</Status></Test>`)
	}))
	defer server.Close()

	// Create your client and set the XML unmarshal function to use Go's default
	client := Create(&Config{BaseURL: server.URL})
	client.SetXMLUnmarshal(xml.Unmarshal)

	// Fetch and attempt to unmarshal the data
	resp, err := client.Get("/").Send(context.Background())
	assert.NoError(t, err)

	var result xmlTestSchema
	err = resp.Scan(&result)
	assert.NoError(t, err)
	assert.Equal(t, "Response message", result.Message)
	assert.True(t, result.Status)
}

func TestSetYAMLMarshal(t *testing.T) {
	// Define a test schema
	type yamlTestSchema struct {
		Message string `yaml:"message"`
		Status  bool   `yaml:"status"`
	}

	// Mock server to check the received YAML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var received yamlTestSchema
		err := yaml.NewDecoder(r.Body).Decode(&received)
		assert.NoError(t, err)
		assert.Equal(t, "Test message", received.Message)
		assert.True(t, received.Status)
	}))
	defer server.Close()

	// Create your client and set the YAML marshal function to use goccy/go-yaml's Marshal
	client := Create(&Config{BaseURL: server.URL})
	client.SetYAMLMarshal(yaml.Marshal)

	// Data to marshal and send
	data := yamlTestSchema{
		Message: "Test message",
		Status:  true,
	}

	// Marshal and send the data
	resp, err := client.Post("/").YAMLBody(&data).Send(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestSetYAMLUnmarshal(t *testing.T) {
	// Define a test schema
	type yamlTestSchema struct {
		Message string `yaml:"message"`
		Status  bool   `yaml:"status"`
	}

	// Mock server to send YAML data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = fmt.Fprintln(w, "message: Response message\nstatus: true")
	}))
	defer server.Close()

	// Create your client and set the YAML unmarshal function to use goccy/go-yaml's Unmarshal
	client := Create(&Config{BaseURL: server.URL})
	client.SetYAMLUnmarshal(yaml.Unmarshal)

	// Fetch and attempt to unmarshal the data
	resp, err := client.Get("/").Send(context.Background())
	assert.NoError(t, err)

	var result yamlTestSchema
	err = resp.Scan(&result)
	assert.NoError(t, err)
	assert.Equal(t, "Response message", result.Message)
	assert.True(t, result.Status)
}

// TestSetAuth verifies that SetAuth correctly sets the Authorization header for basic authentication.
func TestSetAuth(t *testing.T) {
	// Expected username and password.
	expectedUsername := "testuser"
	expectedPassword := "testpass"

	// Expected Authorization header value.
	expectedAuthValue := "Basic " + base64.StdEncoding.EncodeToString([]byte(expectedUsername+":"+expectedPassword))

	// Create a mock server that checks the Authorization header.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the Authorization header from the request.
		authHeader := r.Header.Get("Authorization")

		// Check if the Authorization header matches the expected value.
		if authHeader != expectedAuthValue {
			// If not, respond with 401 Unauthorized.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// If the header is correct, respond with 200 OK.
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Initialize your client.
	client := Create(&Config{
		BaseURL: mockServer.URL, // Use the mock server URL.
	})

	// Set basic authentication using the SetBasicAuth method.
	client.SetAuth(BasicAuth{
		Username: expectedUsername,
		Password: expectedPassword,
	})

	// Send the request through the client.
	resp, err := client.Get("/").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Close() //nolint: errcheck

	// Check the response status code.
	if resp.StatusCode() != http.StatusOK {
		t.Errorf("Expected status code 200, got %d. Indicates Authorization header was not set correctly.", resp.StatusCode())
	}
}

func TestSetDefaultHeaders(t *testing.T) {
	// Create a mock server to check headers
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Header") != "HeaderValue" {
			t.Error("Default header 'X-Custom-Header' not found or value incorrect")
		}
	}))
	defer mockServer.Close()

	// Initialize the client and set a default header
	client := Create(&Config{BaseURL: mockServer.URL})
	client.SetDefaultHeader("X-Custom-Header", "HeaderValue")

	// Make a request to trigger the header check
	_, err := client.Get("/").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
}

func TestDelDefaultHeader(t *testing.T) {
	// Mock server to check for the absence of a specific header
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Deleted-Header") != "" {
			t.Error("Deleted default header 'X-Deleted-Header' was found in the request")
		}
	}))
	defer mockServer.Close()

	// Initialize the client, set, and then delete a default header
	client := Create(&Config{BaseURL: mockServer.URL})
	client.SetDefaultHeader("X-Deleted-Header", "ShouldBeDeleted")
	client.DelDefaultHeader("X-Deleted-Header")

	// Make a request to check for the absence of the deleted header
	_, err := client.Get("/").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
}

func TestSetDefaultContentType(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check the Content-Type header
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Default Content-Type header not set correctly")
		}
	}))
	defer mockServer.Close()

	client := Create(&Config{BaseURL: mockServer.URL})
	client.SetDefaultContentType("application/json")

	_, err := client.Get("/").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
}

func TestSetDefaultAccept(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check the Accept header
		if r.Header.Get("Accept") != "application/xml" {
			t.Error("Default Accept header not set correctly")
		}
	}))
	defer mockServer.Close()

	client := Create(&Config{BaseURL: mockServer.URL})
	client.SetDefaultAccept("application/xml")

	_, err := client.Get("/").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
}

func TestSetDefaultUserAgent(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check the User-Agent header
		if r.Header.Get("User-Agent") != "MyCustomAgent/1.0" {
			t.Error("Default User-Agent header not set correctly")
		}
	}))
	defer mockServer.Close()

	client := Create(&Config{BaseURL: mockServer.URL})
	client.SetDefaultUserAgent("MyCustomAgent/1.0")

	_, err := client.Get("/").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
}

func TestSetDefaultTimeout(t *testing.T) {
	// Create a server that delays its response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay longer than client's timeout
	}))
	defer mockServer.Close()

	client := Create(&Config{BaseURL: mockServer.URL})
	client.SetDefaultTimeout(1 * time.Second) // Set timeout to 1 second

	_, err := client.Get("/").Send(context.Background())
	if err == nil {
		t.Fatal("Expected a timeout error, got nil")
	}

	// Check if the error is a timeout error.
	// This method of checking for a timeout is more generic and should cover the observed error.
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		// If here, it's a timeout error
	} else {
		t.Fatalf("Expected a timeout error, got %v", err)
	}
}

func TestSetDefaultCookieJar(t *testing.T) {
	jar, _ := cookiejar.New(nil)

	// Initialize the client and set the default cookie jar
	client := Create(&Config{})
	client.SetDefaultCookieJar(jar)

	// Start a test HTTP server that sets a cookie
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/set-cookie" {
			http.SetCookie(w, &http.Cookie{Name: "test", Value: "cookie"})
			return
		}

		// Check for the cookie on a different endpoint
		cookie, err := r.Cookie("test")
		if err != nil {
			t.Fatal("Cookie 'test' not found in request, cookie jar not working")
		}
		if cookie.Value != "cookie" {
			t.Fatalf("Expected cookie 'test' to have value 'cookie', got '%s'", cookie.Value)
		}
	}))
	defer server.Close()

	// First request to set the cookie
	_, err := client.Get(server.URL + "/set-cookie").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Second request to check if the cookie is sent back
	_, err = client.Get(server.URL + "/check-cookie").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send second request: %v", err)
	}
}

func TestSetDefaultCookies(t *testing.T) {
	// Create a mock server to check cookies
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for the presence of specific cookies
		sessionCookie, err := r.Cookie("session_id")
		if err != nil || sessionCookie.Value != "abcd1234" {
			t.Error("Default cookie 'session_id' not found or value incorrect")
		}

		authCookie, err := r.Cookie("auth_token")
		if err != nil || authCookie.Value != "token1234" {
			t.Error("Default cookie 'auth_token' not found or value incorrect")
		}
	}))
	defer mockServer.Close()

	// Initialize the client and set default cookies
	client := Create(&Config{BaseURL: mockServer.URL})
	client.SetDefaultCookies(map[string]string{
		"session_id": "abcd1234",
		"auth_token": "token1234",
	})

	// Make a request to trigger the cookie check
	_, err := client.Get("/").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
}

func TestDelDefaultCookie(t *testing.T) {
	// Mock server to check for absence of a specific cookie
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := r.Cookie("session_id")
		if err == nil {
			t.Error("Deleted default cookie 'session_id' was found in the request")
		}
	}))
	defer mockServer.Close()

	// Initialize the client, set, and then delete a default cookie
	client := Create(&Config{BaseURL: mockServer.URL})
	client.SetDefaultCookie("session_id", "abcd1234")
	client.DelDefaultCookie("session_id")

	// Make a request to check for the absence of the deleted cookie
	_, err := client.Get("/").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
}

// Helper function to create a test TLS server.
func createTestTLSServer() *httptest.Server {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Load server certificate and key.
	cert, err := tls.LoadX509KeyPair(".github/testdata/cert.pem", ".github/testdata/key.pem")
	if err != nil {
		panic("failed to load test certificate: " + err.Error())
	}

	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	server.StartTLS()

	return server
}

func TestSetTLSConfig(t *testing.T) {
	// Start a test TLS server.
	server := createTestTLSServer()
	defer server.Close()

	// Initialize your client pointing to the test server.
	client := URL(server.URL)

	// Configure TLS to skip certificate verification.
	// Note: This is for testing with self-signed certificates only.
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}
	client.SetTLSConfig(tlsConfig)

	// Make a request to the test server.
	response, err := client.Get("/").Send(context.Background())

	// Ensure no error occurred and the request was successful.
	if err != nil {
		t.Fatalf("Failed to send request with custom TLS config: %v", err)
	}
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
}

func TestSetTLSConfigWithCert(t *testing.T) {
	server := createTestTLSServer()
	defer server.Close()

	client := URL(server.URL)

	cert, err := os.ReadFile(".github/testdata/cert.pem")
	if err != nil {
		t.Fatalf("Failed to load server certificate: %v", err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(cert) {
		t.Fatal("Failed to append server certificate to pool")
	}
	require.NoError(t, err, "Failed to load server certificate")

	tlsConfig := &tls.Config{
		RootCAs: certPool,
	}
	client.SetTLSConfig(tlsConfig)

	// Make a request to the test server.
	response, err := client.Get("/").Send(context.Background())

	// Ensure no error occurred and the request was successful.
	if err != nil {
		t.Fatalf("Failed to send request with custom TLS config: %v", err)
	}
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
}

func TestInsecureSkipVerify(t *testing.T) {
	// Start a test TLS server.
	server := createTestTLSServer()
	defer server.Close()

	// Initialize your client pointing to the test server.
	client := URL(server.URL)

	// Configure TLS to skip certificate verification.
	client.InsecureSkipVerify()

	// Make a request to the test server.
	response, err := client.Get("/").Send(context.Background())

	// Ensure no error occurred and the request was successful.
	if err != nil {
		t.Fatalf("Failed to send request with custom TLS config: %v", err)
	}
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
}

func createTestRetryServer(t *testing.T) *httptest.Server {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch requestCount {
		case 1:
			w.WriteHeader(http.StatusInternalServerError) // Simulate server error on first attempt
		case 2:
			w.WriteHeader(http.StatusOK) // Successful on second attempt
		default:
			t.Fatalf("Unexpected number of requests: %d", requestCount)
		}
	}))
	return server
}

func TestSetMaxRetriesAndRetryStrategy(t *testing.T) {
	server := createTestRetryServer(t)
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	retryCalled := false
	client.SetMaxRetries(1).SetRetryStrategy(func(attempt int) time.Duration {
		retryCalled = true
		return 10 * time.Millisecond // Short delay for testing
	})

	// Make a request to the test server
	_, err := client.Get("/").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if !retryCalled {
		t.Error("Expected retry strategy to be called, but it wasn't")
	}
}

func TestSetRetryIf(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // Always return server error
	}))
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})
	client.SetMaxRetries(2).SetRetryIf(func(req *http.Request, resp *http.Response, err error) bool {
		// Only retry for 500 Internal Server Error
		return resp.StatusCode == http.StatusInternalServerError
	})

	retryCount := 0
	client.SetRetryStrategy(func(int) time.Duration {
		retryCount++
		return 10 * time.Millisecond // Short delay for testing
	})

	// Make a request to the test server
	_, err := client.Get("/").Send(context.Background())
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if retryCount != 2 {
		t.Errorf("Expected 2 retries, got %d", retryCount)
	}
}

func TestClientCertificates(t *testing.T) {
	serverCert, err := tls.LoadX509KeyPair(".github/testdata/cert.pem", ".github/testdata/key.pem")
	require.NoError(t, err, "load server certificate failed")

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("certificate verification successful"))
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("lack of client certificate"))
		}
	}))
	clientCertPool := x509.NewCertPool()
	clientCertData, err := os.ReadFile(".github/testdata/cert.pem")
	require.NoError(t, err, "load client certificate failed")
	clientCertPool.AppendCertsFromPEM(clientCertData)
	clientCertPath := ".github/testdata/cert.pem"

	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    clientCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	server.StartTLS()
	defer server.Close()

	client := Create(&Config{
		BaseURL: server.URL,
	})

	t.Run("use client certificate", func(t *testing.T) {
		clientCert, err := tls.LoadX509KeyPair(".github/testdata/cert.pem", ".github/testdata/key.pem")
		require.NoError(t, err, "load client certificate failed")

		client.SetTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		})
		client.SetCertificates(clientCert)
		client.SetClientRootCertificate(clientCertPath)
		resp, err := client.Get("/").Send(context.Background())
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Close() //nolint:errcheck

		assert.Equal(t, http.StatusOK, resp.StatusCode(), "status code not correct")
		assert.Equal(t, "certificate verification successful", resp.String(), "response content not correct")
	})

	t.Run("do not use client certificate", func(t *testing.T) {
		clientWithoutCert := Create(&Config{
			BaseURL: server.URL,
		})
		clientWithoutCert.SetTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		})
		clientWithoutCert.SetClientRootCertificate(clientCertPath)

		_, err := clientWithoutCert.Get("/").Send(context.Background())
		assert.Error(t, err, "expect request failed")
	})
}

func TestClientSetRootCertificate(t *testing.T) {
	t.Run("root cert", func(t *testing.T) {
		filePath := ".testdata/sample_root.pem"

		client := Create(nil)
		client.SetRootCertificate(filePath)

		if transport, ok := client.HTTPClient.Transport.(*http.Transport); ok {
			assert.NotNil(t, transport.TLSClientConfig.RootCAs)
		}
	})

	t.Run("root cert not exists", func(t *testing.T) {
		filePath := "../.testdata/not-exists-sample-root.pem"

		client := Create(nil)
		client.SetRootCertificate(filePath)

		if transport, ok := client.HTTPClient.Transport.(*http.Transport); ok {
			assert.Nil(t, transport.TLSClientConfig)
		}
	})

	t.Run("root cert from string", func(t *testing.T) {
		client := Create(nil)

		cert := `-----BEGIN CERTIFICATE-----`

		client.SetRootCertificateFromString(cert)
		if transport, ok := client.HTTPClient.Transport.(*http.Transport); ok {
			assert.NotNil(t, transport.TLSClientConfig.RootCAs)
		}
	})
}

func TestHttp2Scenarios(t *testing.T) {
	tests := []struct {
		name            string
		config          *Config
		url             string
		expectedVersion string
		expectedError   string
	}{
		{
			name:            "Default HTTP version, request to use http2 version URL",
			config:          &Config{},
			url:             "https://tools.scrapfly.io/api/fp/anything",
			expectedVersion: "HTTP/2.0",
			expectedError:   "",
		},
		{
			name:            "Explicit HTTP/2, request to use http2 version URL",
			config:          &Config{HTTP2: true},
			url:             "https://tools.scrapfly.io/api/fp/anything",
			expectedVersion: "HTTP/2.0",
			expectedError:   "",
		},
		{
			name: "Set Transport, request to use http2 version URL,The priority of http2 is lower than that of Transport",
			config: &Config{Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			}},
			url:             "https://tools.scrapfly.io/api/fp/anything",
			expectedVersion: "",
			expectedError:   "Get \"https://tools.scrapfly.io/api/fp/anything\": EOF",
		},
		{
			name: "Set Transport, request to use http1.1 version URL",
			config: &Config{Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			}},
			url:             "https://www.baidu.com",
			expectedVersion: "HTTP/1.1",
			expectedError:   "",
		},
		{
			name:            "Explicit HTTP/2 with Baidu",
			config:          &Config{HTTP2: true},
			url:             "https://www.baidu.com",
			expectedVersion: "",
			expectedError:   "Get \"https://www.baidu.com\": http2: unexpected ALPN protocol \"\"; want \"h2\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := Create(tt.config)

			resp, err := client.Get(tt.url).Send(context.Background())
			if err != nil {
				assert.Equal(t, tt.expectedError, err.Error(), "Protocol settings are incorrect")
				return
			}
			defer resp.Close() //nolint:errcheck
			assert.Equal(t, tt.expectedVersion, resp.RawResponse.Proto, "Protocol version mismatch")
		})
	}
}
