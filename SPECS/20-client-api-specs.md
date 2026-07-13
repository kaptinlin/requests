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

Invalid or typed-nil auth, nil middleware, empty/nil redirect policies, and PEM input containing no certificates also fail construction with `ErrInvalidConfigValue`. Validation happens before values are installed, so a failed `Clone` does not mutate the base client.

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

These defaults apply with one precedence rule: client headers < client auth <
request-local headers < request-local auth. Adapter inbound headers occupy the
request-local header layer. Request cookies replace same-name client cookies
while preserving other client cookie defaults. Client defaults are not mutated
through public runtime setters; callers derive a modified client with
`Clone(opts...)`.

`WithHeaders(http.Header)` captures a clone when the option is applied. A nil
header is an empty default set, and later mutation of the caller's map or value
slices cannot change the client. `Client.Clone` owns another independent header
copy.

Ordered headers preserve caller-specified insertion order as request intent. The implementation uses `github.com/kaptinlin/orderedobject` as the ordered storage model. Default `net/http` transports preserve header semantics but do not guarantee wire order; wire-order delivery is only guaranteed by transports that explicitly support ordered-header metadata.

## Transport and Timeout Policy

`Client` owns the underlying `http.Client` and transport-level configuration:

- `WithTimeout` sets the default `http.Client.Timeout`.
- `WithTransport` and `WithHTTPClient` replace the underlying transport or client; `WithHTTPClient(nil)` is invalid.
- `WithHTTP2` enables HTTP/2 on the active `*http.Transport` and reports configuration errors during construction.
- When a standard transport option first needs a writable `*http.Transport`, it clones `http.DefaultTransport` and changes only the option's named concern. Unrelated `net/http` proxy, dial, timeout, pooling, and HTTP/2 defaults remain intact.
- `WithDialTimeout`, `WithResolver`, `WithDialContext`, `WithLocalAddr`, `WithTLSHandshakeTimeout`, `WithResponseHeaderTimeout`, `WithMaxIdleConns`, `WithMaxIdleConnsPerHost`, `WithMaxConnsPerHost`, and `WithIdleConnTimeout` apply only when the underlying transport is a `*http.Transport`; construction fails with `ErrInvalidTransportType` when they cannot apply.
- Root TLS and session options also apply only to the nil/default or active `*http.Transport`. They return `ErrInvalidTransportType` instead of replacing or partially configuring a custom transport.
- HTTP/2 enablement configures the active `*http.Transport` instead of replacing it, preserving proxy, dialer, resolver, local address, TLS, timeout, and connection-pool settings.
- Resolver and local-address configuration use `net` package types only.
- `WithHTTPClient` MUST be applied before transport-mutating options such as `WithProxy` or `WithDialTimeout`, because replacing the client discards earlier transport mutations.

`WithHTTP2()` enables explicit HTTP/2 transport configuration during construction.

Profiles are applied at the client layer through `WithProfile`. They contribute construction options such as headers, ordered headers, and protocol preferences as reusable defaults. A transport profile such as HTTP/3 is an explicit whole-transport replacement: it may replace earlier TCP dial/proxy concerns, consumes earlier TLS/session intent that it supports, and causes incompatible later root transport options to fail. Request-local metadata still overrides profile-applied defaults.

## TLS and HTTP/2

`Client` owns TLS configuration and certificate material:

- `WithTLSConfig`, `WithInsecureSkipVerify`, `WithCertificates`, `WithClientCertificate`, `WithTLSServerName`, `WithRootCertificate`, and `WithRootCertificateFromString` configure client-level TLS state.
- `WithTLSConfig` captures `tls.Config.Clone()` when the option is applied. This
  gives the client an independent top-level config but deliberately preserves
  the standard library's shallow-clone semantics for referenced slices, maps,
  callbacks, certificate pools, session caches, key loggers, private keys, and
  parsed certificate leaves. Callers MUST NOT mutate those referenced values
  while the client may use them.
