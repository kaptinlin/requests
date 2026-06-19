# Public Surface Decisions

## Overview

The public API is intentionally small. A symbol belongs in the root package only
when it names a durable concept in the request language, not when it shortens one
call site or preserves an obsolete path.

This spec owns public-surface decisions that cut across client construction,
request building, response ownership, streaming, retries, and extension modules.
Feature-specific behavior still belongs in the owning specs.

## Settled Decisions

### Validated Construction

`New(opts ...Option) (*Client, error)` and `Clone(opts ...Option) (*Client, error)`
are the construction paths.

- **Why**: Construction is the point where invalid base URLs, invalid proxy
  values, certificate file failures, profile option failures, and invalid
  numeric values should become caller-visible errors.
- **Rejected**: Parallel constructors, config structs, or best-effort public
  setters that let an invalid client exist and fail later at request time.
- **Contract Impact**: Public examples and tests must use `New`, `Clone`, verb
  helpers, and `With*` options for client-level configuration.

### Explicit Body Language

Request bodies are described with `JSON`, `XML`, `YAML`, `Text`, `Bytes`,
`Reader`, form helpers, and `Multipart`.

- **Why**: The body method should reveal both encoding intent and ownership.
- **Rejected**: A generic body entry point that guesses content type from Go
  value shape.
- **Contract Impact**: Encoded body helpers set their content type explicitly.
  Raw byte and reader bodies do not imply content type unless the caller sets
  one.

### Caller-Owned Streaming

`Send(ctx)` is the buffered path. `SendStream(ctx)` is the streaming path and
returns `StreamResponse`, whose body remains open until the caller closes it.

- **Why**: Buffering and streaming have different ownership models. Keeping them
  separate prevents hidden background readers and ambiguous body lifetime.
- **Rejected**: A second streaming ownership model beside `SendStream`.
- **Contract Impact**: Streaming helpers live on `StreamResponse`; buffered
  decoding, saving, and buffered line iteration live on `Response`.

### Response Escape Hatches

`Response.Raw()` and `StreamResponse.Raw()` are the raw `net/http` escape
hatches. The response structs do not expose mutable storage fields.

- **Why**: Advanced callers sometimes need standard-library details, but ordinary
  response use should read through behavior methods.
- **Rejected**: Public mutable response storage, client references, or context
  fields on response structs.
- **Contract Impact**: Raw access is explicit and narrow; callers that mutate the
  returned `*http.Response` own the consequences.

### Retry Policy As One Value

Retry behavior is configured through `RetryPolicy` at the client layer with
`WithRetry` or at the request layer with `Retry`.

- **Why**: Attempt count, backoff, retry condition, and Retry-After policy are one
  delivery concern.
- **Rejected**: Separate scalar setters for retry count, retry strategy, and
  retry condition.
- **Contract Impact**: Request-local retry policy replaces the client policy for
  that request. `NoRetry()` is the public way to disable a positive default.

### Extension Module Release Boundary

Extension modules remain independently consumable modules. During a breaking
root release, the root module must be tagged and resolvable before extension
modules require that version.

- **Why**: `go mod tidy` validates required versions even when the workspace is
  active. Pre-pinning extensions to an unpublished root version creates a broken
  maintenance state.
- **Rejected**: Local `replace` directives or pre-pinned unpublished root
  versions as a substitute for publishable module verification.
- **Contract Impact**: `task test:published` is the release-boundary gate after
  the root tag exists and extension modules require that exact root version.

## Deliberate Public Escape Hatches

These symbols remain public because they name real integration points:

- `AsHTTPClient` and `AsTransport` expose standard-library adapter shapes.
- `GetHTTPClient` exposes the underlying client for advanced integration. Callers
  that mutate it own the risk.
- `GetTLSConfig` returns a clone so extension modules can inherit TLS intent
  without taking ownership of mutable client state.
- `RoundRobinProxies` and `RandomProxies` create proxy selectors for
  `WithProxySelector`.

## Forbidden

- Do not add aliases for removed construction, body, streaming, or retry names.
- Do not add public runtime setters for client defaults; use `Clone(opts...)` to
  derive a modified client.
- Do not expose mutable response internals as fields.
- Do not add a public symbol unless it names a durable concept that belongs in
  the request language.
- Do not make extension modules depend on an unpublished root version.

## Contract Invariants

- Public construction is limited to `New`, `Clone`, builder creation, and verb
  helpers.
- Request body APIs are explicit about encoding and ownership.
- Buffered and streaming response ownership remain separate.
- Public escape hatches are deliberate and named here.
- Extension module release verification is explicit.
