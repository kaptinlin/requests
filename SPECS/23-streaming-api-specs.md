# Streaming API Specs

## Overview

Streaming is an explicit dispatch mode. `RequestBuilder.SendStream(ctx)` returns a `StreamResponse` with an open response body owned by the caller.

`RequestBuilder.Send(ctx)` is buffered and never starts a background stream reader.

## Stream Result

`StreamResponse` exposes:

- `Raw() *http.Response`
- `StatusCode`, `Status`, `Header`, and `URL`
- `Elapsed` and `Attempts`
- `Body() io.ReadCloser`
- `Lines() iter.Seq2[[]byte, error]`
- `Close() error`

The caller MUST close `StreamResponse`. Closing the stream response closes the response body and releases any request context derived from builder timeout.

`Elapsed()` returns the duration from request dispatch through stream response setup. `Attempts()` returns transport attempts for the stream response, including the first request.

## Line Iteration

`Lines()` is line-oriented.

- It scans `StreamResponse.Body()` with `bufio.Scanner`.
- Each yielded value is one scanner token, which in practice is one line.
- Yielded byte slices are owned by the caller and remain stable after the next scan.
- The maximum line token size is 512 KiB.
- Scanner errors are yielded as the second iterator value.

Callers that need a protocol other than line-oriented scanning SHOULD read directly from `Body()`.

> **Why**: Line-oriented iteration fits SSE, JSONL, and newline-delimited protocols while keeping the helper small. Returning errors through the iterator keeps the lifecycle caller-owned instead of hiding it in callbacks.
>
> **Rejected**: Arbitrary chunk callbacks with transport-dependent chunk boundaries.

## Buffered Helper Boundary

`StreamResponse` is not the buffered `Response` API described in `SPECS/22-response-api-specs.md`. It does not expose `Body() []byte`, `String`, `Decode*`, `Save`, or buffered `Lines`.

## Forbidden

- Do not use `Send(ctx)` when the caller intends to read a live response body.
- Do not assume `Lines()` receives arbitrary transport chunks.
- Do not forget to close `StreamResponse`.

## Contract Invariants

- Streaming is selected with `SendStream(ctx)`.
- Stream body ownership belongs to the caller.
- The line-oriented scanner model and size limit are explicit.
- Scanner errors are observable by callers.
