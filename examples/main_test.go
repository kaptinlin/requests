package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

func TestRunLogsFetchedPost(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"userId":7,"id":42,"title":"example","body":"body"}`))
	}))
	t.Cleanup(server.Close)

	var out bytes.Buffer
	err := run(t.Context(), server.URL, "42", log.New(&out, "", 0))
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Post Received: {UserID:7 ID:42 Title:example Body:body}")
}

func TestRunReturnsFetchError(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := run(t.Context(), "http://127.0.0.1:0", "1", log.New(&out, "", 0))
	require.Error(t, err)
	assert.Empty(t, out.String())
}

func TestFetchPost(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/posts/42", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"userId":7,"id":42,"title":"example","body":"body"}`))
	}))
	t.Cleanup(server.Close)

	got, err := fetchPost(t.Context(), server.URL, "42")
	require.NoError(t, err)

	want := Post{UserID: 7, ID: 42, Title: "example", Body: "body"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("fetchPost() mismatch (-want +got):\n%s", diff)
	}
}

func TestFetchPostReturnsRequestError(t *testing.T) {
	t.Parallel()

	_, err := fetchPost(t.Context(), "http://127.0.0.1:0", "1")
	require.Error(t, err)
}

func TestFetchPostReturnsDecodeError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{`))
	}))
	t.Cleanup(server.Close)

	_, err := fetchPost(t.Context(), server.URL, "1")
	require.Error(t, err)
}
