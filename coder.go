package requests

import (
	"io"
)

// Encoder is the interface that wraps the Encode method.
type Encoder interface {
	Encode(v any) (io.Reader, error)
	ContentType() string
}

// Decoder is the interface that wraps the Decode method.
type Decoder interface {
	Decode(r io.Reader, v any) error
}
