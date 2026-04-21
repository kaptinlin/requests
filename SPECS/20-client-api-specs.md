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
- `SetDialTimeout`, `SetTLSHandshakeTimeout`, `SetResponseHeaderTimeout`, `SetMaxIdleConns`, `SetMaxIdleConnsPerHost`, `SetMaxConnsPerHost`, and `SetIdleConnTimeout` apply only when the underlying transport is a `*http.Transport`.
- `WithHTTPClient` MUST be applied before transport-mutating options such as `WithProxy` or `WithDialTimeout`, because replacing the client discards earlier transport mutations.

## TLS and HTTP/2

`Client` owns TLS configuration and certificate material:

- `SetTLSConfig`, `InsecureSkipVerify`, `SetCertificates`, `SetClientCertificate`, `SetTLSServerName`, `SetRootCertificate`, `SetRootCertificateFromString`, `SetClientRootCertificate`, and `SetClientRootCertificateFromString` mutate client-level TLS state.
- `Config.HTTP2` enables `http2.Transport` only during construction.
- File-loading helpers such as `SetClientCertificate` and `SetRootCertificate` are best-effort: they log when a logger is configured and keep returning the same client for continued chaining.

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

## Forbidden

- Do not chain mutators that return `void`, including `SetDefaultHeader`, `SetDefaultHeaders`, `SetDefaultCookie`, `SetHTTPClient`, and `AddMiddleware`.
- Do not rely on `WithProxy` to report invalid proxy configuration.
- Do not expect `*http.Transport`-specific timeout and pool setters to mutate a custom non-`*http.Transport` transport.

## Acceptance Criteria

- [ ] All reusable configuration lives on `Client`.
- [ ] Constructor behavior and validation expectations are explicit.
- [ ] Proxy and redirect policy are defined as client-level concerns.
- [ ] The distinction between error-returning proxy setters and fluent `WithProxy` is explicit.

**Origin:** Migrated from `docs/client.md`.
