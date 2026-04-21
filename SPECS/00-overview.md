# Requests Overview

## Overview

`requests` defines a fluent HTTP client around three public objects: `Client`, `RequestBuilder`, and `Response`. This spec defines the package boundaries and the request lifecycle that the other `SPECS/*.md` files refine.

## Public Model

- `Client` owns reusable configuration: base URL, default headers and cookies, auth, retry policy, codecs, logger, and transport settings.
- `RequestBuilder` owns one outbound request's method, path, request-local metadata, body, timeout, retries, middleware, and streaming callbacks.
- `Response` exposes the result of one `Send` call, either as a buffered body or as an asynchronously consumed stream.
- Middleware, redirect policies, and proxy selection affect request delivery. They do not change the public roles of `Client`, `RequestBuilder`, or `Response`.

> **Why**: Shared state belongs on `Client`, while request-specific state belongs on `RequestBuilder`. This keeps reuse predictable and makes the point where defaults become fixed explicit.
>
> **Rejected**: A single mutable object that mixes reusable client policy with one-shot request state.

## Request Lifecycle

1. Construct a client with `New`, `URL`, or `Create`.
2. Create a builder with `NewRequestBuilder` or an HTTP verb helper such as `Get` or `Post`.
3. Populate request-local state on the builder.
4. Call `Send(ctx)`, which snapshots the client state before dispatch.
5. Resolve path, query, body, auth, headers, and cookies from the builder plus the client snapshot.
6. Execute middleware and retry policy around the transport attempt.
7. Return a `Response` that either buffers the body or starts asynchronous streaming callbacks.

## Boundary Rules

- `SPECS/20-client-api-specs.md` defines reusable client configuration and transport policy.
- `SPECS/21-request-builder-api-specs.md` defines request construction and per-request overrides.
- `SPECS/22-response-api-specs.md` defines buffered response behavior.
- `SPECS/23-streaming-api-specs.md` defines streaming delivery.
- `SPECS/40-middleware-architecture-specs.md` defines middleware composition.
- `SPECS/41-retry-and-delivery-specs.md` defines retries and delivery timing.

## Forbidden

- Do not put one-shot request state on `Client` when it belongs on `RequestBuilder`.
- Do not assume mutating `Client` after `Send` starts can change an in-flight request.
- Do not define the same public rule in multiple specs; each concept belongs in exactly one file.

## Acceptance Criteria

- [ ] The client, builder, and response roles are distinct.
- [ ] The snapshot point is explicit.
- [ ] Delivery concerns are delegated to the dedicated specs.

**Origin:** Synthesized during SPECS consolidation from `docs/client.md`, `docs/request.md`, `docs/response.md`, `docs/stream.md`, `docs/logging.md`, `docs/middleware.md`, and `docs/retry.md`.
