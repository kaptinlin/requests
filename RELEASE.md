# Release Handoff

This repository is a multi-module Go library. The root module must be released
before extension modules are tested or published outside `go.work`.

## Release Rule

Pick the next semantic version before starting the release. The release sequence
always starts at the root module; extension modules move only after the root
version is tagged, pushed, and resolvable.

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
   git tag -a vX.Y.Z -m vX.Y.Z
   git push origin vX.Y.Z
   GOPROXY=direct go list -m github.com/kaptinlin/requests@vX.Y.Z
   ```

3. After the root version is visible, pin each extension module to the released
   root version without local `replace` directives:

   ```bash
   for dir in browser fingerprint http3; do
     (cd "$dir" && go mod edit -require=github.com/kaptinlin/requests@vX.Y.Z && go mod tidy)
   done
   ```

   Do not pin the extension modules to `vX.Y.Z` before the root tag is
   resolvable. `go mod tidy` validates required versions even inside `go.work`,
   so pre-pinning creates a noisy broken maintenance state instead of a cleaner
   release boundary.

4. Verify each extension outside the workspace:

   ```bash
   task test:published
   ```

5. Tag and publish extension modules only after the `GOWORK=off` checks pass:

   ```bash
   git tag -a browser/vX.Y.Z -m browser/vX.Y.Z
   git tag -a fingerprint/vX.Y.Z -m fingerprint/vX.Y.Z
   git tag -a http3/vX.Y.Z -m http3/vX.Y.Z
   git push origin browser/vX.Y.Z fingerprint/vX.Y.Z http3/vX.Y.Z
   ```

6. Verify published modules:

   ```bash
   GOPROXY=direct go list -m github.com/kaptinlin/requests/browser@vX.Y.Z
   GOPROXY=direct go list -m github.com/kaptinlin/requests/fingerprint@vX.Y.Z
   GOPROXY=direct go list -m github.com/kaptinlin/requests/http3@vX.Y.Z
   ```

## Verification

- `task test:all`
- `task lint:all`
- `task test:published` after the root version is published and extension modules
  require that exact root version
