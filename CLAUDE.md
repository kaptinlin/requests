# requests

Fluent HTTP client library for Go built around `Client`, `RequestBuilder`, and `Response`. It wraps `net/http` with builder-style request construction, retries, redirects, proxy controls, middleware, streaming callbacks, ordered-header intent, coherent profiles, and JSON/XML/YAML codecs.

For usage examples and installation, see [README.md](README.md).

## Commands

```bash
task test          # Run root-module tests with race detection
task test:all      # Run race tests for root and extension modules
task lint          # Run golangci-lint and root go mod tidy checks
task lint:all      # Run lint for root and extension modules
task tidy:all      # Run go mod tidy for root and extension modules
task verify        # Run deps, fmt, vet, lint, test, and vuln checks for root
task fmt           # Format root Go code
task vet           # Run go vet for root
task vuln          # Run govulncheck for root
task deps          # Download and tidy root dependencies
task clean         # Clean build artifacts and caches
```

Use direct module commands when a task touches only one extension:

```bash
cd browser && go test -race ./...
cd fingerprint && go test -race ./...
cd http3 && go test -race ./...
```

## Architecture

```text
requests/
├── client.go        # Client construction, defaults, transport, TLS, profile, and verb helpers
├── client_option.go # Functional options for New(...)
├── request.go       # RequestBuilder state, body handling, middleware, retries, and dispatch
├── response.go      # Buffered responses, decoding helpers, save, TLS, and line iteration
├── ordered_headers.go # Ordered header metadata and merge helpers
├── profile.go       # Core Profile interface and application hook
├── retry.go         # Backoff strategies and Retry-After handling
├── redirect.go      # Redirect policies and sensitive-header handling
├── proxy.go         # Proxy validation, bypass rules, and proxy rotation
├── auth.go          # Basic, bearer, and custom authorization methods
├── logger.go        # Logger interface and slog-backed default logger
├── codec.go         # Encoder and decoder abstractions
├── form.go          # Form and multipart parsing helpers
├── stream.go        # Streaming callback types and limits
├── middlewares/     # Header, cookie, and cache middleware
├── browser/         # Optional browser identity profile module
├── fingerprint/     # Optional uTLS ClientHello fingerprint module
├── http3/           # Optional QUIC HTTP/3 transport module
└── SPECS/           # Contract-level API and architecture specifications
```

The root module is `github.com/kaptinlin/requests`. Extension modules are independently consumable packages organized locally with `go.work`; keep their `go.mod` files publishable and do not add local `replace` directives.

## Agent Workflow

### Design Phase — Read SPECS First

Before designing or modifying code, read the relevant files in [`SPECS/`](SPECS/). The specs define the boundaries between `Client`, `RequestBuilder`, `Response`, middleware, retries, streaming, profiles, transport policy, and response decoding.

Workflow:

1. Identify the relevant specs in the SPECS Index below.
2. Read those specs completely before changing code.
3. Keep new behavior inside the existing client/builder/response/profile boundaries.
4. Ask the user before inventing a new public pattern that is not covered by the specs.

### Implementation Phase — Respect Module Boundaries

Core package changes must not pull optional browser, uTLS, or QUIC dependencies into the root module. Put optional protocol or identity behavior in extension modules unless the relevant spec explicitly says it belongs in core.

For multi-module work:

1. Use local `go.work` for development across root and extensions.
2. Run `task test:all`, `task tidy:all`, and module-specific lint before finishing.
3. Remember extension modules without local `replace` depend on the published root module when `GOWORK=off`.

## SPECS Index

Specification documents in [`SPECS/`](SPECS/) define system contracts, API rules, and architecture decisions:

| Spec | Topic |
|------|-------|
| [`SPECS/00-overview.md`](SPECS/00-overview.md) | Package model, request lifecycle, and object boundaries |
| [`SPECS/20-client-api-specs.md`](SPECS/20-client-api-specs.md) | Client construction, defaults, TLS, transport, proxy, and redirect policy |
| [`SPECS/21-request-builder-api-specs.md`](SPECS/21-request-builder-api-specs.md) | Builder state, path/query/body handling, request-local overrides, and dispatch |
| [`SPECS/22-response-api-specs.md`](SPECS/22-response-api-specs.md) | Buffered response helpers, decoding, save behavior, TLS, and line iteration |
| [`SPECS/23-streaming-api-specs.md`](SPECS/23-streaming-api-specs.md) | Streaming callbacks, delivery rules, and buffer limits |
| [`SPECS/24-logging-api-specs.md`](SPECS/24-logging-api-specs.md) | Logger interface and default logger behavior |
| [`SPECS/25-profile-api-specs.md`](SPECS/25-profile-api-specs.md) | Profile contract, package boundaries, and client-level identity rules |
| [`SPECS/40-middleware-architecture-specs.md`](SPECS/40-middleware-architecture-specs.md) | Middleware composition, ordering, and built-in middleware rules |
| [`SPECS/41-retry-and-delivery-specs.md`](SPECS/41-retry-and-delivery-specs.md) | Retry counts, backoff strategies, Retry-After handling, and cancellation |

