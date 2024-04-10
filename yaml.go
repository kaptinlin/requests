package requests

import (
	"bytes"
	"io"

	"github.com/goccy/go-yaml"
)

type YAMLEncoder struct {
	MarshalFunc func(v any) ([]byte, error)
}

func (e *YAMLEncoder) Encode(v any) (io.Reader, error) {
	var err error
	var data []byte

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

func (e *YAMLEncoder) ContentType() string {
	return "application/yaml;charset=utf-8"
}

// DefaultYAMLEncoder instance using the goccy/go-yaml Marshal function
var DefaultYAMLEncoder = &YAMLEncoder{
	MarshalFunc: yaml.Marshal,
}

type YAMLDecoder struct {
	UnmarshalFunc func(data []byte, v any) error
}

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

// DefaultYAMLDecoder instance using the goccy/go-yaml Unmarshal function
var DefaultYAMLDecoder = &YAMLDecoder{
	UnmarshalFunc: yaml.Unmarshal,
}
