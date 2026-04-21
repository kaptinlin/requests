# Middleware Architecture Specs

## Overview

Middleware wraps request execution. This spec defines the middleware signature, composition order, and the public behavior of the built-in middleware package.

## Signature

The middleware contract is:

```go
type Middleware func(next MiddlewareHandlerFunc) MiddlewareHandlerFunc
```

A middleware receives the next handler and returns a new handler.

## Composition Model

Middleware may be attached at two layers:

- client layer through `Client.AddMiddleware`
- request layer through `RequestBuilder.AddMiddleware`

Within each layer, middleware runs in registration order.

Across layers, client middleware wraps request middleware. The effective stack is:

1. client middleware
2. request middleware
3. final HTTP execution handler

> **Why**: Client middleware expresses cross-cutting policy for all requests, while request middleware expresses one-shot behavior closer to the transport attempt.
>
> **Rejected**: A single undifferentiated middleware list shared by all requests.

## Built-in Middleware

The `middlewares` package defines:

- `HeaderMiddleware`, which adds headers to every request it wraps
- `CookieMiddleware`, which adds cookies to every request it wraps
- `CacheMiddleware`, which caches only successful `GET` responses with status `200 OK`

`CacheMiddleware` requires:

- a `Cacher`
- a TTL
- a non-nil `requests.Logger`

Cache keys are derived from request path plus raw query string.

## Mutation Contract

`Client.AddMiddleware` and `RequestBuilder.AddMiddleware` are mutators. They do not return fluent builders.

## Forbidden

- Do not use the legacy two-argument middleware signature.
- Do not assume `RequestBuilder.AddMiddleware` returns `*RequestBuilder`.
- Do not pass a nil logger to `CacheMiddleware`.

## Acceptance Criteria

- [ ] The middleware function signature is explicit.
- [ ] Layering and execution order are explicit.
- [ ] The built-in middleware contracts are explicit.
- [ ] The mutating, non-fluent nature of `AddMiddleware` is explicit.

**Origin:** Migrated from `docs/middleware.md`.
