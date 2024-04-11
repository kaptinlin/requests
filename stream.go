package requests

const maxStreamBufferSize = 512 * 1024

type StreamCallback func([]byte) error
type StreamErrCallback func(error)
type StreamDoneCallback func()
