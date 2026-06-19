// Package requests provides a fluent HTTP client library for Go.
//
// # Four-object model
//
// The package is built around four public objects:
//
//   - Client owns reusable configuration: base URL, default headers and cookies,
//     auth, retry policy, codecs, logger, and transport settings.
//   - RequestBuilder owns one outbound request: method, path, request-local
//     metadata, body, timeout, retries, and middleware.
//   - Response exposes the buffered result of one Send call.
//   - StreamResponse exposes the unbuffered result of one SendStream call.
//
// Client defaults are formed during New or Clone. State that is reused across
// requests belongs on Client; state for one request belongs on RequestBuilder.
//
// # Quick start
//
//	client, err := requests.New(
//	    requests.WithBaseURL("https://api.example.com"),
//	    requests.WithTimeout(30*time.Second),
//	)
//	if err != nil {
//	    return err
//	}
//
//	resp, err := client.Get("/users/{id}").
//	    PathParam("id", "1").
//	    Send(ctx)
//	if err != nil {
//	    return err
//	}
//	defer resp.Close()
//
//	var user User
//	if err := resp.DecodeJSON(&user); err != nil {
//	    return err
//	}
//
// # Construction
//
// Use [New] with functional options. Construction validates option input and
// returns an error instead of building a partially configured client.
//
// # Body lifecycle
//
// Request body setters fall into two groups:
//
//   - Replayable: [RequestBuilder.JSON], [RequestBuilder.XML],
//     [RequestBuilder.YAML], [RequestBuilder.Text], [RequestBuilder.Bytes],
//     [RequestBuilder.Form], [RequestBuilder.FormField], and
//     [RequestBuilder.FormFields]. The body is buffered or re-readable, so
//     retries are safe.
//   - One-shot: [RequestBuilder.Reader] given a raw [io.Reader] that is not
//     seekable, [RequestBuilder.Files], and non-replayable
//     [RequestBuilder.Multipart].
//     Such bodies cannot be replayed; if a retry is required, Send returns
//     [ErrRequestBodyNotReplayable] instead of silently re-sending or silently
//     skipping the retry. Use [Multipart.Replayable] when a multipart body must
//     be resent.
//
// # Errors
//
// Runtime failures are returned as errors; the package does not panic and does
// not expose Must-style APIs. Use [errors.Is] with the sentinels declared in
// errors.go to detect specific causes, and the helpers [IsTimeout],
// [IsCanceled], and [IsConnectionError] to classify transport-level failures.
//
// # Extensions
//
// Optional protocol and identity behavior lives in extension modules so the
// core does not pull their dependencies:
//
//   - github.com/kaptinlin/requests/browser     — browser-like ordered headers
//   - github.com/kaptinlin/requests/fingerprint — uTLS ClientHello profiles
//   - github.com/kaptinlin/requests/http3       — QUIC HTTP/3 transport
//
// All three plug in through the [Profile] interface.
//
// # Specifications
//
// Contract-level rules live under SPECS/ in the repository. Start with
// SPECS/00-overview.md.
package requests
