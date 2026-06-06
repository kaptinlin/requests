package requests

import (
	"bytes"
	"fmt"
	"io"

	"github.com/goccy/go-yaml"
)

// YAMLEncoder handles encoding of YAML data.
type YAMLEncoder struct {
	MarshalFunc func(v any) ([]byte, error) // MarshalFunc marshals a value into YAML.
}

// Encode marshals the provided value into YAML format.
func (e *YAMLEncoder) Encode(v any) (io.Reader, error) {
	marshal := yaml.Marshal
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
		return nil, fmt.Errorf("failed to write YAML to buffer: %w", err)
	}

	return &poolReader{Reader: bytes.NewReader(buf.B), poolBuf: buf}, nil
}

// ContentType returns the content type for YAML data.
func (e *YAMLEncoder) ContentType() string {
	return "application/yaml;charset=utf-8"
}

// DefaultYAMLEncoder is the default YAMLEncoder instance using the goccy/go-yaml Marshal function.
var DefaultYAMLEncoder = &YAMLEncoder{
	MarshalFunc: yaml.Marshal,
}

// YAMLDecoder handles decoding of YAML data.
type YAMLDecoder struct {
	DecodeFunc func(r io.Reader, v any) error // DecodeFunc decodes YAML data into a value.
}

// Decode decodes YAML data from the reader into the provided value.
func (d *YAMLDecoder) Decode(r io.Reader, v any) error {
	if d.DecodeFunc != nil {
		return d.DecodeFunc(r, v)
	}

	if err := yaml.NewDecoder(r).Decode(v); err != nil {
		return fmt.Errorf("failed to decode YAML: %w", err)
	}
	return nil
}

// DefaultYAMLDecoder is the default YAMLDecoder instance using the goccy/go-yaml streaming decoder.
var DefaultYAMLDecoder = &YAMLDecoder{}
