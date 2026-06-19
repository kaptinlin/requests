# Response API Specs

## Overview

`Response` exposes the result of `RequestBuilder.Send`. This spec defines buffered response behavior, status helpers, decoding, saving, and line iteration.

## Buffered Result

`Response` is the buffered result returned by `RequestBuilder.Send`.

The response body is fully read into an internal buffer, the underlying body is
replaced with a reader over the buffered bytes, and buffered helpers such as
`Body()` and `String()` are available.

> **Why**: Buffered helpers and caller-owned streaming solve different workloads. Keeping them separate avoids ambiguous ownership of the response body.
>
> **Rejected**: A hybrid response that partially buffers streamed data while also promising caller-owned streaming.

## Accessors and Status Helpers

`Response` exposes:

- `StatusCode`, `Status`
- `Raw`
- `Header`, `Cookies`, `Location`, `URL`
- `Elapsed`, `Attempts`, `Protocol`, `TLS`
- `ContentType`, `IsContentType`, `IsJSON`, `IsXML`, `IsYAML`
- `ContentLength`, `IsEmpty`
- `IsSuccess`, `IsError`, `IsClientError`, `IsServerError`, `IsRedirect`
- `Body`, `String`, `Close`

These helpers describe response metadata only. They do not change delivery behavior.

Diagnostics:

- `Elapsed()` returns the duration from request dispatch through buffered response setup.
- `Attempts()` returns transport attempts for this response, including the first request.
- `Protocol()` returns the final `http.Response.Proto` string.
- `TLS()` returns a copy of the final response TLS connection state, or nil for non-TLS responses.

## Decoding Contract

`Decode` dispatches by `Content-Type` and only supports the content types recognized by `IsJSON`, `IsXML`, and `IsYAML`.

- `DecodeJSON` uses the JSON decoder captured when `Send` dispatched the request.
- `DecodeXML` uses the XML decoder captured when `Send` dispatched the request.
- `DecodeYAML` uses the YAML decoder captured when `Send` dispatched the request.

If `Decode` cannot match a supported content type, it returns `ErrUnsupportedContentType`.

> **Why**: A returned response is the result of one dispatch snapshot. Later
> client mutations must not change how that response decodes its buffered body.
>
> **Rejected**: Decoding through the live client after the request has completed.

## Save Contract

`Save` accepts exactly two output forms:

- a filesystem path as `string`
- an `io.Writer`

When saving to a path, parent directories are created as needed. Any other save target is invalid.

When saving to an `io.Writer`, `Save` writes the buffered body but does not close or flush the caller-provided writer. The caller owns that writer's lifecycle. `Save` only closes files it opens itself for string path targets.

## Line Iteration

`Lines()` returns an `iter.Seq[[]byte]` over the buffered response body.

## Forbidden

- Do not call `Decode` and expect fallback decoding for unsupported content types.
- Do not pass unsupported target types to `Save`.

## Contract Invariants

- Buffered response ownership is explicit.
- The supported decoding paths are explicit.
- Response diagnostics are read-only helpers.
- Save targets are explicit.
- `Lines()` is defined over buffered response bytes.
