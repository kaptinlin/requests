# requests

Fluent HTTP client library for Go built around `Client`, `RequestBuilder`, and `Response`. It wraps `net/http` with builder-style request construction, retries, redirect and proxy controls, middleware, streaming callbacks, and JSON/XML/YAML codecs.

For usage examples and installation, see [README.md](README.md).

## Commands

```bash
task test          # Run all tests with race detection
task lint          # Run golangci-lint and go mod tidy checks
task verify        # Run deps, fmt, vet, lint, test, and vuln checks
task fmt           # Format Go code
task vet           # Run go vet
task vuln          # Run govulncheck
task deps          # Download and tidy dependencies
task clean         # Clean build artifacts and caches
```

## Architecture

```text
requests/
├── client.go        # Client construction, defaults, transport, TLS, and verb helpers
├── client_option.go # Functional options for New(...)
├── request.go       # RequestBuilder state, body handling, middleware, retries, and dispatch
├── response.go      # Buffered responses, decoding helpers, save, and line iteration
├── retry.go         # Backoff strategies and Retry-After handling
├── redirect.go      # Redirect policies and sensitive-header handling
├── proxy.go         # Proxy validation, bypass rules, and proxy rotation
├── auth.go          # Basic, bearer, and custom authorization methods
├── logger.go        # Logger interface and slog-backed default logger
├── codec.go         # Encoder and decoder abstractions
├── json.go          # JSON codec implementation
├── xml.go           # XML codec implementation
├── yaml.go          # YAML codec implementation
├── form.go          # Form and multipart parsing helpers
├── stream.go        # Streaming callback types and limits
├── middlewares/     # Header, cookie, and cache middleware
└── SPECS/           # Contract-level API and architecture specifications
```

## Agent Workflow

### Design Phase — Read SPECS First

Before designing or modifying code, read the relevant files in [`SPECS/`](SPECS/). The specs define the boundaries between `Client`, `RequestBuilder`, and `Response`, plus the rules for retries, redirects, middleware, streaming, and response decoding.

Workflow:

1. Identify the relevant specs in the SPECS Index below.
2. Read those specs completely before changing code.
3. Keep new behavior inside the existing client/builder/response boundaries.
4. Ask the user before inventing a new public pattern that is not covered by the specs.

## SPECS Index

Specification documents in [`SPECS/`](SPECS/) — system contracts, API rules, and architecture decisions:

| Spec | Topic |
|------|-------|
| [`SPECS/00-overview.md`](SPECS/00-overview.md) | Package model, request lifecycle, and object boundaries |
| [`SPECS/20-client-api-specs.md`](SPECS/20-client-api-specs.md) | Client construction, transport defaults, TLS, proxy, and redirect policy |
| [`SPECS/21-request-builder-api-specs.md`](SPECS/21-request-builder-api-specs.md) | Builder state, path/query/body handling, request-local overrides, and dispatch |
| [`SPECS/22-response-api-specs.md`](SPECS/22-response-api-specs.md) | Buffered response helpers, decoding, save behavior, and line iteration |
| [`SPECS/23-streaming-api-specs.md`](SPECS/23-streaming-api-specs.md) | Streaming callbacks, delivery rules, and buffer limits |
| [`SPECS/24-logging-api-specs.md`](SPECS/24-logging-api-specs.md) | Logger interface and default logger behavior |
| [`SPECS/40-middleware-architecture-specs.md`](SPECS/40-middleware-architecture-specs.md) | Middleware composition, ordering, and built-in middleware rules |
| [`SPECS/41-retry-and-delivery-specs.md`](SPECS/41-retry-and-delivery-specs.md) | Retry counts, backoff strategies, Retry-After handling, and cancellation |

## Design Philosophy

