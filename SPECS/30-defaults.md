# Defaults Audit

This spec lists every effective default applied by `requests` and the reason it
is chosen. It exists so changes to a default are evaluated against its original
rationale rather than guessed at. Changes to any default value listed here are
contract changes and must be called out in release notes.

The audit is grouped by concern. Each row describes:

- **Field**: where the default appears (`Client`, functional option, internal
  constant, or behavior with no explicit knob).
- **Default**: the value used when the caller does not opt in.
- **Source**: file and function that materializes the default.
- **Why**: the reason this default is correct.

## Construction

| Field | Default | Source | Why |
|---|---|---|---|
| `New()` options | none applied | `client.go: New` | Lets callers compose the smallest meaningful client while still receiving construction errors. `New()` and `New(WithBaseURL("..."))` are both valid. |
| underlying `*http.Client` | `&http.Client{}` | `client.go: newClient` | Keeps transport, timeout, cookie jar, and redirect behavior on standard-library defaults until callers opt in. |

## Timeouts

| Field | Default | Source | Why |
|---|---|---|---|
| `http.Client.Timeout` | `0` | `client.go: newClient` | Zero passes through to `net/http`'s "no timeout" behavior. We deliberately do not impose a 30 s default: many call sites already use `context.WithTimeout`, and adding a hidden ceiling would surprise long-poll, SSE, and large-download users. Callers who forget to set a timeout are no worse off than they would be using `net/http` directly. |
| `WithDialTimeout` | not applied | `client_option.go` / `client.go` | Dial timeout is only configured when the caller explicitly sets it; otherwise the underlying `http.Transport` default applies. |
| `WithTLSHandshakeTimeout` | not applied | `client_option.go` / `client.go` | Same passthrough rule. |
| `WithResponseHeaderTimeout` | not applied | `client_option.go` / `client.go` | Same passthrough rule. |
| `WithIdleConnTimeout` | not applied | `client_option.go` / `client.go` | Same passthrough rule. |
| `RequestBuilder.Timeout` | `0` (no per-request deadline) | `request.go: prepareContext` | Per-request timeout layered onto the caller's `ctx` only when set; `ctx`'s own deadline is preserved otherwise. |

## Retries

| Field | Default | Source | Why |
|---|---|---|---|
| `RetryPolicy.Max` | `0` (no retries) | `client.go: newClient` | Retries are opt in. Defaulting to >0 would silently re-send POSTs, PUTs, and PATCHes when the body happens to be replayable; that is not a safe assumption to bake into a primitive. |
| `RetryPolicy.Backoff` | `DefaultBackoffStrategy(1*time.Second)` | `retry.go: DefaultRetryPolicy` | Always populated so callers who set `Max` without thinking about backoff still get a sane delay between attempts. The constant 1 s is short enough for unit tests and long enough to avoid hammering a flapping server. |
| `RetryPolicy.ShouldRetry` | `DefaultRetryIf` | `retry.go: DefaultRetryPolicy` | Retries timeout-classified errors, errors containing `*net.OpError`, 408, 429, and 5xx. Deterministic TLS verification errors are not retried. Callers who want stricter behavior must override. |
| `RetryPolicy.IgnoreRetryAfter` | `false` | `retry.go: RetryPolicy` | `Retry-After` should be honored by default for 429 and 503 responses. Callers can opt out when their latency budget makes backoff authoritative. |
| Body replay | auto-snapshot for `*bytes.Buffer` and `ReadAt+Seek+Size` readers | `request.go: snapshotReaderBody` | The library opts callers into replay safely (buffer if cheap, refuse otherwise) instead of silently re-sending unreplayable streams. |

## TLS

| Field | Default | Source | Why |
|---|---|---|---|
| TLS config | `nil` (Go default `tls.Config`) | `client.go: newClient` | Passthrough to `crypto/tls`. The Go standard library already defaults to a safe configuration; we do not override it. |
| Internally created `tls.Config` | `MinVersion: tls.VersionTLS12` | `client.go: updateTLSConfigLocked` | When the library has to construct a TLS config because the caller supplied SNI, certificates, or CA material through client helpers/options, it pins TLS 1.2 as the floor. Callers supplying their own `tls.Config` choose their own floor. |
| `WithClientCertificate` | not applied | `client_option.go` | mTLS is opt in. File-loading errors fail construction instead of being logged and ignored. |
| `WithTLSServerName` | not applied | `client_option.go` | SNI defaults to the request host. Only set when callers need a different value. |