## Design Philosophy

- **KISS** — Keep request construction centered on `Client`, `RequestBuilder`, `Response`, and `Profile`; avoid parallel APIs for the same operation.
- **YAGNI** — Prefer a narrow fluent API over convenience wrappers that only hide one call site or one transport quirk.
- **OCP** — Extend behavior through middleware, codecs, retry strategies, redirect policies, proxy selectors, profiles, and optional modules.
- **ISP** — Keep interfaces small; `AuthMethod`, `Logger`, `Encoder`, `Decoder`, and `Profile` each have one focused role.
- **APIs as language** — Calls should read like a request script: `client.Post(...).Header(...).JSONBody(...).Send(ctx)`.
- **Never:** accidental complexity, feature gravity, abstraction theater, configurability cope.

## API Design Principles

- **Progressive Disclosure**: `New`, `URL`, and verb helpers cover the common path; transport, proxy, TLS, redirect, codec, retry, profile, and extension hooks stay available when needed.
- **Default Passthrough**: zero values on `Config` preserve `net/http` defaults unless the caller opts into custom transport behavior.
- **Core stays light**: optional browser headers, TLS fingerprints, and HTTP/3 live in extension modules so ordinary users do not pay their dependency cost.
- **Request snapshot model**: once `Send` starts, later `Client` mutations must not affect the in-flight request.

## Coding Rules

### Must Follow

