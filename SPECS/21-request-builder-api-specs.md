# Request Builder API Specs

## Overview

`RequestBuilder` defines one outbound HTTP request. This spec defines path, metadata, body, timeout, middleware, retry, clone, and dispatch behavior.

## Builder Creation

A builder is created by either:

- `Client.NewRequestBuilder(method, path)`
- one of the verb helpers: `Get`, `Post`, `Delete`, `Put`, `Patch`, `Options`, `Head`, `Connect`, `Trace`, or `Custom`

A builder is mutable until `Send(ctx)` is called.

## Path and Query Construction

A builder MAY define:

- path replacement through `Path`, `PathParam`, `PathParams`, and `DelPathParam`
- query parameters through `Query`, `Queries`, `QueriesStruct`, and `DelQuery`

Path parameters use `{name}` placeholders and MUST be URL-path-escaped before dispatch.

## Request Metadata

A builder MAY define request-local metadata through:

- `Header`, `Headers`, `AddHeader`, `DelHeader`
- `OrderedHeaders`
- `Cookie`, `Cookies`, `DelCookie`
- `ContentType`, `Accept`, `UserAgent`, `Referer`
- `Auth`

Request-local auth overrides client auth for that request.

Request-local headers override client default headers with the same header name, using case-insensitive header-name matching. Request-local `AddHeader` adds values within the request-local header set; it does not preserve an older client default value for that same header name.

`OrderedHeaders` accepts an `orderedobject.Object[[]string]` where keys are header names and values are all values for each header. It sets request-local header values and preserves insertion order as request intent. Pseudo-headers are retained in ordered metadata for supporting HTTP/2 or HTTP/3 transports, but are not applied to `net/http` header maps.

When ordered headers are active, all request-local header helpers that mutate headers, including `Header`, `AddHeader`, `DelHeader`, `ContentType`, `Accept`, `UserAgent`, `Referer`, and body helpers that set `Content-Type`, MUST keep the ordered metadata in sync with the semantic `http.Header` values.

If a request-local plain header overrides a client ordered default without supplying request-local ordered metadata for that header, the client ordered metadata for that header is removed so supporting transports do not observe stale default values.

## Body Selection and Encoding

The outbound body uses the first applicable source in this order:

1. explicit multipart form data from `Multipart`
2. multipart form data when one or more files are attached with `File` or `Files`
3. URL-encoded form data when form fields exist and no files exist
4. `bodyData` from `JSON`, `XML`, `YAML`, `Text`, `Bytes`, or `Reader`

`JSON`, `XML`, `YAML`, and `Text` set the corresponding content type explicitly. `Bytes` leaves `Content-Type` unchanged. `Reader` accepts an optional content type and is one-shot unless the reader is seekable and sized.

The builder does not infer content type from Go value shape. Non-raw encoded bodies without an explicit content type fail before dispatch.

`Multipart` is the streaming multipart builder. It supports fields, file readers, bytes, strings, explicit file metadata through `FilePart`, custom boundaries, and explicit retry buffering through `Replayable(maxBytes)`. Without `Replayable`, multipart bodies stream and are not replayable after the first transport attempt.

> **Why**: The body source order keeps multipart and form workflows predictable while preserving one explicit fallback for generic body data.
>
> **Rejected**: Merging multipart, form, and arbitrary body data in one request body.

## Timeout and Retry Overrides

A builder MAY define request-local delivery policy through:

- `Timeout`
- `Retry`
- `NoRetry`

`Timeout` only creates a derived deadline when the provided context does not already have one.

`Retry(policy)` replaces the client retry policy for that request. `NoRetry()` disables retries even when the client has a positive default.

Request bodies that can be replayed SHOULD be restored before each retry attempt. Non-replayable bodies MUST NOT be retried after the first attempt once delivery has started.

## Middleware and Streaming

A builder MAY attach request-local middleware with `AddMiddleware`.

`AddMiddleware` mutates the builder in place and does not return `*RequestBuilder`.

`Send(ctx)` returns a buffered `Response`. `SendStream(ctx)` returns an unbuffered `StreamResponse` whose body is owned by the caller.

## Clone Behavior

`Clone()` creates a new builder that:

- shares the same client reference
- deep-copies headers, ordered headers, cookies, queries, path params, and form fields
- does not copy body data, form files, middleware, or retry policy

Callers MUST reapply non-cloned concerns on the clone when they are required.

## Dispatch

`Send(ctx)`:

1. snapshots client state
2. resolves path, query, body, auth, headers, and cookies
3. constructs the outbound `http.Request`
4. executes middleware and retry policy
5. returns a buffered `Response`

`SendStream(ctx)` follows the same preparation and delivery path, but returns a `StreamResponse` without buffering the response body. The caller must close the stream response.

Client mutations after `Send` starts do not affect that in-flight request.

## Forbidden

- Do not chain `AddMiddleware`; it is a mutator, not a fluent builder method.
- Do not assume `Clone` copies body data, files, retry policy, or middleware.
- Do not assume `Timeout` overrides an existing context deadline.
- Do not add body aliases or content-type inference that obscure `JSON`, `XML`, `YAML`, `Text`, `Bytes`, `Reader`, form, and multipart ownership.

## Contract Invariants

- Builder creation and mutation boundaries are explicit.
- Body source precedence is explicit.
- The clone contract is explicit.
- The retry-override rule and request body replay behavior are documented.
