package requests

import (
	"bytes"
	"io"

	"github.com/goccy/go-yaml"
)

// YAMLEncoder handles encoding of YAML data.
type YAMLEncoder struct {
	MarshalFunc func(v any) ([]byte, error)
}

// Encode marshals the provided value into YAML format.
func (e *YAMLEncoder) Encode(v any) (io.Reader, error) {
	var data []byte
	var err error

	if e.MarshalFunc != nil {
		data, err = e.MarshalFunc(v)
	} else {
		// Use goccy/go-yaml for marshaling by default
		data, err = yaml.Marshal(v)
	}

	if err != nil {
		return nil, err
	}

	buf := GetBuffer()
	_, err = buf.Write(data)
	if err != nil {
		PutBuffer(buf)
		return nil, err
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
	UnmarshalFunc func(data []byte, v any) error
}

// Decode reads the data from the reader and unmarshals it into the provided value.
func (d *YAMLDecoder) Decode(r io.Reader, v any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if d.UnmarshalFunc != nil {
		return d.UnmarshalFunc(data, v)
	}

	// Fallback to standard YAML unmarshal using goccy/go-yaml
	return yaml.Unmarshal(data, v)
}

// DefaultYAMLDecoder is the default YAMLDecoder instance using the goccy/go-yaml Unmarshal function.
var DefaultYAMLDecoder = &YAMLDecoder{
	UnmarshalFunc: yaml.Unmarshal,
}
