package requests

// maxStreamBufferSize is the maximum size of the buffer used for streaming.
const maxStreamBufferSize = 512 * 1024

// StreamCallback is a callback function that is called when data is received.
type StreamCallback func([]byte) error

// StreamErrCallback is a callback function that is called when an error occurs.
type StreamErrCallback func(error)

// StreamDoneCallback is a callback function that is called when the stream is done.
type StreamDoneCallback func()
