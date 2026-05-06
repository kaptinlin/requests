# Defaults Audit

This spec lists every effective default applied by `requests` and the reason it
is chosen. It exists so future changes to a default can be evaluated against
its original rationale rather than guessed at. Changes to any default value
listed here are a compatibility event and must be called out in release notes.

The audit is grouped by concern. Each row describes:

- **Field**: where the default appears (`Config` struct, functional option,
  internal constant, or behavior with no explicit knob).
- **Default**: the value used when the caller does not opt in.
- **Source**: file and function that materializes the default.
- **Why**: the reason this default is correct.

## Construction

| Field | Default | Source | Why |
|---|---|---|---|
| `New()` options | none applied | `client.go: New` | Lets callers compose the smallest meaningful client. `New()` and `New(WithBaseURL("..."))` are both valid. |
| `URL(baseURL)` | `Config{BaseURL: baseURL}` | `client.go: URL` | Shortcut for the most common one-liner. Behavior identical to `Create(&Config{BaseURL: ...})`. |
| `Create(nil)` | `&Config{}` | `client.go: Create` | Treats nil as "all defaults". Avoids forcing callers to pass an empty struct. |

## Timeouts

| Field | Default | Source | Why |
|---|---|---|---|
| `Config.Timeout` | `0` | `client.go: Create` | Zero passes through to `net/http`'s "no timeout" behavior. We deliberately do not impose a 30 s default: many call sites already use `context.WithTimeout`, and adding a hidden ceiling would surprise long-poll, SSE, and large-download users. Callers who forget to set a timeout are no worse off than they would be using `net/http` directly. |
| `Config.DialTimeout` | `0` (transport default) | `client.go: applyTransportConfig` | Only configured when the caller explicitly sets it; otherwise the underlying `http.Transport` default applies. |
| `Config.TLSHandshakeTimeout` | `0` (transport default) | `client.go: applyTransportConfig` | Same passthrough rule. |
| `Config.ResponseHeaderTimeout` | `0` (transport default) | `client.go: applyTransportConfig` | Same passthrough rule. |
| `Config.IdleConnTimeout` | `0` (transport default) | `client.go: applyTransportConfig` | Same passthrough rule. |
| `RequestBuilder.Timeout` | `0` (no per-request deadline) | `request.go: prepareContext` | Per-request timeout layered onto the caller's `ctx` only when set; `ctx`'s own deadline is preserved otherwise. |

## Retries

| Field | Default | Source | Why |
|---|---|---|---|
| `Config.MaxRetries` | `0` (no retries) | `client.go: Create` | Retries are an opt-in behavior. Defaulting to >0 would silently re-send POSTs, PUTs, and PATCHes when the body happens to be replayable; that is not a safe assumption to bake into a primitive. |
| `Config.RetryStrategy` | `DefaultBackoffStrategy(1*time.Second)` | `client.go: Create` | Always populated so callers who set `MaxRetries` without thinking about backoff still get a sane delay between attempts. The constant 1 s is short enough for unit tests and long enough to avoid hammering a flapping server. |
| `Config.RetryIf` | `DefaultRetryIf` | `client.go: Create` | Retries on transport errors, 408, 429, and 5xx. Retrying 5xx is conservative but matches widely held expectations; callers who want stricter behavior must override. |
| Body replay | Auto-snapshot for `*bytes.Buffer` and `ReadAt+Seek+Size` readers | `request.go: snapshotBody` | The library opts callers into replay safely (buffer if cheap, refuse otherwise) instead of silently re-sending unreplayable streams. |

## TLS

| Field | Default | Source | Why |
|---|---|---|---|
| `Config.TLSConfig` | `nil` (Go default `tls.Config`) | `client.go: Create` | Passthrough to `crypto/tls`. The Go standard library already defaults to a safe configuration; we do not override it. |
| Internally created `tls.Config` | `MinVersion: tls.VersionTLS12` | `client.go: Create` (TLSServerName / cert paths) | When the library has to construct a TLS config because the user supplied a server name or client cert paths, it pins TLS 1.2 as the floor. Users supplying their own `tls.Config` choose their own floor. |
| `Config.TLSClientCertFile` / `TLSClientKeyFile` | both empty | `client.go: Create` | mTLS is opt-in. Both must be set together (validated in `Config.Validate`). |
| `Config.TLSServerName` | empty | `client.go: Create` | SNI defaults to the request host. Only set when callers need a different value. |

