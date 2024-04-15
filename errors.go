package requests

import (
	"errors"
)

// ErrUnsupportedContentType is returned when the content type is unsupported.
var ErrUnsupportedContentType = errors.New("unsupported content type")

// ErrUnsupportedDataType is returned when the data type is unsupported.
var ErrUnsupportedDataType = errors.New("unsupported data type")

// ErrEncodingFailed is returned when the encoding fails.
var ErrEncodingFailed = errors.New("encoding failed")

// ErrRequestCreationFailed is returned when the request cannot be created.
var ErrRequestCreationFailed = errors.New("failed to create request")

// ErrResponseReadFailed is returned when the response cannot be read.
var ErrResponseReadFailed = errors.New("failed to read response")

// ErrUnsupportedScheme is returned when the proxy scheme is unsupported.
var ErrUnsupportedScheme = errors.New("unsupported proxy scheme")

// ErrUnsupportedFormFieldsType is returned when the form fields type is unsupported.
var ErrUnsupportedFormFieldsType = errors.New("unsupported form fields type")

// ErrNotSupportSaveMethod is returned when the provided type for saving is not supported.
var ErrNotSupportSaveMethod = errors.New("the provided type for saving is not supported")

// ErrInvalidTransportType is returned when the transport type is invalid.
var ErrInvalidTransportType = errors.New("invalid transport type")

// ErrResponseNil is returned when the response is nil.
var ErrResponseNil = errors.New("response is nil")
