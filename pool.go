package requests

import (
	"bytes"

	"github.com/valyala/bytebufferpool"
)

var bufferPool bytebufferpool.Pool

// GetBuffer retrieves a buffer from the pool.
func GetBuffer() *bytebufferpool.ByteBuffer {
	return bufferPool.Get()
}

// PutBuffer returns a buffer to the pool.
func PutBuffer(b *bytebufferpool.ByteBuffer) {
	bufferPool.Put(b)
}

// poolReader wraps bytes.Reader to return the buffer to the pool when closed.
type poolReader struct {
	*bytes.Reader
	poolBuf *bytebufferpool.ByteBuffer
}

func (r *poolReader) Close() error {
	PutBuffer(r.poolBuf)
	return nil
}
