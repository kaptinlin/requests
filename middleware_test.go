package requests

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMiddleware ensures that the Middleware correctly applies
// middleware to outgoing requests.
func TestMiddleware(t *testing.T) {
	// Set up a mock server to inspect incoming requests
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for the custom header added by our middleware
		assert.Equal(t, "true", r.Header.Get("X-Custom-Header"), "Expected custom header 'X-Custom-Header' to be 'true'")
		w.WriteHeader(http.StatusOK) // All good if the header is present
	}))
	defer mockServer.Close()

	// Define the middleware that adds a custom header
	customHeaderMiddleware := func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			// Add the custom header
			req.Header.Set("X-Custom-Header", "true")
			// Proceed with the next middleware or the actual request
			return next(req)
		}
	}

	// Initialize the client with our custom middleware
	client := Create(&Config{
		BaseURL:     mockServer.URL,                       // Use our mock server as the base URL
		Transport:   http.DefaultTransport,                // Use the default transport
		Middlewares: []Middleware{customHeaderMiddleware}, // Apply our custom header middleware
	})

	// Create an HTTP request object
	resp, err := client.Get("/").Send(context.Background())
	require.NoError(t, err, "Failed to send request")
	defer resp.Close() //nolint: errcheck

	// Check if the server responded with a 200 OK, indicating the middleware applied the header successfully
	assert.Equal(t, http.StatusOK, resp.StatusCode(), "Expected status code 200")
}

func TestNestedMiddleware(t *testing.T) {
	var buf bytes.Buffer

	mid0 := func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			buf.WriteString("0>>")
			resp, err := next(req)
			buf.WriteString(">>0")
			return resp, err
		}
	}

	mid1 := func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			buf.WriteString("1>>")
			resp, err := next(req)
			buf.WriteString(">>1")
			return resp, err
		}
	}

	mid2 := func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			buf.WriteString("2>>")
			resp, err := next(req)
			buf.WriteString(">>2")
			return resp, err
		}
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf.WriteString("(served)")
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	client := Create(&Config{
		BaseURL:     mockServer.URL,
		Middlewares: []Middleware{mid0, mid1, mid2},
	})

	// Create an HTTP request object
	resp, err := client.Get("/").Send(context.Background())
	require.NoError(t, err, "Failed to send request")
	defer resp.Close() //nolint: errcheck

	expected := "0>>1>>2>>(served)>>2>>1>>0"
	assert.Equal(t, expected, buf.String(), "Middleware execution order incorrect")
}

// TestDynamicMiddlewareAddition tests the dynamic addition of middleware to the client
func TestDynamicMiddlewareAddition(t *testing.T) {
	// Buffer to track middleware execution order
	var executionOrder bytes.Buffer

	// Define middleware functions
	loggingMiddleware := func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			executionOrder.WriteString("Logging>")
			return next(req)
		}
	}

	authenticationMiddleware := func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			executionOrder.WriteString("Auth>")
			return next(req)
		}
	}

	// Set up a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executionOrder.WriteString("Handler")
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Create a new client
	client := Create(&Config{
		BaseURL: mockServer.URL,
	})

	// Dynamically add middleware
	client.AddMiddleware(loggingMiddleware)
	client.AddMiddleware(authenticationMiddleware)

	// Make a request to the mock server
	_, err := client.Get("/").Send(context.Background())
	require.NoError(t, err, "Failed to send request")

	// Check the order of middleware execution
	expectedOrder := "Logging>Auth>Handler"
	assert.Equal(t, expectedOrder, executionOrder.String(), "Middleware executed in incorrect order")
}

// TestRequestMiddlewareAddition tests the addition of middleware at the request level,
// and ensures that both client and request level middlewares are executed in the correct order.
func TestRequestMiddlewareAddition(t *testing.T) {
	// Buffer to track middleware execution order
	var executionOrder bytes.Buffer

	// Define client-level middleware
	clientLoggingMiddleware := func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			executionOrder.WriteString("ClientLogging>")
			return next(req)
		}
	}

	// Define request-level middleware
	requestAuthMiddleware := func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc {
		return func(req *http.Request) (*http.Response, error) {
			executionOrder.WriteString("RequestAuth>")
			return next(req)
		}
	}

	// Set up a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executionOrder.WriteString("Handler")
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Create a new client with client-level middleware
	client := Create(&Config{
		BaseURL:     mockServer.URL,
		Middlewares: []Middleware{clientLoggingMiddleware}, // Apply client-level middleware
	})

	// Create a request and dynamically add request-level middleware
	reqBuilder := client.Get("/")
	reqBuilder.AddMiddleware(requestAuthMiddleware) // Apply request-level middleware

	// Make a request to the mock server
	_, err := reqBuilder.Send(context.Background())
	require.NoError(t, err, "Failed to send request")

	// Check the order of middleware execution
	expectedOrder := "ClientLogging>RequestAuth>Handler"
	assert.Equal(t, expectedOrder, executionOrder.String(), "Middleware executed in incorrect order")
}
