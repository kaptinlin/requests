# Retry and Delivery Specs

## Overview

Retry behavior is part of request delivery. This spec defines retry counts, retry conditions, backoff strategies, `Retry-After` handling, and cancellation behavior.

## Retry Count Semantics

`MaxRetries` is the number of retry attempts after the initial transport attempt.

- `0` means a single transport attempt with no retries.
- `3` means up to four total attempts.

Client defaults come from `Client.SetMaxRetries` or `Config.MaxRetries`. Request-local overrides come from `RequestBuilder.MaxRetries`.

## Retry Conditions

`RetryIfFunc` decides whether a request should be retried.

`DefaultRetryIf` retries on:

- transport errors
- `408 Request Timeout`
- `429 Too Many Requests`
- `5xx` responses

## Backoff Strategies

The package defines:

- `DefaultBackoffStrategy`
- `LinearBackoffStrategy`
- `ExponentialBackoffStrategy`
- `JitterBackoffStrategy`

Backoff strategies receive a zero-based retry attempt index.

## Retry-After Handling

For `429` and `503`, the retry loop checks `Retry-After` before falling back to the configured backoff strategy.

Supported `Retry-After` forms are:

- integer seconds
- HTTP date

Invalid or negative `Retry-After` values fall back to the configured strategy.

## Cancellation and Cleanup

The retry loop respects the request context.

- If the context is canceled or reaches its deadline during backoff, delivery stops and returns `ctx.Err()`.
- Before sleeping for a retry, any received response body is closed.
- When proxy rotation is configured through `SetProxies` or a proxy selector, proxy choice is evaluated per transport attempt, so retries may use different proxies.

> **Why**: Delivery policy must be explicit about attempt counts and backoff sources so callers can reason about latency and failure handling.
>
> **Rejected**: Ambiguous retry rules that leave attempt counts or `Retry-After` precedence implicit.

## Request Override Rule

Request-local retry configuration SHOULD override the client retry configuration completely for that request.

> **Status**: not yet implemented for zero retries in `request.go:415-418`; `RequestBuilder.MaxRetries(0)` does not currently disable a positive client default.

## Forbidden

- Do not count the initial transport attempt as a retry.
- Do not expect `Retry-After` to apply to status codes other than `429` and `503`.
- Do not assume request-local `MaxRetries(0)` currently disables a positive client default without verifying the implementation status note.

## Acceptance Criteria

- [ ] Retry count semantics are explicit.
- [ ] Default retry conditions are explicit.
- [ ] `Retry-After` precedence is explicit.
- [ ] Context cancellation and retry cleanup rules are explicit.
- [ ] The current zero-retry override gap is explicit.

**Origin:** Migrated from `docs/retry.md`.