## Redirects

| Field | Default | Source | Why |
|---|---|---|---|
| `http.Client.CheckRedirect` | unset (Go default: up to 10 redirects) | `client.go: newClient` | Passthrough to `net/http`'s default redirect behavior. Callers who want stricter limits or host pinning opt in via `WithRedirectPolicy(...)` with `Prohibit` / `Allow(N)` / `SpecifiedDomain(...)`. |

## Proxy

| Field | Default | Source | Why |
|---|---|---|---|
| Proxy | `http.ProxyFromEnvironment` through the nil/default transport | `client.go: newClient`; `proxy.go: ensureTransport` | Default passthrough matches `net/http`. Standard transport options clone `http.DefaultTransport`, so changing an unrelated field does not change proxy behavior. Callers use `WithoutProxy()` for explicit direct delivery. |
| Bypass list | empty | `proxy.go` | Bypass rules are explicit through `WithProxyBypass`. |

## Codecs

| Field | Default | Source | Why |
|---|---|---|---|
| `JSONEncoder` / `JSONDecoder` | private client-local `JSONEncoder` / `JSONDecoder` (`go-json-experiment/json`) | `client.go: newClient` | Always populated so `JSON` / `DecodeJSON` work without ceremony. Callers can swap encoder/decoder with construction options. |
| `XMLEncoder` / `XMLDecoder` | private client-local `XMLEncoder` / `XMLDecoder` | `client.go: newClient` | Same. |
| `YAMLEncoder` / `YAMLDecoder` | private client-local `YAMLEncoder` / `YAMLDecoder` (`goccy/go-yaml`) | `client.go: newClient` | Same. |

## Headers, Cookies, Body

| Field | Default | Source | Why |
|---|---|---|---|
| default headers | nil | `client.go: newClient` | No default headers. Callers add `User-Agent`, `Accept`, etc. explicitly. |
| ordered header intent | nil | `client.go: newClient` | Ordered header intent is opt in; standard transport semantics apply otherwise. |
| `http.Client.Jar` | nil | `client.go: newClient` | No cookie persistence by default. `WithSession()` enables a `cookiejar.Jar`. |
| Content-Type inference | none | `request.go: prepareBody` | Request bodies must be explicit: typed helpers set their own content type, while raw/stream bodies may omit it. The library no longer guesses from Go value shape. |

## Connection Pool

| Field | Default | Source | Why |
|---|---|---|---|
| `WithMaxIdleConns` / `WithMaxIdleConnsPerHost` / `WithMaxConnsPerHost` | not applied | `client_option.go` / `client.go` | Transport pool sizing only changes when the caller explicitly opts in. |

## Logger

| Field | Default | Source | Why |
|---|---|---|---|
| logger | nil | `client.go: newClient` | The library is silent by default. A nil-check in every emit site keeps the hot path zero-cost. Callers who want logs supply a logger via `WithLogger`. |

## HTTP/2 and HTTP/3

| Field | Default | Source | Why |
|---|---|---|---|
| `WithHTTP2` | not applied | `client_option.go` / `client.go` | The Go standard library already negotiates HTTP/2 over TLS by default; this option is a manual override for callers who want explicit HTTP/2 transport. |
| HTTP/3 | not enabled | `http3/` extension | HTTP/3 is opt in via the extension module. The core does not link `quic-go`. |

## Profiles

| Field | Default | Source | Why |
|---|---|---|---|
| `WithProfile` | not applied | `client_option.go` / `profile.go` | Browser-like headers, uTLS fingerprints, and HTTP/3 are opt in via `Profile`. The core has no implicit identity. |

## Stability Rules

- Adding a new default value where today there is none is a contract change. It
  must be called out in release notes and must preserve the existing behavior
  unless the change is explicitly desired.
- Removing or renaming a row is a contract change that must follow the release
  policy and appear in release notes.
- This document is the source of truth when reviewing a PR that changes `newClient`, a `With*` option, a `Default*` constructor, or a passthrough behavior.
