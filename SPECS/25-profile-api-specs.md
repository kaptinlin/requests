# Profile API Specs

## Overview

`Profile` expresses a coherent client identity as construction-time options. A profile may contribute default headers, ordered headers, protocol preferences, transport configuration, and fingerprint hooks.

Profiles are not request builders. They configure reusable client defaults before requests are created.

## Contract

The profile contract is:

```go
type Profile interface {
	Name() string
	Options() []Option
}
```

`WithProfile(profile)` applies the returned options in order during `New` or `Clone`. If a profile option fails, construction fails with profile context.

## Scope

Profiles MAY contribute:

- default headers
- ordered headers
- protocol preferences such as HTTP/2
- transport profiles such as HTTP/3
- TLS fingerprint configuration in extension modules

Profiles MUST NOT apply request-local state. Request-local headers and ordered headers continue to override profile defaults according to `SPECS/21-request-builder-api-specs.md`.

## Package Boundary

The root package defines the profile interface and option only. Concrete profiles SHOULD live in small extension packages such as `browser`, `fingerprint`, and `http3`, so root APIs stay stable and do not accumulate browser-version or transport dependency details.

## Forbidden

- Do not expose browser version details as root package methods.
- Do not make profile application per-request.
- Do not use profile as a general mutation hook.
- Do not silently change TLS verification defaults from a profile.

## Contract Invariants

- Profiles are client-level only.
- Profile errors surface through `New` or `Clone`.
- `WithProfile` preserves construction-time option ordering.
- Request-local metadata overrides profile defaults.
