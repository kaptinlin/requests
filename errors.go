package requests

import (
	"context"
	"errors"
	"net"
)

// Sentinel errors returned by the package. Use [errors.Is] to match against
// them. The classification helpers below ([IsTimeout], [IsCanceled],
// [IsConnectionError]) cover the most common transport-level questions
// without forcing callers to enumerate every cause.
var (
	// ErrUnsupportedContentType is returned when a body encoder cannot handle
	// the request's Content-Type, or when a raw body is given a value type
	// other than string, []byte, or io.Reader. Set Content-Type explicitly or
	// use a typed body setter (JSONBody, XMLBody, YAMLBody, TextBody, RawBody).
	ErrUnsupportedContentType = errors.New("unsupported content type")

	// ErrUnsupportedDataType is returned when a decoder cannot handle the
	// destination type. Pass a pointer to a struct, map, or slice that the
	// codec can populate.
	ErrUnsupportedDataType = errors.New("unsupported data type")

	// ErrEncodingFailed is returned when an encoder rejects a body value.
	// Wrapped errors carry the codec-specific cause; inspect with errors.As.
	ErrEncodingFailed = errors.New("encoding failed")

	// ErrRequestCreationFailed is returned when http.NewRequestWithContext
	// rejects the resolved method, URL, or body. Almost always indicates a
	// malformed BaseURL + path combination; verify with errors.As to surface
	// the underlying *url.Error.
	ErrRequestCreationFailed = errors.New("failed to create request")

	// ErrRequestBodyNotReplayable is returned when retries are configured but
	// the body is a one-shot io.Reader (see [RequestBuilder.Body]). Switch to
	// a buffered body setter such as JSONBody, RawBody, Form, or call
	// Multipart.Replayable(maxBytes) to opt in to buffering.
	ErrRequestBodyNotReplayable = errors.New("request body is not replayable")

	// ErrRequestBodyReadIncomplete is returned when a sized request body
	// (ReadAt+Seek+Size) reports fewer bytes than its declared size. Indicates
	// a truncated source; do not retry until the source is fixed.
	ErrRequestBodyReadIncomplete = errors.New("request body read incomplete")

	// ErrResponseReadFailed is returned when the response body cannot be read
	// in full. Wrapped errors carry the I/O cause.
	ErrResponseReadFailed = errors.New("failed to read response")

	// ErrUnsupportedScheme is returned when a proxy URL uses a scheme other
	// than http, https, socks5, or socks5h.
	ErrUnsupportedScheme = errors.New("unsupported proxy scheme")

	// ErrNoProxies is returned when a proxy rotation function is given an
	// empty list of proxy URLs.
	ErrNoProxies = errors.New("no proxy URLs provided")

	// ErrUnsupportedFormFieldsType is returned when [RequestBuilder.FormFields]
	// or [RequestBuilder.Form] receives a value that is not a struct, map, or
	// url.Values.
	ErrUnsupportedFormFieldsType = errors.New("unsupported form fields type")

	// ErrNotSupportSaveMethod is returned when [Response.Save] is given a
	// destination it does not understand. Use a string path, *os.File, or
	// io.Writer.
	ErrNotSupportSaveMethod = errors.New("unsupported save type")

	// ErrInvalidTransportType is returned when a TLS or HTTP/2 helper is
	// asked to mutate a transport that is not *http.Transport. Set the
	// transport before applying TLS/HTTP/2 options, or apply the option to a
	// *http.Transport directly.
	ErrInvalidTransportType = errors.New("invalid transport type")

	// ErrResponseNil is returned when transport returned no error and no
	// response. Indicates a transport bug; the request should not be retried
	// blindly.
	ErrResponseNil = errors.New("response is nil")

	// ErrAutoRedirectDisabled is returned by [ProhibitRedirectPolicy] when a
	// 3xx response would normally trigger a follow-up request.
	ErrAutoRedirectDisabled = errors.New("auto redirect disabled")

	// ErrTooManyRedirects is returned when a redirect chain exceeds the
	// configured limit. Wrapped errors include the observed limit.
	ErrTooManyRedirects = errors.New("too many redirects")

	// ErrRedirectNotAllowed is returned by [RedirectSpecifiedDomainPolicy]
	// when the redirect target host is not in the allow-list.
	ErrRedirectNotAllowed = errors.New("redirect not allowed")

	// ErrTestTimeout is reserved for use by the package's own tests. It is
	// not produced by Send.
	ErrTestTimeout = errors.New("test timeout: request took too long")

	// ErrInvalidConfigValue is returned by [Config.Validate] when a numeric
	// field has a negative value. Wrapped errors name the offending field.
	ErrInvalidConfigValue = errors.New("invalid config value")

	// ErrInvalidTLSClientCertificateConfig is returned when one of
	// TLSClientCertFile or TLSClientKeyFile is set without the other.
	ErrInvalidTLSClientCertificateConfig = errors.New("TLSClientCertFile and TLSClientKeyFile must both be set or both be empty")
)

// IsTimeout reports whether err is or wraps a deadline-driven failure:
// [context.DeadlineExceeded] from a request or per-call timeout, or any
// [net.Error] whose Timeout method returns true (for example, dial,
// TLS-handshake, or response-header timeouts).
//
// IsTimeout intentionally does not match [context.Canceled]; use [IsCanceled]
// for explicit cancellation.
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	netErr, ok := errors.AsType[net.Error](err)
	return ok && netErr.Timeout()
}

// IsCanceled reports whether err is or wraps [context.Canceled], which
// indicates the caller cancelled the request before completion. Use this to
// distinguish caller-initiated cancellation from deadline-driven timeout
// (see [IsTimeout]).
func IsCanceled(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.Canceled)
}

// IsConnectionError reports whether err is a connection-level failure
// (DNS resolution, TCP connect, TLS handshake), as surfaced by
// [net.OpError].
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := errors.AsType[*net.OpError](err)
	return ok
}