- **KISS** — Keep request construction centered on `Client`, `RequestBuilder`, and `Response`; avoid adding parallel APIs for the same operation.
- **YAGNI** — Prefer a narrow fluent surface over convenience helpers that only wrap one call site or one transport quirk.
- **Open-Closed (OCP)** — Extend behavior through middleware, codecs, retry strategies, redirect policies, proxy selectors, and functional options instead of modifying core flow.
- **APIs as language** — Calls should read like a request script: `client.Post(...).Header(...).JSONBody(...).Send(ctx)`.
- **Errors as teachers** — Return typed or wrapped errors that preserve the transport or configuration failure instead of hiding it.
- **Never:** accidental complexity, feature gravity, abstraction theater, configurability cope.

## API Design Principles

- **Progressive Disclosure**: `New`, `URL`, and verb helpers cover the common path, while transport, proxy, TLS, redirect, codec, and retry hooks stay available when needed.
- **Default Passthrough**: Zero values on `Config` preserve `net/http` defaults unless the caller opts into custom transport behavior.

## Coding Rules

### Must Follow

- Go 1.26.2 — use modern standard-library features already present in this repo when they simplify code.
- Follow [Google Go Best Practices](https://google.github.io/go-style/best-practices).
- Follow [Google Go Style Decisions](https://google.github.io/go-style/decisions).
- KISS/DRY/YAGNI — no premature abstractions, no unused features, no duplicated logic.
- Keep interfaces small — `AuthMethod`, `Logger`, `Encoder`, and `Decoder` stay focused.
- Return errors instead of panicking; preserve context with wrapped errors.
- Keep shared defaults on `Client` and one-shot request state on `RequestBuilder`.
- Preserve the request snapshot model: once `Send` starts, later `Client` mutations must not affect the in-flight request.
- Use `httptest.Server` for request/response behavior tests.

### Go 1.26 Features

| Feature | Where Used |
|---------|-----------|
| `maps.Copy` | Request path and header copying in builder and redirect handling |
| `slices.Clone` / `slices.DeleteFunc` / `slices.Backward` | Response buffering, request mutation helpers, and middleware ordering |
| `strings.SplitSeq` | NO_PROXY parsing in `proxy.go` |
| `iter.Seq` | `Response.Lines()` iterator API |
| `log/slog` | Default logger implementation |
| `errors.Join` | Config validation and aggregated retry failures |

### Forbidden

- No `panic` in production code.
- No premature abstraction — three similar lines are better than a helper used once.
- No feature creep — only implement behavior supported by the package contracts.
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
- Use `httptest.Server` for transport behavior, redirects, retries, cookies, and response parsing.
- Use `github.com/stretchr/testify/assert` and `github.com/test-go/testify/require` for assertions.
- Keep examples runnable with `go test`.

```bash
go test -race ./...                   # Run all tests directly
go test -run TestClientGetRequest ./... # Run a focused test
go test -run Example ./...            # Run examples
```

## Dependencies

| Dependency | Purpose |
|------------|---------|
| `github.com/go-json-experiment/json` | JSON encoding and decoding |
| `github.com/goccy/go-yaml` | YAML encoding and decoding |
| `github.com/google/go-querystring` | Struct-to-query encoding |
| `github.com/valyala/bytebufferpool` | Response buffer pooling |
| `golang.org/x/net` | HTTP/2 transport support |

## Error Handling

- Sentinel errors live in `errors.go` for unsupported content types, redirects, invalid transport usage, and configuration errors.
- Use `IsTimeout` and `IsConnectionError` to classify transport failures.
- Keep configuration validation in `Config.Validate()` and return joined errors for multiple invalid fields.

## Linting

- golangci-lint v2.9.0. Config lives in [`.golangci.yml`](.golangci.yml).
- `task lint` also checks that `go.mod` and `go.sum` stay tidy.

## Agent Skills

Specialized skills are normally exposed through `.claude/skills/`, but this checkout currently has a broken `.claude/skills` symlink because the `.agents/skills` submodule is missing. Reinitialize the skills checkout before relying on those paths.
