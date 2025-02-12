package requests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 3; i++ {
			_, _ = fmt.Fprintf(w, "data: Message %d\n", i)
			w.(http.Flusher).Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()
	doneCh := make(chan struct{})
	dataReceived := make([]string, 0)

	client := Create(&Config{BaseURL: server.URL})
	_, err := client.Get("/").Stream(func(data []byte) error {
		dataReceived = append(dataReceived, string(data))

		assert.Contains(t, string(data), "data: Message")
		return nil
	}).StreamErr(func(err error) {
		assert.NoError(t, err)
	}).StreamDone(func() {
		assert.Equal(t, 3, len(dataReceived))
		close(doneCh)
	}).Send(context.Background())
	assert.NoError(t, err)

	// Proper synchronization to ensure all callbacks have completed
	time.Sleep(1 * time.Second)
	<-doneCh

	assert.Equal(t, 3, len(dataReceived))
}
