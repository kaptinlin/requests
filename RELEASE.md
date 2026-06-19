# Release Handoff

This repository is a multi-module Go library. The root module must be released
before extension modules are tested or published outside `go.work`.

## Current Breaking Release

Target version: `v0.7.0`

Decision: freeze the current work as the root breaking release. The release
sequence starts at the root module; extension modules move only after the root
version is tagged, pushed, and resolvable.

Why this is not a patch release:

- `New` now returns `(*Client, error)`.
- `Config`, `Create`, and `URL` were removed.
- Request body helpers were narrowed to `JSON`, `XML`, `YAML`, `Text`, `Bytes`, and `Reader`.
- `Response` and `StreamResponse` now expose behavior methods and `Raw()` instead of mutable storage fields.
- The streaming surface is caller-owned through `SendStream`.

Do not add removed-surface aliases, pre-pin extension modules to an unpublished
root version, or weaken `task test:published` to make the unpublished workspace
look complete. A failed published-module check before the root tag exists is the
correct signal.

## Required Order

1. Run the full workspace gate:

   ```bash
   task test:all
   task lint:all
   ```

2. Tag and publish the root module first:

   ```bash
   git tag -a v0.7.0 -m v0.7.0
   git push origin v0.7.0
   GOPROXY=direct go list -m github.com/kaptinlin/requests@v0.7.0
   ```

3. After the root version is visible, pin each extension module to the released
   root version without local `replace` directives:

   ```bash
   for dir in browser fingerprint http3; do
     (cd "$dir" && go mod edit -require=github.com/kaptinlin/requests@v0.7.0 && go mod tidy)
   done
   ```

   Do not pin the extension modules to `v0.7.0` before the root tag is
   resolvable. `go mod tidy` validates required versions even inside `go.work`,
   so pre-pinning creates a noisy broken maintenance state instead of a cleaner
   release boundary.

4. Verify each extension outside the workspace:

   ```bash
   task test:published
   ```

5. Tag and publish extension modules only after the `GOWORK=off` checks pass:

   ```bash
   git tag -a browser/v0.7.0 -m browser/v0.7.0
   git tag -a fingerprint/v0.7.0 -m fingerprint/v0.7.0
   git tag -a http3/v0.7.0 -m http3/v0.7.0
   git push origin browser/v0.7.0 fingerprint/v0.7.0 http3/v0.7.0
   ```

6. Verify published modules:

   ```bash
   GOPROXY=direct go list -m github.com/kaptinlin/requests/browser@v0.7.0
   GOPROXY=direct go list -m github.com/kaptinlin/requests/fingerprint@v0.7.0
   GOPROXY=direct go list -m github.com/kaptinlin/requests/http3@v0.7.0
   ```

## Release Notes

### Breaking Changes

- Replaced best-effort construction with validated `New(opts ...Option) (*Client, error)`.
- Removed parallel construction paths and public mutable client configuration.
- Kept streaming caller-owned through `SendStream`.
- Split buffered and streaming response ownership; use `Raw()` for the underlying `*http.Response`.
- Replaced body helper names with short explicit verbs.
- Replaced scalar retry setters with `RetryPolicy`.

### Verification

- `task test:all`
- `task lint:all`
- `task test:published` after root `v0.7.0` is published and extension modules
  require that exact root version
