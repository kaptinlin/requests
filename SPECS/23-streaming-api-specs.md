# Streaming API Specs

## Overview

Streaming is configured on `RequestBuilder` before `Send`. This spec defines callback registration, the line-oriented delivery model, and the limits of streamed responses.

## Callback Registration

A streaming request is configured through:

- `Stream(StreamCallback)`
- `StreamErr(StreamErrCallback)`
- `StreamDone(StreamDoneCallback)`

These callbacks MUST be attached to the builder before `Send(ctx)` is called.

## Delivery Model

Streaming is line-oriented.

- The response body is scanned with `bufio.Scanner`.
- Each callback invocation receives one scanner token, which in practice is one line.
- The maximum token size is `MaxStreamBufferSize`.
- `Send(ctx)` returns the `Response` after streaming has been armed; callback execution continues asynchronously.

If `StreamCallback` returns an error, scanning stops immediately.

If the scanner itself fails and `StreamErr` is configured, `StreamErr` receives that scanner error.

If `StreamDone` is configured, it is called after scanning ends.

> **Why**: Line-oriented scanning fits SSE, JSONL, and other newline-delimited protocols while keeping the implementation small and allocation-aware.
>
> **Rejected**: Arbitrary chunk callbacks with transport-dependent chunk boundaries.

## Buffered Helper Boundary

A streamed response is not the buffered-response API described in `SPECS/22-response-api-specs.md`. Callers SHOULD consume streamed data through callbacks instead of through `Body`, `String`, `Scan*`, `Save`, or `Lines`.

## Size Limit

`MaxStreamBufferSize` is the maximum scanner token size. Payloads that require larger single tokens are outside the supported streaming contract.

## Forbidden

- Do not register `StreamErr` or `StreamDone` on `Response`; they are builder configuration methods.
- Do not assume stream callbacks receive arbitrary transport chunks.
- Do not assume buffered helpers are populated for streamed responses.

## Acceptance Criteria

- [ ] Callback registration happens on `RequestBuilder`.
- [ ] The line-oriented scanner model is explicit.
- [ ] The asynchronous execution model is explicit.
- [ ] The stream size limit is explicit.

**Origin:** Migrated from `docs/stream.md`.
