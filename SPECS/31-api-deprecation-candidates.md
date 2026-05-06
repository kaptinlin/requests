# API Deprecation Candidates

This spec records public symbols that look redundant, accidental, or
overly broad. Listing a symbol here is a *proposal*, not a commitment;
nothing in 1.x is removed or renamed without an explicit major bump.

The audit was produced by walking `go doc -all .` on the root module
and asking three questions per symbol:

1. Is the symbol still necessary?
2. Will the name still read clearly in ten years?
3. Does it overlap meaningfully with another symbol?

A symbol that fails at least one question is a candidate. Symbols not
listed here are considered settled.

## Construction

| Symbol | Concern | Proposed action (≥ 2.0) |
|---|---|---|
| [`URL(baseURL string) *Client`](../client.go) | Three constructors (`New`, `URL`, `Create`) for the same object. `URL` is a one-line wrapper around `Create(&Config{BaseURL: baseURL})`. | Keep `New` (functional options) and `Create` (explicit `*Config`). Drop `URL` in 2.0; until then, leave a godoc note pointing callers at `New(WithBaseURL(...))`. |

## Marshal/Unmarshal pairs

| Symbol | Concern | Proposed action (≥ 2.0) |
|---|---|---|
| `Client.SetJSONMarshal` / `SetJSONUnmarshal` | Each codec already has a swappable [Encoder]/[Decoder] field on `Client`. The marshal-func setters are a parallel API for the same operation, expressed at a lower abstraction level (`func(any) ([]byte, error)`). Two ways to do the same thing. | Drop in 2.0 in favor of `Client.JSONEncoder` / `JSONDecoder` field assignment. `WithJSONMarshal` / `WithJSONUnmarshal` go with them. |
| `SetXMLMarshal` / `SetXMLUnmarshal` / `WithXMLMarshal` / `WithXMLUnmarshal` | Same as JSON. | Same as JSON. |
| `SetYAMLMarshal` / `SetYAMLUnmarshal` / `WithYAMLMarshal` / `WithYAMLUnmarshal` | Same as JSON. | Same as JSON. |

## Buffer pool

| Symbol | Concern | Proposed action (≥ 2.0) |
|---|---|---|
| `GetBuffer()` / `PutBuffer(b)` | Internal allocation primitives surfaced as part of the public API. No external caller has a reason to share the same `bytebufferpool.Pool` as the response decoder. | Unexport in 2.0. |
| `MaxStreamBufferSize` const | Implementation detail of `Response.Lines` / streaming. Exposing it invites callers to depend on a value that may need tuning. | Unexport in 2.0. |
| `DirPermissions` const | Used only by `Response.Save` to mkdir parent directories. Not part of the contract callers should rely on. | Unexport in 2.0. |

## TLS surface

| Symbol | Concern | Proposed action (≥ 2.0) |
|---|---|---|
| `Client.SetClientRootCertificate` / `SetClientRootCertificateFromString` / `WithClientRootCertificate` | Implementation appends to `tls.Config.ClientCAs`. `ClientCAs` is meaningful on the *server* side (verifying client certs presented during mTLS); on the client side it has no documented effect. The public surface implies a capability the client cannot deliver. | Confirm there is no transport that consults `ClientCAs` in this codebase, then drop in 2.0. Until then, keep but add a godoc warning. |
| `Client.SetCertificates` (variadic) vs `Client.SetClientCertificate(cert, key)` | Two paths to install a TLS client certificate: pre-loaded `tls.Certificate` values vs file paths. Both are real use cases; not a candidate but documented for completeness. | None — keep both. |

## Proxy surface

| Symbol | Concern | Proposed action (≥ 2.0) |
|---|---|---|
| `NoProxy` struct | Exported type whose only public surface is the type name itself; `parseNoProxy` is unexported and `matches` is unexported. External callers can neither construct a useful value nor inspect one. | Unexport in 2.0. |

## Retry surface

| Symbol | Concern | Proposed action (≥ 2.0) |
|---|---|---|
| `RetryConfig` struct (`MaxRetries`, `Strategy`, `RetryIf`) | Declared in `retry.go` but not referenced anywhere else in the codebase. The same fields are present directly on `Config` and `Client`. Dead public type. | Drop in 2.0. |

## Notes that stay public

The audit deliberately *retains* these symbols even though earlier surveys
have flagged them:

- The full set of `WithXxx` options paired with `SetXxx` setters: the two
  forms cover construction-time vs runtime configuration cleanly. They are
  not duplicates; they target different lifecycle stages.
- Three streaming setters (`Stream` / `StreamErr` / `StreamDone`) instead of a
  single struct: callers consistently want to set a subset, and three setters
  read more naturally on a fluent chain than a struct literal.
- `AsHTTPClient` / `AsTransport`: both adapter shapes are needed because
  consuming SDKs ask for different types.

## Process

1. None of the candidates above are removed in 1.x.
2. When a 2.0 plan is opened, copy the candidate rows into the release notes
   alongside the migration path for each symbol.
3. Add `// Deprecated:` markers in source as soon as a 2.0 plan is committed,
   not before — premature deprecation creates noise without any release
   actually shipping the change.