- `WithTLSConfig(nil)` clears TLS state established by earlier root TLS options
  and assigns nil to the active standard transport's `TLSClientConfig`. It is
  still a root transport option and fails on a custom non-`*http.Transport`
  instead of leaving client state and live transport state inconsistent.
- `WithCertificates` additionally owns copies of the certificate slice, DER
  bytes, supported-signature slice, OCSP staple, and SCT bytes. Private keys and
  parsed leaves remain borrowed opaque collaborators.
- `WithHTTP2()` configures HTTP/2 on the existing or default `*http.Transport`; custom non-`*http.Transport` implementations fail with `ErrInvalidTransportType`.
- `WithSession` creates a cookie jar and TLS client session cache when missing.
- `WithSession` is atomic: if the active transport cannot use the TLS session config, construction fails before installing the jar or cache.
- File-loading construction options such as `WithClientCertificate` and `WithRootCertificate` return errors from `New` when files cannot be loaded.
- Root certificate options also reject readable input that contains no valid PEM certificates; they do not log and continue with an empty pool.

`WithSession` MUST NOT replace an existing cookie jar or `TLSConfig.ClientSessionCache`.

`GetTLSConfig` and `Client.Clone` return or retain another standard shallow
`tls.Config.Clone`. They do not expose the client's top-level config pointer and
do not claim to deep-copy opaque collaborators.

> **Why**: TLS policy is connection-level state, so it belongs on the client instead of on individual builders.
>
> **Rejected**: Per-request TLS mutation and constructors that silently mix transport and request concerns.

## Proxy Policy

Proxy configuration belongs on `Client`:

- `WithProxy`, `WithProxyBypass`, `WithProxyFromEnv`, `WithProxies`, and `WithProxySelector` affect transport delivery and return errors when validation fails.
- `WithoutProxy` clears any configured proxy while constructing or cloning a client.
- With no explicit proxy option, the nil/default transport follows `http.ProxyFromEnvironment`. `WithoutProxy` materializes a standard transport with `Proxy == nil` for explicit direct delivery.
- `WithProxies` and selector-based proxy functions apply per transport attempt, including retry attempts.
- Explicit proxy URLs accept only `http`, `https`, and `socks5` schemes.

## Redirect Policy

Redirect policy belongs on `Client` through `WithRedirectPolicy` and the `RedirectPolicy` interface.

The built-in policies are:

- `NewProhibitRedirectPolicy`
- `NewAllowRedirectPolicy`
- `NewRedirectSpecifiedDomainPolicy`

Multiple redirect policies MAY be composed in one `WithRedirectPolicy` call. They run in argument order and the first error stops redirect processing.

Policies only admit, reject, or limit redirects and apply the package's
credential-forwarding rule. `net/http` exclusively constructs each redirect
request, including method, body, `GetBody`, Referer, Cookie, and payload-header
transitions.

`NewAllowRedirectPolicy` retains sensitive credentials from the initial request
only when the target has the same scheme, canonical hostname, and effective
port. Hostname comparison canonicalizes IDNA names and parsed IP literals;
omitted HTTP and HTTPS ports mean 80 and 443. A hostname, scheme, or effective
port change removes Authorization, proxy authorization, and explicit cookie
headers. Cookie Jar values remain owned by `net/http` and follow their domain,
path, and secure scope.

## `net/http` Adapters

`AsHTTPClient()` returns a new `*http.Client` that snapshots the current underlying timeout, cookie jar, redirect policy, and transport. Its transport applies client-level defaults: headers, cookies, auth, and client middleware.

`AsTransport()` returns the same configured transport wrapper for callers that already own an `*http.Client`.

Adapter boundaries:

- preserve client headers, cookies, auth, middleware, timeout, cookie jar, redirect policy, and the underlying transport
- do not preserve `RequestBuilder` behavior such as request-local retry, response buffering, stream responses, decoding helpers, `Save`, or `Lines`
- clone inbound `net/http` requests before applying defaults
- apply the same header/auth and name-based cookie precedence as builders
- do not change the meaning of `UnsafeHTTPClient`, which remains a raw mutable escape hatch

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
