# Stream Support

The Requests library offers comprehensive support for streaming HTTP responses, enabling real-time data processing for applications requiring live updates, such as feeds, notifications, or other continuously updated data sources.

## Table of Contents

1. [Stream Callbacks](#stream-callbacks)
2. [Configuring Stream Callbacks](#configuring-stream-callbacks)
3. [Handling Stream Errors](#handling-stream-errors)
4. [Completing Stream Processing](#completing-stream-processing)
5. [Example: Consuming an SSE Stream](#example-consuming-an-sse-stream)

### Stream Callbacks

Stream callbacks are functions that you define to handle chunks of data as they are received from the server. The Requests library supports three types of stream callbacks:

- **StreamCallback**: Invoked for each chunk of data received.
- **StreamErrCallback**: Invoked when an error occurs during streaming.
- **StreamDoneCallback**: Invoked once streaming is completed, regardless of whether it ended due to an error or successfully.

### Configuring Stream Callbacks

To configure streaming for a request, use the `Stream` method on a `RequestBuilder` instance. This method accepts a `StreamCallback` function, which will be called with each chunk of data received from the server.

```go
streamCallback := func(data []byte) error {
    fmt.Println("Received stream data:", string(data))
    return nil // Return an error if needed to stop streaming
}

request := client.Get("/stream-endpoint").Stream(streamCallback)
```

### Handling Stream Errors

To handle errors that occur during streaming, set a `StreamErrCallback` using the `StreamErr` method on the `Response` object.

```go
streamErrCallback := func(err error) {
    fmt.Printf("Stream error: %v\n", err)
}

response, _ := request.Send(context.Background())
response.StreamErr(streamErrCallback)
```

### Completing Stream Processing

Once streaming is complete, you can use the `StreamDone` method on the `Response` object to set a `StreamDoneCallback`. This callback is invoked after the stream is fully processed, either successfully or due to an error.

```go
streamDoneCallback := func() {
    fmt.Println("Stream processing completed")
}

response.StreamDone(streamDoneCallback)
```

### Example: Consuming an SSE Stream

The following example demonstrates how to consume a Server-Sent Events (SSE) stream, processing each event as it arrives, handling errors, and performing cleanup once the stream ends.

```go
// Configure the stream callback to handle data chunks
streamCallback := func(data []byte) error {
    fmt.Println("Received stream event:", string(data))
    return nil
}

// Configure error and done callbacks
streamErrCallback := func(err error) {
    fmt.Printf("Error during streaming: %v\n", err)
}

streamDoneCallback := func() {
    fmt.Println("Stream ended")
}

// Create the streaming request
client := requests.Create(&requests.Config{BaseURL: "https://example.com"})
request := client.Get("/events").Stream(streamCallback)

// Send the request and configure callbacks
response, err := request.Send(context.Background())
if err != nil {
    fmt.Printf("Failed to start streaming: %v\n", err)
    return
}

response.StreamErr(streamErrCallback).StreamDone(streamDoneCallback)
```
