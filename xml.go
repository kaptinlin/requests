package requests

import (
	"bytes"
	"encoding/xml"
	"io"
)

type XMLEncoder struct {
	MarshalFunc func(v any) ([]byte, error)
}

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

func (e *XMLEncoder) ContentType() string {
	return "application/xml;charset=utf-8"
}

var DefaultXMLEncoder = &XMLEncoder{
	MarshalFunc: xml.Marshal,
}

type XMLDecoder struct {
	UnmarshalFunc func(data []byte, v any) error
}

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

var DefaultXMLDecoder = &XMLDecoder{
	UnmarshalFunc: xml.Unmarshal,
}
