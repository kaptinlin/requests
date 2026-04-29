# Client API Specs

## Overview

`Client` owns reusable HTTP configuration. This spec defines construction, persistent defaults, transport policy, proxy and redirect controls, and the rules that apply before a request becomes a `RequestBuilder`.

## Construction

The package exposes three client constructors:

- `New(opts ...ClientOption)` for fluent option-based setup.
- `URL(baseURL string)` for a short-form client with only a base URL.
- `Create(config *Config)` for struct-based initialization.

`Create` returns `*Client` and does not surface configuration errors directly. Code that assembles a rich `Config` SHOULD call `Config.Validate()` before `Create` when it needs deterministic validation of malformed URLs or invalid numeric values.

> **Why**: Construction stays lightweight and allocation-oriented, while explicit validation remains available for callers that need stricter setup guarantees.
>
> **Rejected**: A single constructor that forces every caller through an error-returning setup path.

## Persistent Defaults

A `Client` MAY define reusable defaults for:

- base URL
- headers
- cookies
- authentication
- retry policy
- codecs for JSON, XML, and YAML
- logger
- transport and timeout settings

These defaults apply to every request created from the client unless the request supplies a request-local value for the same concern.

## Transport and Timeout Policy

`Client` owns the underlying `http.Client` and transport-level configuration:

- `SetDefaultTimeout` sets the default `http.Client.Timeout`.
- `SetDefaultTransport` and `SetHTTPClient` replace the underlying transport or client.
- `SetDialTimeout`, `SetResolver`, `SetDialContext`, `SetLocalAddr`, `SetTLSHandshakeTimeout`, `SetResponseHeaderTimeout`, `SetMaxIdleConns`, `SetMaxIdleConnsPerHost`, `SetMaxConnsPerHost`, and `SetIdleConnTimeout` apply only when the underlying transport is a `*http.Transport`.
- Resolver and local-address configuration use standard-library types only.
- `WithHTTPClient` MUST be applied before transport-mutating options such as `WithProxy` or `WithDialTimeout`, because replacing the client discards earlier transport mutations.

`WithHTTP2()` is the functional-option equivalent of `Config.HTTP2`.

## TLS and HTTP/2

`Client` owns TLS configuration and certificate material:

- `SetTLSConfig`, `InsecureSkipVerify`, `SetCertificates`, `SetClientCertificate`, `SetTLSServerName`, `SetRootCertificate`, `SetRootCertificateFromString`, `SetClientRootCertificate`, and `SetClientRootCertificateFromString` mutate client-level TLS state.
- `Config.HTTP2` enables `http2.Transport` only during construction.
- `WithSession` and `EnableSession` create a cookie jar and TLS client session cache when missing.
- File-loading helpers such as `SetClientCertificate` and `SetRootCertificate` are best-effort: they log when a logger is configured and keep returning the same client for continued chaining.

`EnableSession` MUST NOT replace an existing cookie jar or `TLSConfig.ClientSessionCache`.

> **Why**: TLS policy is connection-level state, so it belongs on the client instead of on individual builders.
>
> **Rejected**: Per-request TLS mutation and constructors that silently mix transport and request concerns.

## Proxy Policy

Proxy configuration belongs on `Client`:

- `SetProxy`, `SetProxyWithBypass`, `SetProxyFromEnv`, `SetProxies`, and `SetProxySelector` affect transport delivery and return errors when validation fails.
- `RemoveProxy` clears any configured proxy.
- `SetProxies` and selector-based proxy functions apply per transport attempt, including retry attempts.
- `WithProxy` preserves the fluent option pattern and therefore ignores proxy-parse errors. Use `SetProxy` directly when proxy validation must fail fast.

## Redirect Policy

Redirect policy belongs on `Client` through `SetRedirectPolicy` and the `RedirectPolicy` interface.

The built-in policies are:

- `NewProhibitRedirectPolicy`
- `NewAllowRedirectPolicy`
- `NewSmartRedirectPolicy`
- `NewRedirectSpecifiedDomainPolicy`

Multiple redirect policies MAY be composed in one `SetRedirectPolicy` call. They run in argument order and the first error stops redirect processing.

## Standard Library Adapters

`AsHTTPClient()` returns a new `*http.Client` that snapshots the current underlying timeout, cookie jar, redirect policy, and transport. Its transport applies client-level defaults: headers, cookies, auth, and client middleware.

`AsTransport()` returns the same configured transport wrapper for callers that already own an `*http.Client`.

Adapter boundaries:

- preserve client headers, cookies, auth, middleware, timeout, cookie jar, redirect policy, and the underlying transport
- do not preserve `RequestBuilder` behavior such as request-local retry, response buffering, streaming callbacks, decoding helpers, `Save`, or `Lines`
- clone inbound standard-library requests before applying defaults
- do not change the meaning of `Client.HTTPClient` or `GetHTTPClient`, which remain raw escape hatches

## Forbidden

- Do not chain mutators that return `void`, including `SetDefaultHeader`, `SetDefaultHeaders`, `SetDefaultCookie`, `SetHTTPClient`, and `AddMiddleware`.
- Do not rely on `WithProxy` to report invalid proxy configuration.
- Do not expect `*http.Transport`-specific timeout and pool setters to mutate a custom non-`*http.Transport` transport.
- Do not expect `AsHTTPClient` or `AsTransport` to run the `RequestBuilder` pipeline.

## Acceptance Criteria

- [ ] All reusable configuration lives on `Client`.
- [ ] Constructor behavior and validation expectations are explicit.
- [ ] Proxy and redirect policy are defined as client-level concerns.
- [ ] The distinction between error-returning proxy setters and fluent `WithProxy` is explicit.

**Origin:** Migrated from `docs/client.md`.
