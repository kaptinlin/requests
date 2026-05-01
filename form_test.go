package requests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-json-experiment/json"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

// startFileUploadServer starts a mock server to test file uploads.
func startFileUploadServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the multipart form
		err := r.ParseMultipartForm(10 << 20) // Limit: 10MB
		if err != nil {
			http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
			return
		}

		// Collect file upload details
		uploads := make(map[string][]string)
		for key, files := range r.MultipartForm.File {
			for _, fileHeader := range files {
				file, err := fileHeader.Open()
				if err != nil {
					http.Error(w, "Failed to open file", http.StatusInternalServerError)
					return
				}
				defer file.Close() //nolint: errcheck

				// Read file content (for demonstration; in real tests, might hash or skip)
				content, err := io.ReadAll(file)
				if err != nil {
					http.Error(w, "Failed to read file content", http.StatusInternalServerError)
					return
				}

				// Store file details (e.g., filename and a snippet of content for verification)
				contentSnippet := string(content)
				if len(contentSnippet) > 10 {
					contentSnippet = contentSnippet[:10] + "..."
				}
				uploads[key] = append(uploads[key], fmt.Sprintf("%s: %s", fileHeader.Filename, contentSnippet))
			}
		}

		// Respond with details of the uploaded files in JSON format
		w.Header().Set("Content-Type", "application/json")

		if err = json.MarshalWrite(w, uploads); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
}

func TestFileSetters(t *testing.T) {
	t.Parallel()

	content := io.NopCloser(strings.NewReader("payload"))
	file := &File{}
	file.SetName("avatar")
	file.SetFileName("avatar.txt")
	file.SetContent(content)

	assert.Equal(t, "avatar", file.Name)
	assert.Equal(t, "avatar.txt", file.FileName)
	body, err := io.ReadAll(file.Content)
	require.NoError(t, err)
	assert.Equal(t, "payload", string(body))
}

func TestFormEncoder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "url values", in: url.Values{"name": {"Jane Doe"}}, want: "name=Jane+Doe"},
		{name: "string slice map", in: map[string][]string{"tag": {"go", "http"}}, want: "tag=go&tag=http"},
		{name: "string map", in: map[string]string{"name": "Jane Doe"}, want: "name=Jane+Doe"},
		{name: "struct", in: struct {
			Name string `url:"name"`
		}{Name: "Jane Doe"}, want: "name=Jane+Doe"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reader, err := DefaultFormEncoder.Encode(tc.in)
			require.NoError(t, err)

			body, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(body))
		})
	}
}

func TestFormEncoderUnsupportedType(t *testing.T) {
	t.Parallel()

	_, err := DefaultFormEncoder.Encode(make(chan string))
	assert.ErrorIs(t, err, ErrEncodingFailed)
}

func TestFiles(t *testing.T) {
	t.Parallel()

	server := startFileUploadServer()
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})

	fileContent1 := strings.NewReader("File content 1")
	fileContent2 := strings.NewReader("File content 2")

	resp, err := client.Post("/").
		Files(
			&File{Name: "file1", FileName: "test1.txt", Content: io.NopCloser(fileContent1)},
			&File{Name: "file2", FileName: "test2.txt", Content: io.NopCloser(fileContent2)},
		).
		Send(context.Background())
	assert.NoError(t, err, "No error expected on sending request")

	var uploads map[string][]string
	err = resp.ScanJSON(&uploads)
	assert.NoError(t, err, "Expect no error on parsing response")

	// Validate the file uploads
	assert.Contains(t, uploads, "file1", "file1 should be present in the uploads")
	assert.Contains(t, uploads, "file2", "file2 should be present in the uploads")
	// Optionally check for specific file content snippets
}
func TestFile(t *testing.T) {
	t.Parallel()

	server := startFileUploadServer() // Start the mock file upload server
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})

	// Simulate a file's content
	fileContent := strings.NewReader("This is the file content")

	// Send a request with a single file
	resp, err := client.Post("/").
		File("file", "single.txt", io.NopCloser(fileContent)).
		Send(context.Background())
	assert.NoError(t, err, "No error expected on sending request")

	// Parse the server's JSON response
	var uploads map[string][]string
	err = resp.ScanJSON(&uploads)
	assert.NoError(t, err, "Expect no error on parsing response")

	// Check if the server received the file correctly
	assert.Contains(t, uploads, "file", "The file should be present in the uploads")
	assert.Contains(t, uploads["file"][0], "single.txt", "The file name should be correctly received")
}

