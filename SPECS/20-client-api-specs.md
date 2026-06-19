# Client API Specs

## Overview

`Client` owns reusable HTTP configuration. This spec defines construction, persistent defaults, transport policy, proxy and redirect controls, and the rules that apply before a request becomes a `RequestBuilder`.

## Construction

The package construction contract is:

```go
func New(opts ...Option) (*Client, error)
func (c *Client) Clone(opts ...Option) (*Client, error)
```

`New` applies options in order and returns the first option error. `Clone` copies
the current client defaults, applies options through the same validation path,
and returns a new client. Invalid base URLs, invalid numeric values, invalid
proxy URLs, profile option errors, and file-loading failures from certificate
options fail during construction or cloning. A caller that receives a non-nil
`*Client` receives a validated client.

> **Why**: Construction is a trust boundary. A client should either be valid and
> ready to create requests, or construction should return an error the caller can
> handle.
>
> **Rejected**: Constructors or fluent options that hide validation failures in
> logs, best-effort mutation, or later request-time surprises.

## Persistent Defaults

The full audit of every effective default value applied by `New` (and the
rationale behind each) lives in [`SPECS/30-defaults.md`](30-defaults.md).
Changes to any default value listed there are contract changes.

A `Client` MAY define reusable defaults for:

- base URL
- headers
- ordered headers
- cookies
- authentication
- profile-applied identity defaults
- retry policy
- codecs for JSON, XML, and YAML
- logger
- transport and timeout settings

These defaults apply to every request created from the client unless the request supplies a request-local value for the same concern. Client defaults are not mutated through public runtime setters; callers derive a modified client with `Clone(opts...)`.

Ordered headers preserve caller-specified insertion order as request intent. The implementation uses `github.com/kaptinlin/orderedobject` as the ordered storage model. Default `net/http` transports preserve header semantics but do not guarantee wire order; wire-order delivery is only guaranteed by transports that explicitly support ordered-header metadata.

## Transport and Timeout Policy

`Client` owns the underlying `http.Client` and transport-level configuration:

- `WithTimeout` sets the default `http.Client.Timeout`.
- `WithTransport` and `WithHTTPClient` replace the underlying transport or client.
- `WithHTTP2` enables HTTP/2 on the active `*http.Transport`.
- `WithDialTimeout`, `WithResolver`, `WithDialContext`, `WithLocalAddr`, `WithTLSHandshakeTimeout`, `WithResponseHeaderTimeout`, `WithMaxIdleConns`, `WithMaxIdleConnsPerHost`, `WithMaxConnsPerHost`, and `WithIdleConnTimeout` apply only when the underlying transport is a `*http.Transport`.
- HTTP/2 enablement configures the active `*http.Transport` instead of replacing it, preserving proxy, dialer, resolver, local address, TLS, timeout, and connection-pool settings.
- Resolver and local-address configuration use `net` package types only.
- `WithHTTPClient` MUST be applied before transport-mutating options such as `WithProxy` or `WithDialTimeout`, because replacing the client discards earlier transport mutations.

`WithHTTP2()` enables explicit HTTP/2 transport configuration during construction.

Profiles are applied at the client layer through `WithProfile`. They contribute construction options such as headers, ordered headers, and protocol preferences as reusable defaults. Request-local metadata still overrides profile-applied defaults.

## TLS and HTTP/2

`Client` owns TLS configuration and certificate material:

- `WithTLSConfig`, `WithInsecureSkipVerify`, `WithCertificates`, `WithClientCertificate`, `WithTLSServerName`, `WithRootCertificate`, and `WithRootCertificateFromString` configure client-level TLS state.
- `WithHTTP2()` configures HTTP/2 on the existing or default `*http.Transport`; custom non-`*http.Transport` implementations are left unchanged.
- `WithSession` creates a cookie jar and TLS client session cache when missing.
- File-loading construction options such as `WithClientCertificate` and `WithRootCertificate` return errors from `New` when files cannot be loaded.

`WithSession` MUST NOT replace an existing cookie jar or `TLSConfig.ClientSessionCache`.

> **Why**: TLS policy is connection-level state, so it belongs on the client instead of on individual builders.
>
> **Rejected**: Per-request TLS mutation and constructors that silently mix transport and request concerns.

## Proxy Policy

Proxy configuration belongs on `Client`:

- `WithProxy`, `WithProxyBypass`, `WithProxyFromEnv`, `WithProxies`, and `WithProxySelector` affect transport delivery and return errors when validation fails.
- `WithoutProxy` clears any configured proxy while constructing or cloning a client.
- `WithProxies` and selector-based proxy functions apply per transport attempt, including retry attempts.

## Redirect Policy

Redirect policy belongs on `Client` through `WithRedirectPolicy` and the `RedirectPolicy` interface.

The built-in policies are:

- `NewProhibitRedirectPolicy`
- `NewAllowRedirectPolicy`
- `NewSmartRedirectPolicy`
- `NewRedirectSpecifiedDomainPolicy`

Multiple redirect policies MAY be composed in one `WithRedirectPolicy` call. They run in argument order and the first error stops redirect processing.

## `net/http` Adapters

`AsHTTPClient()` returns a new `*http.Client` that snapshots the current underlying timeout, cookie jar, redirect policy, and transport. Its transport applies client-level defaults: headers, cookies, auth, and client middleware.

`AsTransport()` returns the same configured transport wrapper for callers that already own an `*http.Client`.

Adapter boundaries:

- preserve client headers, cookies, auth, middleware, timeout, cookie jar, redirect policy, and the underlying transport
- do not preserve `RequestBuilder` behavior such as request-local retry, response buffering, stream responses, decoding helpers, `Save`, or `Lines`
- clone inbound `net/http` requests before applying defaults
- do not change the meaning of `GetHTTPClient`, which remains a raw escape hatch

## Forbidden

- Do not ignore errors returned by `New`.
- Do not add public runtime setters for client defaults; derive modified clients with `Clone(opts...)`.
- Do not expect `*http.Transport`-specific timeout and pool options to mutate a custom non-`*http.Transport` transport.
- Do not expect `AsHTTPClient` or `AsTransport` to run the `RequestBuilder` pipeline.

## Contract Invariants

- All reusable configuration lives on `Client`.
- Constructor behavior and validation expectations are explicit.
- Proxy and redirect policy are defined as client-level concerns.
- `WithProxy` errors surface through `New`.
