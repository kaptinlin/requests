# Response API Specs

## Overview

`Response` exposes the result of `RequestBuilder.Send`. This spec defines buffered response behavior, status helpers, decoding, saving, and line iteration.

## Buffered and Streaming Modes

A response has two delivery modes:

- buffered mode when no stream callback is configured
- streaming mode when `RequestBuilder.Stream` is configured

In buffered mode, the response body is fully read into `BodyBytes`, the underlying body is replaced with a reader over the buffered bytes, and buffered helpers are available.

In streaming mode, body consumption is delegated to asynchronous callbacks. Callers SHOULD treat buffered helpers such as `Body`, `String`, `Scan*`, `Save`, and `Lines` as unavailable in practice for that response.

> **Why**: Buffered helpers and streaming callbacks solve different workloads. Keeping them separate avoids ambiguous ownership of the response body.
>
> **Rejected**: A hybrid mode that partially buffers streamed data while also promising callback-driven consumption.

## Accessors and Status Helpers

`Response` exposes:

- `StatusCode`, `Status`
- `Header`, `Cookies`, `Location`, `URL`
- `Elapsed`, `Attempts`, `Protocol`, `TLS`
- `ContentType`, `IsContentType`, `IsJSON`, `IsXML`, `IsYAML`
- `ContentLength`, `IsEmpty`
- `IsSuccess`, `IsError`, `IsClientError`, `IsServerError`, `IsRedirect`
- `Body`, `String`, `Close`

These helpers describe response metadata only. They do not change delivery behavior.

Diagnostics:

- `Elapsed()` returns the duration from request dispatch through buffered response setup or stream arming.
- `Attempts()` returns transport attempts for this response, including the first request.
- `Protocol()` returns the final `http.Response.Proto` string.
- `TLS()` returns a copy of the final response TLS connection state, or nil for non-TLS responses.

## Decoding Contract

`Scan` dispatches by `Content-Type` and only supports the content types recognized by `IsJSON`, `IsXML`, and `IsYAML`.

- `ScanJSON` uses the client JSON decoder.
- `ScanXML` uses the client XML decoder.
- `ScanYAML` uses the client YAML decoder.

If `Scan` cannot match a supported content type, it returns `ErrUnsupportedContentType`.

## Save Contract

`Save` accepts exactly two output forms:

- a filesystem path as `string`
- an `io.Writer`

When saving to a path, parent directories are created as needed. Any other save target is invalid.

## Line Iteration

`Lines()` returns an `iter.Seq[[]byte]` over a buffered response body. It is intended for non-streaming responses and yields no data when `BodyBytes` is absent.

## Forbidden

- Do not call `Scan` and expect fallback decoding for unsupported content types.
- Do not rely on `Lines()` for streaming responses.
- Do not pass unsupported target types to `Save`.

## Acceptance Criteria

- [ ] Buffered and streaming response modes are distinct.
- [ ] The supported decoding paths are explicit.
- [ ] Response diagnostics are read-only helpers.
- [ ] Save targets are explicit.
- [ ] The non-streaming boundary for `Lines()` is explicit.

**Origin:** Migrated from `docs/response.md`.
