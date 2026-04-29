# Profile API Specs

## Overview

`Profile` expresses a coherent client identity as a single client-level intent. A profile may apply default headers, ordered headers, protocol preferences, and future fingerprint hooks.

Profiles are not request builders. They configure reusable client defaults before requests are created.

## Contract

The profile contract is:

```go
type Profile interface {
	Name() string
	Apply(*Client) error
}
```

`Client.ApplyProfile(profile)` applies a profile and returns profile errors with context.

`WithProfile(profile)` is the functional-option form. Like other fluent options that cannot return errors, it keeps construction lightweight and logs failures only when a logger is already configured.

## Scope

Profiles MAY apply:

- default headers
- ordered headers
- protocol preferences such as HTTP/2
- future transport or fingerprint hooks

Profiles MUST NOT apply request-local state. Request-local headers and ordered headers continue to override profile defaults according to `SPECS/21-request-builder-api-specs.md`.

## Package Boundary

The root package defines the profile interface and option only. Concrete profiles SHOULD live in small extension packages such as `browser`, `fingerprint`, and `http3`, so root APIs stay stable and do not accumulate browser-version or transport dependency details.

## Forbidden

- Do not expose browser version details as root package methods.
- Do not make profile application per-request.
- Do not use `any` middleware-style profile dispatch.
- Do not silently change TLS verification defaults from a profile.

## Acceptance Criteria

- [ ] Profiles are client-level only.
- [ ] Profile errors are available through `Client.ApplyProfile`.
- [ ] `WithProfile` preserves the existing functional option pattern.
- [ ] Request-local metadata overrides profile defaults.