- Go 1.26.2 — use modern Go features already present in this repo when they simplify code.
- Follow [Google Go Best Practices](https://google.github.io/go-style/best-practices).
- Follow [Google Go Style Decisions](https://google.github.io/go-style/decisions).
- KISS/DRY/YAGNI — no premature abstractions, no unused features, no duplicated logic.
- Keep shared defaults on `Client` and one-shot request state on `RequestBuilder`.
- Keep profile behavior client-level; request-local headers and ordered headers override profile defaults.
- Keep extension modules publishable; do not encode local workspace relationships in extension `go.mod` files.
- Return errors instead of panicking; preserve context with wrapped errors.
- Use `httptest.Server` or local protocol servers for request/response behavior tests.

### Go 1.26 Features

| Feature | Where Used |
|---------|------------|
| `maps.Copy` / `maps.Clone` | Redirect header preservation and builder map cloning |
| `slices.Clone` / `slices.DeleteFunc` / `slices.Backward` / `slices.Contains` | Response copies, request mutation helpers, middleware ordering, ALPN checks |
| `strings.SplitSeq` | NO_PROXY parsing in `proxy.go` |
| `iter.Seq` | `Response.Lines()` iterator API |
| `log/slog` | Default logger and HTTP/3 extension logger option |
| `errors.Join` / `errors.AsType` | Config validation, aggregated retry failures, and transport error classification |
| `sync.OnceValue` | Lazy uTLS session cache initialization in `fingerprint` |
| `t.Context()` | Tests and examples that need a scoped context |
| `for range N` | Retry and rotation tests |

### Forbidden

- No `panic` in production code.
- No premature abstraction — three similar lines are better than a helper used once.
- No feature creep — only implement behavior supported by the package contracts.
- No per-request TLS or protocol mutation.
- No hidden dependency bloat in the root module; heavy optional protocols belong in extension modules.
- No encoding spec prose as runtime code; keep rules in `SPECS/` and executable behavior in source.
- No working around dependency bugs — if a dependency is the problem, write a report in `reports/` instead of reimplementing it here.

## Dependency Issue Reporting

When you encounter a bug, limitation, or unexpected behavior in a dependency library:

1. Do **not** work around it by reimplementing the dependency's functionality.
2. Do **not** skip the dependency and write a local replacement.
3. Create a report file: `reports/<dependency-name>.md`.
4. Include the dependency version, trigger scenario, expected behavior, actual behavior, errors, and any non-code workaround suggestion.
5. Continue with tasks that do not depend on the broken functionality.

## Testing

- Tests live beside source in `*_test.go` files.
- Use `github.com/stretchr/testify/assert` and `github.com/test-go/testify/require` following existing local style.
- Use `httptest.Server` for HTTP behavior, redirects, retries, cookies, and response parsing.
- Use local protocol servers for optional transport behavior, such as HTTP/3.
- Keep examples runnable with `go test`.
- Prefer `t.Context()` over `context.Background()` in tests.

```bash
go test -race ./...                    # Run all root tests directly
go test -run TestClientGetRequest ./... # Run a focused root test
go test -run Example ./...             # Run examples
```

## Dependencies

Core module dependencies:

| Dependency | Purpose |
|------------|---------|
| `github.com/go-json-experiment/json` | JSON encoding and decoding |
| `github.com/goccy/go-yaml` | YAML encoding and decoding |
| `github.com/google/go-querystring` | Struct-to-query encoding |
| `github.com/kaptinlin/orderedobject` | Ordered header intent |
| `github.com/valyala/bytebufferpool` | Response buffer pooling |
| `golang.org/x/net` | HTTP/2 transport support and public suffix support |

Optional extension dependencies:

| Module | Dependency | Purpose |
|--------|------------|---------|
| `browser` | `github.com/kaptinlin/orderedobject` | Browser-like ordered header profiles |
| `fingerprint` | `github.com/refraction-networking/utls` | TLS ClientHello fingerprint profiles |
| `http3` | `github.com/quic-go/quic-go` | QUIC HTTP/3 transport profiles |

## Error Handling

- Sentinel errors live in `errors.go` for unsupported content types, redirects, invalid transport usage, and configuration failures.
- Use `IsTimeout` and `IsConnectionError` to classify transport failures.
- Keep configuration validation in `Config.Validate()` and return joined errors for multiple invalid fields.
- Functional options that cannot return errors may log when a logger is already configured; use direct setters when callers need fail-fast behavior.

## Linting

- golangci-lint config lives in [`.golangci.yml`](.golangci.yml).
- `task lint` checks root lint plus root `go.mod` and `go.sum` tidiness.
- `task lint:all` lints root and extension modules.
- If the worktree intentionally changes `go.mod` or `go.sum`, run module lint commands directly to separate lint failures from expected tidy diffs.

## Agent Skills

Specialized skills live in [`.agents/skills/`](.agents/skills/). Use the smallest skill set that matches the task.

| Skill | When to Use |
|-------|-------------|
| [`agent-md-writing`](.agents/skills/agent-md-writing/) | Updating `CLAUDE.md` / `AGENTS.md` development instructions |
| [`go-best-practices`](.agents/skills/go-best-practices/) | Writing or reviewing idiomatic Go APIs, errors, tests, naming, and concurrency |
| [`modernizing`](.agents/skills/modernizing/) | Adopting Go 1.20-1.26 features when they simplify code |
| [`code-simplifying`](.agents/skills/code-simplifying/) | Refining recently written code for clarity without changing behavior |
| [`code-refactoring`](.agents/skills/code-refactoring/) | Larger refactors after feature work or when architecture smells accumulate |
| [`dependency-selecting`](.agents/skills/dependency-selecting/) | Choosing Go dependencies and keeping optional costs out of core |
| [`golangci-linting`](.agents/skills/golangci-linting/) | Running, configuring, or fixing golangci-lint v2 issues |
| [`multimodule-initializing`](.agents/skills/multimodule-initializing/) | Adding or maintaining extension modules under `go.work` |
| [`taskfile-configuring`](.agents/skills/taskfile-configuring/) | Updating Taskfile workflows for root and extension modules |
| [`readme-writing`](.agents/skills/readme-writing/) | Updating user-facing README content |
| [`spec-writing`](.agents/skills/spec-writing/) | Creating or updating contract-level specs |
| [`spec-reviewing`](.agents/skills/spec-reviewing/) | Reviewing specs for completeness, consistency, and over-engineering |
| [`tdd-planning`](.agents/skills/tdd-planning/) | Planning non-trivial behavior before implementation |
| [`tdd-implementing`](.agents/skills/tdd-implementing/) | Implementing behavior with strict red-green-refactor cycles |
| [`committing`](.agents/skills/committing/) | Creating conventional commits |
| [`releasing`](.agents/skills/releasing/) | Preparing semantic-version releases and tags |
