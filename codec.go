package requests

import (
	"io"
)

// Encoder encodes values into an io.Reader format with a specific content type.
type Encoder interface {
	Encode(v any) (io.Reader, error)
	ContentType() string
}

// Decoder decodes data from an io.Reader into a value.
type Decoder interface {
	Decode(r io.Reader, v any) error
}
