package requests

import (
	"bytes"
	"fmt"
	"io"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

// JSONEncoder handles encoding of JSON data.
type JSONEncoder struct {
	MarshalFunc func(v any) ([]byte, error) // MarshalFunc marshals a value into JSON.
}

// Encode marshals the provided value into JSON format.
func (e *JSONEncoder) Encode(v any) (io.Reader, error) {
	marshal := jsonMarshal
	if e.MarshalFunc != nil {
		marshal = e.MarshalFunc
	}

	data, err := marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncodingFailed, err)
	}

	buf := GetBuffer()
	_, err = buf.Write(data)
	if err != nil {
		PutBuffer(buf)
		return nil, fmt.Errorf("failed to write JSON to buffer: %w", err)
	}

	return &poolReader{Reader: bytes.NewReader(buf.B), poolBuf: buf}, nil
}

// ContentType returns the content type for JSON data.
func (e *JSONEncoder) ContentType() string {
	return "application/json;charset=utf-8"
}

func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// DefaultJSONEncoder is the default JSONEncoder instance using the JSON v2 marshal function.
var DefaultJSONEncoder = &JSONEncoder{
	MarshalFunc: jsonMarshal,
}

// JSONDecoder handles decoding of JSON data.
type JSONDecoder struct {
	DecodeFunc func(r io.Reader, v any) error // DecodeFunc decodes JSON data into a value.
}

// Decode decodes JSON data from the reader into the provided value.
func (d *JSONDecoder) Decode(r io.Reader, v any) error {
	if d.DecodeFunc != nil {
		return d.DecodeFunc(r, v)
	}

	if err := json.UnmarshalDecode(jsontext.NewDecoder(r), v); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}
	return nil
}

// DefaultJSONDecoder is the default JSONDecoder instance using the JSON v2 streaming decoder.
var DefaultJSONDecoder = &JSONDecoder{}
