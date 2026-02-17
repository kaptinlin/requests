package requests

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/google/go-querystring/query"
)

// File represents a form file.
type File struct {
	Name     string        // Form field name
	FileName string        // File name
	Content  io.ReadCloser // File content
}

// SetContent sets the content of the file.
func (f *File) SetContent(content io.ReadCloser) {
	f.Content = content
}

// SetFileName sets the file name.
func (f *File) SetFileName(fileName string) {
	f.FileName = fileName
}

// SetName sets the form field name.
func (f *File) SetName(name string) {
	f.Name = name
}

// stringMapToValues converts a map[string]string to url.Values.
func stringMapToValues(data map[string]string) url.Values {
	values := make(url.Values, len(data))
	for key, value := range data {
		values.Set(key, value)
	}
	return values
}

// parseFormFields parses the given form fields into url.Values.
func parseFormFields(fields any) (url.Values, error) {
	switch data := fields.(type) {
	case url.Values:
		return data, nil
	case map[string][]string:
		return url.Values(data), nil
	case map[string]string:
		return stringMapToValues(data), nil
	default:
		values, err := query.Values(fields)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnsupportedFormFieldsType, err)
		}
		return values, nil
	}
}

// parseForm parses the given form data into url.Values and []*File.
func parseForm(v any) (url.Values, []*File, error) {
	switch data := v.(type) {
	case url.Values:
		return data, nil, nil
	case map[string][]string:
		return url.Values(data), nil, nil
	case map[string]string:
		return stringMapToValues(data), nil, nil
	case map[string]any:
		values := make(url.Values)
		var files []*File
		for key, value := range data {
			switch v := value.(type) {
			case string:
				values.Set(key, v)
			case []string:
				for _, v := range v {
					values.Add(key, v)
				}
			case *File:
				v.SetName(key)
				files = append(files, v)
			default:
				return nil, nil, fmt.Errorf("%w: %T", ErrUnsupportedDataType, value)
			}
		}
		return values, files, nil
	default:
		values, err := query.Values(v)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %w", ErrUnsupportedFormFieldsType, err)
		}
		return values, nil, nil
	}
}

// FormEncoder handles encoding of form data.
type FormEncoder struct{}

// Encode encodes the given value into URL-encoded form data.
func (e *FormEncoder) Encode(v any) (io.Reader, error) {
	switch data := v.(type) {
	case url.Values:
		return strings.NewReader(data.Encode()), nil
	case map[string][]string:
		return strings.NewReader(url.Values(data).Encode()), nil
	case map[string]string:
		return strings.NewReader(stringMapToValues(data).Encode()), nil
	default:
		values, err := query.Values(v)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrEncodingFailed, err)
		}
		return strings.NewReader(values.Encode()), nil
	}
}

// DefaultFormEncoder is the default FormEncoder instance.
var DefaultFormEncoder = &FormEncoder{}
