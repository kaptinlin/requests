package requests

import (
	"fmt"
	"net/url"

	"github.com/google/go-querystring/query"
)

func stringMapToValues(data map[string]string) url.Values {
	values := make(url.Values, len(data))
	for key, value := range data {
		values.Set(key, value)
	}
	return values
}

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
