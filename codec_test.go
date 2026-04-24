package requests

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

type codecFixture struct {
	Name string `json:"name" xml:"name" yaml:"name"`
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errCodecRead
}

var errCodecRead = errors.New("codec read failed")

func TestCodecContentTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		encoder Encoder
		want    string
	}{
		{name: "json", encoder: DefaultJSONEncoder, want: "application/json;charset=utf-8"},
		{name: "xml", encoder: DefaultXMLEncoder, want: "application/xml;charset=utf-8"},
		{name: "yaml", encoder: DefaultYAMLEncoder, want: "application/yaml;charset=utf-8"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.encoder.ContentType())
		})
	}
}

func TestCodecEncodeDefaultMarshalers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		encoder Encoder
		want    string
	}{
		{name: "json", encoder: &JSONEncoder{}, want: `{"name":"Jane"}`},
		{name: "xml", encoder: &XMLEncoder{}, want: `<codecFixture><name>Jane</name></codecFixture>`},
		{name: "yaml", encoder: &YAMLEncoder{}, want: "name: Jane\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reader, err := tc.encoder.Encode(codecFixture{Name: "Jane"})
			require.NoError(t, err)
			body, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(body))
		})
	}
}

func TestCodecEncodeWrapsMarshalErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		encoder Encoder
	}{
		{name: "json", encoder: &JSONEncoder{}},
		{name: "xml", encoder: &XMLEncoder{}},
		{name: "yaml", encoder: &YAMLEncoder{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := tc.encoder.Encode(make(chan string))
			assert.ErrorIs(t, err, ErrEncodingFailed)
		})
	}
}

func TestCodecDecodeDefaultUnmarshalers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		decoder Decoder
		body    string
	}{
		{name: "json", decoder: &JSONDecoder{}, body: `{"name":"Jane"}`},
		{name: "xml", decoder: &XMLDecoder{}, body: `<codecFixture><name>Jane</name></codecFixture>`},
		{name: "yaml", decoder: &YAMLDecoder{}, body: "name: Jane\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got codecFixture
			err := tc.decoder.Decode(strings.NewReader(tc.body), &got)
			require.NoError(t, err)
			assert.Equal(t, "Jane", got.Name)
		})
	}
}

func TestCodecDecodeReadErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		decoder Decoder
	}{
		{name: "json", decoder: DefaultJSONDecoder},
		{name: "xml", decoder: DefaultXMLDecoder},
		{name: "yaml", decoder: DefaultYAMLDecoder},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.decoder.Decode(failingReader{}, &codecFixture{})
			assert.ErrorIs(t, err, errCodecRead)
		})
	}
}

func TestCodecDecodeUnmarshalErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		decoder Decoder
		body    string
	}{
		{name: "json", decoder: &JSONDecoder{}, body: `{`},
		{name: "xml", decoder: &XMLDecoder{}, body: `<`},
		{name: "yaml", decoder: &YAMLDecoder{}, body: `:`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.decoder.Decode(strings.NewReader(tc.body), &codecFixture{})
			require.Error(t, err)
		})
	}
}
