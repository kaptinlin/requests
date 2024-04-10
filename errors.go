package requests

import (
	"errors"
)

var ErrUnsupportedContentType = errors.New("unsupported content type")
var ErrUnsupportedDataType = errors.New("unsupported data type")
var ErrEncodingFailed = errors.New("encoding failed")
var ErrRequestCreationFailed = errors.New("failed to create request")
var ErrResponseReadFailed = errors.New("failed to read response")
var ErrUnsupportedScheme = errors.New("unsupported proxy scheme")
var ErrUnsupportedFormFieldsType = errors.New("unsupported form fields type")
var ErrNotSupportSaveMethod = errors.New("the provided type for saving is not supported")