func TestDelFile(t *testing.T) {
	t.Parallel()

	server := startFileUploadServer() // Start the mock file upload server
	defer server.Close()

	client := Create(&Config{BaseURL: server.URL})

	// Simulate file contents
	fileContent1 := strings.NewReader("File content 1")
	fileContent2 := strings.NewReader("File content 2")

	// Prepare the request with two files, then delete one before sending
	resp, err := client.Post("/").
		Files(
			&File{Name: "file1", FileName: "file1.txt", Content: io.NopCloser(fileContent1)},
			&File{Name: "file2", FileName: "file2.txt", Content: io.NopCloser(fileContent2)},
		).
		DelFile("file1"). // Remove the first file
		Send(context.Background())
	assert.NoError(t, err, "No error expected on sending request")

	// Parse the server's JSON response
	var uploads map[string][]string
	err = resp.ScanJSON(&uploads)
	assert.NoError(t, err, "Expect no error on parsing response")

	// Validate that only the second file was uploaded
	assert.NotContains(t, uploads, "file1", "file1 should have been removed from the uploads")
	assert.Contains(t, uploads, "file2", "file2 should be present in the uploads")
}

func TestMultipartBuilder(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fileHeader := r.MultipartForm.File["avatar"][0]
		assert.Equal(t, "avatar.txt", fileHeader.Filename)
		assert.Equal(t, "text/plain", fileHeader.Header.Get("Content-Type"))
		if diff := cmp.Diff([]string{"alice"}, r.MultipartForm.Value["user"]); diff != "" {
			t.Errorf("multipart form field mismatch (-want +got):\n%s", diff)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	body := NewMultipart().
		Field("user", "alice").
		Part(FilePart{
			Field:       "avatar",
			Filename:    "avatar.txt",
			ContentType: "text/plain",
			Body:        strings.NewReader("hello"),
		})

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Post("/").Multipart(body).Send(t.Context())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
}

func TestMultipartReplayableBuffersBody(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if diff := cmp.Diff([]string{"alice"}, r.MultipartForm.Value["user"]); diff != "" {
			t.Errorf("multipart form field mismatch (-want +got):\n%s", diff)
		}
		requestCount++
		if requestCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	body := NewMultipart().
		Field("user", "alice").
		FileString("avatar", "avatar.txt", "hello").
		Replayable(1024)

	client := Create(&Config{
		BaseURL:       server.URL,
		MaxRetries:    1,
		RetryStrategy: DefaultBackoffStrategy(0),
	})
	resp, err := client.Post("/").Multipart(body).Send(t.Context())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode())
	assert.Equal(t, 2, requestCount)
	assert.Equal(t, 2, resp.Attempts())
}

func TestMultipartFileBytesUsesSnapshotAndCustomBoundary(t *testing.T) {
	t.Parallel()

	data := []byte("hello")
	body := NewMultipart().
		Boundary("requests-boundary").
		FileBytes("avatar", "avatar.txt", data).
		Replayable(1024)
	data[0] = 'j'

	reader, contentType, err := body.reader()
	require.NoError(t, err)
	assert.Contains(t, contentType, "boundary=requests-boundary")

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.com/upload", reader)
	require.NoError(t, err)
	request.Header.Set("Content-Type", contentType)
	require.NoError(t, request.ParseMultipartForm(1024))

	file, _, err := request.FormFile("avatar")
	require.NoError(t, err)
	defer file.Close() //nolint:errcheck

	got, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

func TestMultipartReportsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body *Multipart
	}{
		{
			name: "replay limit exceeded",
			body: NewMultipart().
				FileString("avatar", "avatar.txt", strings.Repeat("x", 20)).
				Replayable(10),
		},
		{
			name: "negative replay limit",
			body: NewMultipart().
				FileString("avatar", "avatar.txt", "hello").
				Replayable(-1),
		},
		{
			name: "missing part field",
			body: NewMultipart().
				Part(FilePart{Filename: "avatar.txt", Body: strings.NewReader("hello")}).
				Replayable(1024),
		},
		{
			name: "missing part body",
			body: NewMultipart().
				Part(FilePart{Field: "avatar", Filename: "avatar.txt"}).
				Replayable(1024),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := tc.body.reader()
			assert.ErrorIs(t, err, ErrInvalidConfigValue)
		})
	}
}

func TestMultipartReportsInvalidBoundary(t *testing.T) {
	t.Parallel()

	body := NewMultipart().
		Boundary(strings.Repeat("x", 71)).
		FileString("avatar", "avatar.txt", "hello")

	_, _, err := body.reader()
	require.Error(t, err)
}
