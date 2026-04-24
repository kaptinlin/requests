package requests

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

// XMLEncoder handles encoding of XML data.
type XMLEncoder struct {
	MarshalFunc func(v any) ([]byte, error) // MarshalFunc marshals a value into XML.
}

// Encode marshals the provided value into XML format.
func (e *XMLEncoder) Encode(v any) (io.Reader, error) {
	marshal := xml.Marshal
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
		return nil, fmt.Errorf("failed to write XML to buffer: %w", err)
	}

	return &poolReader{Reader: bytes.NewReader(buf.B), poolBuf: buf}, nil
}

// ContentType returns the content type for XML data.
func (e *XMLEncoder) ContentType() string {
	return "application/xml;charset=utf-8"
}

// DefaultXMLEncoder is the default XMLEncoder instance using the standard xml.Marshal function.
var DefaultXMLEncoder = &XMLEncoder{
	MarshalFunc: xml.Marshal,
}

// XMLDecoder handles decoding of XML data.
type XMLDecoder struct {
	UnmarshalFunc func(data []byte, v any) error // UnmarshalFunc unmarshals XML data into a value.
}

// Decode reads the data from the reader and unmarshals it into the provided value.
func (d *XMLDecoder) Decode(r io.Reader, v any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read XML data: %w", err)
	}

	unmarshal := xml.Unmarshal
	if d.UnmarshalFunc != nil {
		unmarshal = d.UnmarshalFunc
	}

	if err := unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal XML: %w", err)
	}
	return nil
}

// DefaultXMLDecoder is the default XMLDecoder instance using the standard xml.Unmarshal function.
var DefaultXMLDecoder = &XMLDecoder{
	UnmarshalFunc: xml.Unmarshal,
}
