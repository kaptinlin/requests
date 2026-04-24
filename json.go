package requests

import (
	"bytes"
	"fmt"
	"io"

	"github.com/go-json-experiment/json"
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

// jsonMarshal wraps JSON v2 marshal to match the expected signature.
func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// DefaultJSONEncoder is the default JSONEncoder instance using the JSON v2 marshal function.
var DefaultJSONEncoder = &JSONEncoder{
	MarshalFunc: jsonMarshal,
}

// JSONDecoder handles decoding of JSON data.
type JSONDecoder struct {
	UnmarshalFunc func(data []byte, v any) error // UnmarshalFunc unmarshals JSON data into a value.
}

// Decode reads the data from the reader and unmarshals it into the provided value.
func (d *JSONDecoder) Decode(r io.Reader, v any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read JSON data: %w", err)
	}

	unmarshal := jsonUnmarshal
	if d.UnmarshalFunc != nil {
		unmarshal = d.UnmarshalFunc
	}

	if err := unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return nil
}

// jsonUnmarshal wraps JSON v2 unmarshal to match the expected signature.
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// DefaultJSONDecoder is the default JSONDecoder instance using the JSON v2 unmarshal function.
var DefaultJSONDecoder = &JSONDecoder{
	UnmarshalFunc: jsonUnmarshal,
}
