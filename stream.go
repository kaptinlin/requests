package requests

// MaxStreamBufferSize is the maximum size of the buffer used for streaming.
const MaxStreamBufferSize = 512 * 1024

// StreamCallback handles a streamed response chunk.
type StreamCallback func([]byte) error

// StreamErrCallback handles a streaming read error.
type StreamErrCallback func(error)

// StreamDoneCallback runs after streaming finishes.
type StreamDoneCallback func()
