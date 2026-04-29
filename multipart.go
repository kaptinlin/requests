package requests

import (
	"bytes"
	"fmt"
	"io"
	mimeMultipart "mime/multipart"
	"net/textproto"
	"net/url"
	"strings"
)

// Multipart builds a multipart/form-data request body.
type Multipart struct {
	fields         url.Values
	parts          []FilePart
	boundary       string
	canReplay      bool
	replayMaxBytes int64
}

// FilePart describes one multipart file part.
type FilePart struct {
	Field       string
	Filename    string
	ContentType string
	Body        io.Reader
}

// NewMultipart creates an empty multipart builder.
func NewMultipart() *Multipart {
	return &Multipart{
		fields: url.Values{},
	}
}

// Field adds a form field.
func (m *Multipart) Field(name, value string) *Multipart {
	if m.fields == nil {
		m.fields = url.Values{}
	}
	m.fields.Add(name, value)
	return m
}

// File adds a file part.
func (m *Multipart) File(field, filename string, r io.Reader) *Multipart {
	return m.Part(FilePart{
		Field:    field,
		Filename: filename,
		Body:     r,
	})
}

// Part adds a file part with explicit metadata.
func (m *Multipart) Part(part FilePart) *Multipart {
	m.parts = append(m.parts, part)
	return m
}

// FileBytes adds a file part backed by bytes.
func (m *Multipart) FileBytes(field, filename string, data []byte) *Multipart {
	return m.File(field, filename, bytes.NewReader(bytes.Clone(data)))
}

// FileString adds a file part backed by a string.
func (m *Multipart) FileString(field, filename, data string) *Multipart {
	return m.File(field, filename, strings.NewReader(data))
}

// Boundary sets the multipart boundary.
func (m *Multipart) Boundary(boundary string) *Multipart {
	m.boundary = boundary
	return m
}

// Replayable buffers the multipart body up to maxBytes so it can be replayed for retries.
func (m *Multipart) Replayable(maxBytes int64) *Multipart {
	m.canReplay = true
	m.replayMaxBytes = maxBytes
	return m
}

func (m *Multipart) reader() (io.Reader, string, error) {
	if m.canReplay {
		return m.bufferedReader()
	}
	return m.streamingReader()
}

func (m *Multipart) streamingReader() (io.Reader, string, error) {
	reader, writer := io.Pipe()
	multipartWriter := mimeMultipart.NewWriter(writer)
	if err := m.setBoundary(multipartWriter); err != nil {
		_ = writer.CloseWithError(err)
		return nil, "", err
	}

	go func() {
		err := m.writeTo(multipartWriter)
		if closeErr := multipartWriter.Close(); err == nil {
			err = closeErr
		}
		_ = writer.CloseWithError(err)
	}()

	return reader, multipartWriter.FormDataContentType(), nil
}

func (m *Multipart) bufferedReader() (io.Reader, string, error) {
	if m.replayMaxBytes < 0 {
		return nil, "", fmt.Errorf("%w: multipart replay maxBytes", ErrInvalidConfigValue)
	}

	var buf bytes.Buffer
	limited := &limitWriter{writer: &buf, max: m.replayMaxBytes}
	multipartWriter := mimeMultipart.NewWriter(limited)
	if err := m.setBoundary(multipartWriter); err != nil {
		return nil, "", err
	}
	if err := m.writeTo(multipartWriter); err != nil {
		return nil, "", err
	}
	if err := multipartWriter.Close(); err != nil {
		return nil, "", err
	}
	return bytes.NewReader(buf.Bytes()), multipartWriter.FormDataContentType(), nil
}

func (m *Multipart) setBoundary(writer *mimeMultipart.Writer) error {
	if m.boundary == "" {
		return nil
	}
	if err := writer.SetBoundary(m.boundary); err != nil {
		return fmt.Errorf("setting custom boundary failed: %w", err)
	}
	return nil
}

func (m *Multipart) writeTo(writer *mimeMultipart.Writer) error {
	for key, values := range m.fields {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return fmt.Errorf("writing form field failed: %w", err)
			}
		}
	}

	for _, part := range m.parts {
		if err := writeFilePart(writer, part); err != nil {
			return err
		}
	}

	return nil
}

func writeFilePart(writer *mimeMultipart.Writer, part FilePart) error {
	if part.Field == "" {
		return fmt.Errorf("%w: multipart field", ErrInvalidConfigValue)
	}
	if part.Body == nil {
		return fmt.Errorf("%w: multipart body", ErrInvalidConfigValue)
	}

	writerPart, err := createFilePart(writer, part)
	if err != nil {
		return err
	}
	if _, err := io.Copy(writerPart, part.Body); err != nil {
		return fmt.Errorf("copying file content failed: %w", err)
	}
	if closer, ok := part.Body.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			return fmt.Errorf("closing file content failed: %w", err)
		}
	}
	return nil
}

func createFilePart(writer *mimeMultipart.Writer, part FilePart) (io.Writer, error) {
	if part.ContentType == "" {
		writerPart, err := writer.CreateFormFile(part.Field, part.Filename)
		if err != nil {
			return nil, fmt.Errorf("creating form file failed: %w", err)
		}
		return writerPart, nil
	}

	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(
		`form-data; name="%s"; filename="%s"`,
		escapeMultipartQuote(part.Field),
		escapeMultipartQuote(part.Filename),
	))
	header.Set("Content-Type", part.ContentType)

	writerPart, err := writer.CreatePart(header)
	if err != nil {
		return nil, fmt.Errorf("creating form file failed: %w", err)
	}
	return writerPart, nil
}

func escapeMultipartQuote(s string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(s)
}

type limitWriter struct {
	writer io.Writer
	max    int64
	wrote  int64
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if w.max >= 0 && w.wrote+int64(len(p)) > w.max {
		return 0, fmt.Errorf("%w: multipart replay body exceeds %d bytes", ErrInvalidConfigValue, w.max)
	}
	n, err := w.writer.Write(p)
	w.wrote += int64(n)
	return n, err
}
