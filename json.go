package requests

import (
	"bytes"
	"encoding/json"
	"io"
)

type JSONEncoder struct {
	MarshalFunc func(v any) ([]byte, error)
}

func (e *JSONEncoder) Encode(v any) (io.Reader, error) {
	var err error
	var data []byte

	if e.MarshalFunc == nil {
		data, err = json.Marshal(v) // Fallback to standard JSON marshal if no custom function is provided
	} else {
		data, err = e.MarshalFunc(v)
	}

	if err != nil {
		return nil, err
	}

	buf := GetBuffer()
	_, err = buf.Write(data)
	if err != nil {
		PutBuffer(buf) // Ensure the buffer is returned to the pool in case of an error
		return nil, err
	}

	// Here, we need to ensure the buffer will be returned to the pool after being read.
	// One approach is to wrap the bytes.Reader in a custom type that returns the buffer on close.
	reader := &poolReader{Reader: bytes.NewReader(buf.B), poolBuf: buf}
	return reader, nil
}

func (e *JSONEncoder) ContentType() string {
	return "application/json;charset=utf-8"
}

var DefaultJSONEncoder = &JSONEncoder{
	MarshalFunc: json.Marshal,
}

type JSONDecoder struct {
	UnmarshalFunc func(data []byte, v any) error
}

func (d *JSONDecoder) Decode(r io.Reader, v any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if d.UnmarshalFunc != nil {
		return d.UnmarshalFunc(data, v)
	}

	return json.Unmarshal(data, v) // Fallback to standard JSON unmarshal
}

var DefaultJSONDecoder = &JSONDecoder{
	UnmarshalFunc: json.Unmarshal,
}
