package requests

import (
	"context"
	"errors"
	"net"
)

var (
	// ErrUnsupportedContentType is returned when the content type is unsupported.
	ErrUnsupportedContentType = errors.New("unsupported content type")

	// ErrUnsupportedDataType is returned when the data type is unsupported.
	ErrUnsupportedDataType = errors.New("unsupported data type")

	// ErrEncodingFailed is returned when the encoding fails.
	ErrEncodingFailed = errors.New("encoding failed")

	// ErrRequestCreationFailed is returned when the request cannot be created.
	ErrRequestCreationFailed = errors.New("failed to create request")

	// ErrResponseReadFailed is returned when the response cannot be read.
	ErrResponseReadFailed = errors.New("failed to read response")

	// ErrUnsupportedScheme is returned when the proxy scheme is unsupported.
	ErrUnsupportedScheme = errors.New("unsupported proxy scheme")

	// ErrNoProxies is returned when no proxy URLs are provided to a rotation function.
	ErrNoProxies = errors.New("no proxy URLs provided")

	// ErrUnsupportedFormFieldsType is returned when the form fields type is unsupported.
	ErrUnsupportedFormFieldsType = errors.New("unsupported form fields type")

	// ErrNotSupportSaveMethod is returned when the provided type for saving is not supported.
	ErrNotSupportSaveMethod = errors.New("unsupported save type")

	// ErrInvalidTransportType is returned when the transport type is invalid.
	ErrInvalidTransportType = errors.New("invalid transport type")

	// ErrResponseNil is returned when the response is nil.
	ErrResponseNil = errors.New("response is nil")

	// ErrAutoRedirectDisabled is returned when the auto redirect is disabled.
	ErrAutoRedirectDisabled = errors.New("auto redirect disabled")

	// ErrTooManyRedirects is returned when the number of redirects is too many.
	ErrTooManyRedirects = errors.New("too many redirects")

	// ErrRedirectNotAllowed is returned when the redirect is not allowed.
	ErrRedirectNotAllowed = errors.New("redirect not allowed")

	// ErrTestTimeout is returned when a test request times out (used in tests).
	ErrTestTimeout = errors.New("test timeout: request took too long")
)

// IsTimeout reports whether err is or wraps a timeout error.
// It checks for context.DeadlineExceeded and net.Error timeout errors.
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// IsConnectionError reports whether err is a connection-level failure
// (DNS resolution, TCP connect, TLS handshake).
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}
