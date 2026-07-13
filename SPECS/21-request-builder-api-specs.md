# Request Builder API Specs

## Overview

`RequestBuilder` defines one outbound HTTP request. This spec defines path, metadata, body, timeout, middleware, retry, and dispatch behavior.

## Builder Creation

A builder is created by either:

- `Client.NewRequestBuilder(method, path)`
- one of the verb helpers: `Get`, `Post`, `Delete`, `Put`, `Patch`, `Options`, `Head`, `Connect`, `Trace`, or `Request(method, path)`

A builder is mutable until `Send(ctx)` is called.

## Preparation Errors

Fluent helpers that cannot return an error directly retain the first
preparation failure on the builder. This applies to `QueriesStruct`, `Form`,
`FormFields`, invalid or typed-nil request-local `Auth`, and nil entries passed
to `AddMiddleware`.

`Send` and `SendStream` return that original cause before taking a client
snapshot, deriving a context, opening or encoding a body, applying middleware,
or calling the transport. Later fluent calls do not replace the first cause.
Logging MAY report the failure but MUST NOT be its only caller-visible channel.

## Path and Query Construction

A builder MAY define:

- path replacement through `Path`, `PathParam`, `PathParams`, and `DelPathParam`
- query parameters through `Query`, `Queries`, `QueriesStruct`, and `DelQuery`

Path parameters use `{name}` placeholders and MUST be URL-path-escaped before dispatch.

Dispatch uses one resolver for base URL, request path, path params, absolute URLs, and query composition. Base URL paths and request paths are joined without accidental slash loss or duplication. Builder query values are appended to any existing raw query values, so repeated keys remain repeated.

## Request Metadata

A builder MAY define request-local metadata through:

- `Header`, `Headers`, `AddHeader`, `DelHeader`
- `OrderedHeaders`
- `Cookie`, `Cookies`, `DelCookie`
- `ContentType`, `Accept`, `UserAgent`, `Referer`
- `Auth`

Metadata applies in this order: client headers, client auth, request-local
headers, request-local auth. Each later layer replaces same-name values from
earlier layers, so Authorization has one unambiguous owner and value set.

Request-local headers override client default headers with the same header name, using case-insensitive header-name matching. Request-local `AddHeader` adds values within the request-local header set; it does not preserve an older client default value for that same header name.

Client and request-local cookies merge by cookie name. The request layer
replaces same-name defaults and preserves different-name defaults; repeated
cookies within one layer use the last value.

`OrderedHeaders` accepts an `orderedobject.Object[[]string]` where keys are header names and values are all values for each header. It sets request-local header values and preserves insertion order as request intent. Pseudo-headers are retained in ordered metadata for supporting HTTP/2 or HTTP/3 transports, but are not applied to `net/http` header maps.

When ordered headers are active, all request-local header helpers that mutate headers, including `Header`, `AddHeader`, `DelHeader`, `ContentType`, `Accept`, `UserAgent`, `Referer`, and body helpers that set `Content-Type`, MUST keep the ordered metadata in sync with the semantic `http.Header` values.

After auth and cookie precedence is applied, every existing non-pseudo ordered
entry is synchronized to the final semantic values. Pseudo-header entries stay
as intent and are never inserted into `http.Header`.

If a request-local plain header overrides a client ordered default without supplying request-local ordered metadata for that header, the client ordered metadata for that header is removed so supporting transports do not observe stale default values.

## Body Selection and Encoding

Each builder owns one body selection. Every body setter atomically replaces the
previous selection, so the last setter determines the outbound payload:

- `JSON`, `XML`, `YAML`, and `Text` select their named encoding.
- `Bytes` and `Reader` select a raw body.
- `Form`, `FormFields`, and `FormField` select a URL-encoded form.
- `Multipart` selects the multipart builder.

Repeated `FormField` and `FormFields` calls are additive while the current
selection is a form. The first such call after another body kind starts a new
form instead of reviving older fields.

`JSON`, `XML`, `YAML`, `Text`, form, and multipart generate their corresponding
content type. A later explicit request-header call owns `Content-Type` instead.
`Bytes` preserves an explicit caller header but removes a media type generated
by the body it replaces. `Reader` generates a media type only when its
`contentType` argument is non-empty and is one-shot unless the reader is
seekable and sized. Changing `Content-Type` after a typed setter does not change
which encoder that setter selected.

`Encoder.Encode` returns an `io.Reader` whose complete lifecycle contract is
reading. The request pipeline reads the value and MUST NOT discover or invoke
an additional `Close` method based on its dynamic type. Default and custom
encoders therefore cannot make cleanup an implicit second contract.

The builder does not infer content type from Go value shape. Non-raw encoded bodies without an explicit content type fail before dispatch.

`Form` and `FormFields` accept `url.Values`, `map[string][]string`,
`map[string]string`, or a struct encoded from `url` tags. They do not infer
multipart files from `map[string]any`.

`JSON(nil)` encodes JSON `null`. `Form(nil)` selects a real empty URL-encoded
form. `Multipart(nil)` records `ErrInvalidConfigValue` and fails before body or
transport work.

`Multipart` is the only multipart upload vocabulary. It supports fields, file readers, bytes, strings, explicit file metadata through `FilePart`, custom boundaries, and explicit retry buffering through `Replayable(maxBytes)`. Without `Replayable`, multipart bodies stream and are not replayable after the first transport attempt.

`FilePart.Body` is borrowed. The multipart writer reads it but MUST NOT close it
based on its dynamic type. The caller retains ownership of that nested source.
The outer `http.Request.Body` follows the standard transport contract and is
closed by the transport for each attempt or redirect hop.

> **Why**: One replaceable selection makes the fluent call order equal the
> payload order and prevents stale body sources or generated metadata from
> resurfacing.
>
> **Rejected**: Fixed source priority, merged multipart/form/arbitrary bodies,
> and a public generic `Body(any)` mode switch.

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

## Dispatch

`Send(ctx)`:

1. returns any retained preparation error
2. snapshots client state
3. resolves the URL and asks `http.NewRequestWithContext` to validate the
   context, method, and resolved URL with no body
4. derives the request timeout context and only then opens or encodes the body
5. constructs the outbound `http.Request` and applies auth, headers, and cookies
6. executes middleware and retry policy
7. returns a buffered `Response`

Invalid context, method, or resolved URL returns `ErrRequestCreationFailed`
before streaming producers start. URL-resolution and retained preparation
errors have the same no-open guarantee. The library does not read or close a
caller body source on these preflight failures.

`SendStream(ctx)` follows the same preparation and delivery path, but returns a `StreamResponse` without buffering the response body. The caller must close the stream response.

Client mutations after `Send` starts do not affect that in-flight request.

## Forbidden

- Do not chain `AddMiddleware`; it is a mutator, not a fluent builder method.
- Do not add a `Custom(path, method)` alias; arbitrary request creation is method-first through `Request(method, path)`.
- Do not assume `Timeout` overrides an existing context deadline.
- Do not add body aliases or content-type inference that obscure `JSON`, `XML`, `YAML`, `Text`, `Bytes`, `Reader`, form, and multipart ownership.

## Contract Invariants

- Builder creation and mutation boundaries are explicit.
- Body selection and generated content-type ownership are explicit.
- The retry-override rule and request body replay behavior are documented.
