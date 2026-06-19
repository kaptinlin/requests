package requests

import (
	"bytes"

	"github.com/valyala/bytebufferpool"
)

var bufferPool bytebufferpool.Pool

func getBuffer() *bytebufferpool.ByteBuffer {
	return bufferPool.Get()
}

func putBuffer(b *bytebufferpool.ByteBuffer) {
	bufferPool.Put(b)
}

// poolReader wraps bytes.Reader to return the buffer to the pool when closed.
type poolReader struct {
	*bytes.Reader
	poolBuf *bytebufferpool.ByteBuffer
}

func (r *poolReader) Close() error {
	putBuffer(r.poolBuf)
	return nil
}