## Redirects

| Field | Default | Source | Why |
|---|---|---|---|
| `http.Client.CheckRedirect` | unset (Go default: up to 10 redirects) | `client.go: Create` | Passthrough to `net/http`'s default redirect behavior. Callers who want stricter limits, host pinning, or stripping of sensitive headers opt in via `SetRedirectPolicy(...)` with `Prohibit` / `Allow(N)` / `Smart(N)` / `SpecifiedDomain(...)`. |
| `SmartRedirectPolicy` | strips `Authorization`, `Cookie`, `Cookie2`, `Proxy-Authorization`, `Www-Authenticate` on cross-host or HTTPS→HTTP | `redirect.go: sensitiveHeaders` | Mirrors the safety rules used by `requests` (Python) and `curl --location-trusted=false`. |

## Proxy

| Field | Default | Source | Why |
|---|---|---|---|
| Proxy | none | `proxy.go` | The library does **not** automatically read `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY`. Callers who want env-driven proxying call `SetProxyFromEnv()`. Reading env by default would surprise embedded uses and tests. |
| Bypass list | empty | `proxy.go` | Bypass rules are explicit per call to `SetProxy*` helpers. |

## Codecs

| Field | Default | Source | Why |
|---|---|---|---|
| `JSONEncoder` / `JSONDecoder` | `DefaultJSONEncoder` / `DefaultJSONDecoder` (`go-json-experiment/json`) | `client.go: Create` | Always populated so `JSONBody` / `ScanJSON` work without ceremony. Callers can swap encoder/decoder via setters. |
| `XMLEncoder` / `XMLDecoder` | `DefaultXMLEncoder` / `DefaultXMLDecoder` | `client.go: Create` | Same. |
| `YAMLEncoder` / `YAMLDecoder` | `DefaultYAMLEncoder` / `DefaultYAMLDecoder` (`goccy/go-yaml`) | `client.go: Create` | Same. |

## Headers, cookies, body

| Field | Default | Source | Why |
|---|---|---|---|
| `Config.Headers` | nil | `client.go: Create` | No default headers. Callers add `User-Agent`, `Accept`, etc. explicitly. |
| `Config.OrderedHeaders` | nil | `client.go: Create` | Ordered header intent is opt-in; standard transport semantics apply otherwise. |
| `Config.CookieJar` | nil | `client.go: Create` | No cookie persistence by default. `WithSession()` enables a `cookiejar.Jar`. |
| Inferred Content-Type | `application/json` for struct/map/slice; `application/x-www-form-urlencoded` for `url.Values`/maps; `text/plain` for string/[]byte | `request.go: inferContentType` | Last-resort heuristic when the caller used `Body(...)` without setting Content-Type. Typed setters (`JSONBody`, etc.) bypass inference. |

## Connection pool

| Field | Default | Source | Why |
|---|---|---|---|
| `MaxIdleConns` / `MaxIdleConnsPerHost` / `MaxConnsPerHost` | 0 (transport defaults) | `client.go: applyTransportConfig` | Transport pool sizing only changes when the caller explicitly opts in. |

## Logger

| Field | Default | Source | Why |
|---|---|---|---|
| `Config.Logger` | nil | `client.go: Create` | The library is silent by default. A nil-check in every emit site keeps the hot path zero-cost. Callers who want logs supply `slog.New(...)` via `WithLogger`. |

## HTTP/2 and HTTP/3

| Field | Default | Source | Why |
|---|---|---|---|
| `Config.HTTP2` | false | `client.go: Create` | The Go standard library already negotiates HTTP/2 over TLS by default; this flag is a manual override for callers who want explicit HTTP/2 transport. |
| HTTP/3 | not enabled | `http3/` extension | HTTP/3 is opt-in via the extension module. The core does not link `quic-go`. |

## Profiles

| Field | Default | Source | Why |
|---|---|---|---|
| `Config.Profile` (via `WithProfile`) | nil | `profile.go` | Browser-like headers, uTLS fingerprints, and HTTP/3 are opt-in via `Profile`. The core has no implicit identity. |

## Stability rules

- Adding a new default value (where today there is none) is a **compatibility event**. It must be called out in release notes and must default to the existing behavior unless the change is explicitly desired.
- Removing or renaming a row in this table requires a major version bump.
- This document is the source of truth when reviewing a PR that changes a `Config` field, a `Default*` constructor, or a passthrough block in `Create`.
