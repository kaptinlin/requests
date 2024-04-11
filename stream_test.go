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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 3; i++ {
			fmt.Fprintf(w, "data: Message %d\n\n", i)
			w.(http.Flusher).Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()

	defer server.Close()

	dataReceived := make([]string, 0)
	streamCallback := func(data []byte) error {
		dataReceived = append(dataReceived, string(data))
		return nil
	}

	var streamErr error
	streamErrCallback := func(err error) {
		streamErr = err
	}

	var streamDoneCalled bool
	streamDoneCallback := func() {
		streamDoneCalled = true
	}

	client := Create(&Config{BaseURL: server.URL})
	resp, err := client.Get("/").Stream(streamCallback).Send(context.Background())
	assert.NoError(t, err)

	resp.StreamErr(streamErrCallback)
	resp.StreamDone(streamDoneCallback)

	// Wait for the stream to be processed. Adjust the wait time based on expected stream duration.
	time.Sleep(1 * time.Second)

	assert.Greater(t, len(dataReceived), 0, "Should have received data")
	assert.Nil(t, streamErr, "Should not have errors")
	assert.True(t, streamDoneCalled, "StreamDoneCallback should be called")
}
