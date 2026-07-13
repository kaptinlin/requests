package requests

import "github.com/valyala/bytebufferpool"

var bufferPool bytebufferpool.Pool

func getBuffer() *bytebufferpool.ByteBuffer {
	return bufferPool.Get()
}

func putBuffer(b *bytebufferpool.ByteBuffer) {
	bufferPool.Put(b)
}
