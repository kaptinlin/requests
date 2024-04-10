package requests

import (
	"io"
)

type Encoder interface {
	Encode(v any) (io.Reader, error)
	ContentType() string
}

type Decoder interface {
	Decode(r io.Reader, v any) error
}
