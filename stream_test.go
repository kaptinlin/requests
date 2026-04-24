package requests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

func TestStream(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := range 3 {
			_, _ = fmt.Fprintf(w, "data: Message %d\n", i)
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()

	doneCh := make(chan struct{})
	dataReceived := make([]string, 0, 3)

	client := Create(&Config{BaseURL: server.URL})
	_, err := client.Get("/").Stream(func(data []byte) error {
		dataReceived = append(dataReceived, string(data))
		assert.Contains(t, string(data), "data: Message")
		return nil
	}).StreamErr(func(err error) {
		assert.NoError(t, err)
	}).StreamDone(func() {
		close(doneCh)
	}).Send(context.Background())
	require.NoError(t, err)

	select {
	case <-doneCh:
	case <-time.After(time.Second):
		t.Fatal("expected stream callbacks to finish")
	}
	assert.Equal(t, 3, len(dataReceived))
}
