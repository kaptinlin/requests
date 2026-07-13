# Retry and Delivery Specs

## Overview

Retry behavior is part of request delivery. This spec defines `RetryPolicy`, attempt counts, retry conditions, backoff strategies, `Retry-After` handling, cancellation behavior, and request body replay.

## Retry Policy

Retry configuration is one value:

```go
type RetryPolicy struct {
	Max              int
	Backoff          BackoffStrategy
	ShouldRetry      RetryIfFunc
	IgnoreRetryAfter bool
}
```

`Max` is the number of retry attempts after the initial transport attempt.

- `0` means a single transport attempt with no retries.
- `3` means up to four total attempts.

Client defaults come from `WithRetry(policy)` during construction or cloning. Request-local overrides come from `RequestBuilder.Retry(policy)`. `RequestBuilder.NoRetry()` disables retries for one request.

`Response.Attempts()` and `StreamResponse.Attempts()` report total transport attempts, including the initial attempt.

## Retry Conditions

`RetryIfFunc` decides whether an HTTP response or transport error should be retried. The retry loop calls `ShouldRetry(req, resp, err)` for both response and error outcomes, so custom policies control all retry decisions.

If `RetryPolicy.ShouldRetry` is nil, delivery uses `DefaultRetryIf`.

`DefaultRetryIf` retries on:

- errors classified by `IsTimeout`
- errors whose chain contains `*net.OpError`, as classified by `IsConnectionError`
- `408 Request Timeout`
- `429 Too Many Requests`
- `5xx` responses

Default retry does not retry caller cancellation, TLS certificate verification,
malformed request construction, invalid headers, redirect policy failures, or
other deterministic caller/configuration errors. Error joins from exhausted
attempts preserve `errors.Is` / `errors.As` traversal and therefore preserve the
same helper classifications.

## Backoff Strategies

The package defines:

- `DefaultBackoffStrategy`
- `LinearBackoffStrategy`
- `ExponentialBackoffStrategy`
- `JitterBackoffStrategy`

Backoff strategies receive a zero-based retry attempt index. If `RetryPolicy.Backoff` is nil, delivery uses `DefaultBackoffStrategy(1*time.Second)`.

## Retry-After Handling

For `429` and `503`, the retry loop checks `Retry-After` before falling back to the configured backoff strategy unless `RetryPolicy.IgnoreRetryAfter` is true.

Supported `Retry-After` forms are:

- integer seconds
- HTTP date

Invalid or negative `Retry-After` values fall back to the configured strategy.

## Cancellation and Cleanup

The retry loop respects the request context.

- If the context is canceled or reaches its deadline during backoff, delivery stops and returns `ctx.Err()`.
- Before sleeping for a retry, any received response body is drained up to an internal cap and closed.
- When proxy rotation is configured through `WithProxies` or a proxy selector, proxy choice is evaluated per transport attempt, so retries may use different proxies.

Callers classify the failure with the package helpers: `IsCanceled` matches `context.Canceled` only, and `IsTimeout` matches `context.DeadlineExceeded` and `net.Error` timeouts. The two are orthogonal so caller-driven cancellation is distinguishable from a deadline hit.

> **Why**: Delivery policy must be a single value so callers can reason about latency, retry conditions, and request-local override without mentally merging three separate fields.
>
> **Rejected**: Ambiguous retry rules that split attempt count, backoff, and condition across unrelated setters.

## Request Override Rule

Request-local retry configuration overrides the client retry configuration completely for that request.

`RequestBuilder.NoRetry()` disables retries even when the client has a positive default.

## Request Body Replay

Before retrying a request with a replayable body, delivery restores `req.Body` through `req.GetBody`.

Replayable body sources include built-in buffered/string body helpers (`JSON`, `XML`, `YAML`, `Text`, `Bytes`, `Form`, `FormField`, `FormFields`) and multipart builders that explicitly opt into `Replayable(maxBytes)`. A non-seekable `io.Reader` passed to `Reader` is non-replayable: when a retry would need to resend the body, delivery returns `ErrRequestBodyNotReplayable` instead of silently re-sending or silently skipping.

Body preparation populates `GetBody` for replayable sources regardless of the
active retry count, because 307/308 redirects consume the same standard
contract. Every `GetBody` call returns a fresh body with byte-equivalent
content. The delivery loop does not inspect concrete reader types; it only
checks and invokes `req.GetBody`.

Non-replayable streaming bodies are attempted once; if their first attempt returns a retryable response, delivery returns `ErrRequestBodyNotReplayable`.

The first request body is closed by the HTTP transport. Preparation does not
pre-close a caller-provided request reader. Nested multipart `FilePart.Body`
values remain caller-owned and are not closed by the multipart writer.

## Forbidden

- Do not count the initial transport attempt as a retry.
- Do not expect `Retry-After` to apply to status codes other than `429` and `503`.
- Do not assume streaming request bodies can be retried unless the body source explicitly supports replay.

## Contract Invariants

- Retry configuration is expressed as one policy value.
- Attempts reporting includes the initial transport attempt.
- Default retry conditions are explicit.
- `Retry-After` precedence is explicit.
- Context cancellation and retry cleanup rules are explicit.
- Request body replay behavior is explicit.
