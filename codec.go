package requests

import (
	"io"
)

// Encoder encodes values into an io.Reader with an associated content type.
type Encoder interface {
	// Encode encodes v and returns a reader for the encoded payload.
	Encode(v any) (io.Reader, error)
	// ContentType returns the payload content type.
	ContentType() string
}

// Decoder decodes data from an io.Reader into a value.
type Decoder interface {
	// Decode decodes data from r into v.
	Decode(r io.Reader, v any) error
}
