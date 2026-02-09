package requests

import (
	"bytes"
	"encoding/xml"
	"io"
)

// XMLEncoder handles encoding of XML data.
type XMLEncoder struct {
	MarshalFunc func(v any) ([]byte, error)
}

// Encode marshals the provided value into XML format.
func (e *XMLEncoder) Encode(v any) (io.Reader, error) {
	var err error
	var data []byte

	if e.MarshalFunc != nil {
		data, err = e.MarshalFunc(v)
	} else {
		data, err = xml.Marshal(v) // Fallback to standard XML marshal if no custom function is provided
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
	UnmarshalFunc func(data []byte, v any) error
}

// Decode reads the data from the reader and unmarshals it into the provided value.
func (d *XMLDecoder) Decode(r io.Reader, v any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if d.UnmarshalFunc != nil {
		return d.UnmarshalFunc(data, v)
	}

	return xml.Unmarshal(data, v) // Fallback to standard XML unmarshal
}

// DefaultXMLDecoder is the default XMLDecoder instance using the standard xml.Unmarshal function.
var DefaultXMLDecoder = &XMLDecoder{
	UnmarshalFunc: xml.Unmarshal,
}
