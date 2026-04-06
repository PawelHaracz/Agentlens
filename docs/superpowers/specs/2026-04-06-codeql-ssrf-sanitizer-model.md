# Task: Teach CodeQL that `service.ValidateURL` is an SSRF sanitizer

## Background

`internal/service/card_fetcher.go:194` (`f.client.Do(req)`) is **still** flagged by GitHub Advanced Security under `go/request-forgery` ("Uncontrolled data used in network request"), even though the code has four layers of SSRF defense:

1. `ValidateURL` — scheme allowlist (`http`/`https` only) and private/loopback/link-local hostname & IP rejection.
2. URL reconstruction — the outbound `safeURL` is rebuilt from validated `Scheme`/`Host`/`Path`/`RawQuery` components, dropping `Fragment` and `Userinfo`.
3. `CheckRedirect` — re-runs `ValidateURL` on every redirect target, blocking public→internal bounces.
4. `net.Dialer.Control` — rejects connections to private IPs at socket level, closing the DNS-rebinding TOCTOU window.

These protections are covered by 49 unit tests in `internal/service/card_fetcher_test.go` (red-green verified) and the alert was manually dismissed once. CodeQL re-raises it because its built-in `go/request-forgery` query does not recognize `service.ValidateURL` as a sanitizer in the taint-tracking dataflow — the user-controlled URL string flows uninterrupted from the request body all the way to `client.Do`, regardless of the runtime checks we perform on it.

We have three options. We're picking option **C** and shipping it:

| Option | Why not |
|---|---|
| A. Domain allowlist | Breaks the core feature: users legitimately import agent cards from arbitrary public URLs. |
| B. Permanent dismissal in the Security UI | Drifts: every new push that touches the file re-raises it; dismissals don't survive code motion; new contributors don't know it's intentional. |
| **C. Custom CodeQL model** | **Picked.** Teaches CodeQL the truth: `ValidateURL` is a sanitizer. Travels with the code, applies to every PR automatically, and is the GitHub-recommended path for this exact situation. Falls back to inline `// lgtm[...]` annotation as a safety net if the model fails to load. |

## Authoritative sources of truth

1. `internal/service/card_fetcher.go` — the function under scrutiny. Read it end-to-end before writing the model.
2. `internal/service/card_fetcher_test.go` — the test suite that proves the runtime guard works. Do not weaken it.
3. `.github/workflows/code-scanning.yml` — current CodeQL workflow (lines 16-42). You will extend it with a custom config.
4. CodeQL Go standard library: `Sanitizer` and `BarrierGuard` patterns in the `RequestForgery::RequestForgery` taint-tracking module.
5. GitHub docs on [model packs](https://docs.github.com/en/code-security/codeql-cli/using-the-advanced-functionality-of-the-codeql-cli/creating-and-working-with-codeql-packs) and [config files](https://docs.github.com/en/code-security/code-scanning/creating-an-advanced-setup-for-code-scanning/customizing-your-advanced-setup-for-code-scanning#using-a-custom-configuration-file).

## Required changes

### 1. Custom CodeQL config + sanitizer query pack

Create the following directory tree at the repo root:

```
.github/
  codeql/
    codeql-config.yml
    qlpack.yml
    sanitizers/
      AgentLensSanitizers.qll
      RequestForgerySanitizer.ql
```

#### `.github/codeql/codeql-config.yml`

```yaml
name: "AgentLens CodeQL Config"

# Use the standard security-and-quality suite plus our custom sanitizer pack.
queries:
  - uses: security-and-quality
  - uses: ./sanitizers

# We don't want to scan generated files or vendored deps.
paths-ignore:
  - "web/dist/**"
  - "web/node_modules/**"
  - "**/*_gen.go"
  - "**/zz_generated_*.go"
```

#### `.github/codeql/qlpack.yml`

```yaml
name: agentlens/custom-sanitizers
version: 0.0.1
library: false
extractor: go
dependencies:
  codeql/go-all: "*"
```

#### `.github/codeql/sanitizers/AgentLensSanitizers.qll`

A barrier-guard module that marks any value passing through `service.ValidateURL` (or its method receiver `(*service.CardFetcher).ValidateURL`) as sanitized for the request-forgery flow.

```ql
import go
import semmle.go.security.RequestForgeryCustomizations

/**
 * A call to `internal/service.ValidateURL` (or the method form on
 * `*service.CardFetcher`). When this call returns nil error, the URL
 * argument has been verified to be:
 *   - non-empty
 *   - http or https scheme only
 *   - not pointing to a private/loopback/link-local hostname or IP
 *
 * The runtime guarantee is reinforced by:
 *   - URL reconstruction (Fragment/Userinfo stripped before Do)
 *   - CheckRedirect re-validating every redirect hop
 *   - net.Dialer.Control rejecting private IPs at connect time
 *
 * See internal/service/card_fetcher.go and its accompanying test suite.
 */
class AgentLensValidateURLBarrier extends RequestForgery::Sanitizer {
  AgentLensValidateURLBarrier() {
    exists(DataFlow::CallNode call |
      call.getTarget().hasQualifiedName("github.com/PawelHaracz/agentlens/internal/service",
                                        "ValidateURL")
      or
      call.getTarget().(Method).hasQualifiedName(
        "github.com/PawelHaracz/agentlens/internal/service",
        "CardFetcher", "ValidateURL"
      )
    |
      this = call.getArgument(0)
    )
  }
}
```

> Note for the agent: the exact API for `RequestForgery::Sanitizer` may differ slightly between CodeQL library versions. If `RequestForgery::Sanitizer` is not the correct extension point in the version pinned by GitHub, use `RequestForgery::SanitizerGuard` or model it as a `TaintTracking::DefaultTaintSanitizer` member instead. Verify by reading the file at `codeql/go/ql/lib/semmle/go/security/RequestForgeryCustomizations.qll` from the CodeQL distribution and pick the extension point that the upstream `go/request-forgery` query actually consults. Document which variant you used and why in the PR description.

#### `.github/codeql/sanitizers/RequestForgerySanitizer.ql`

A trivial query file that just imports the customizations so CodeQL loads them. CodeQL only honors `.qll` files when they are reachable from a `.ql` query in the same pack.

```ql
/**
 * @name AgentLens custom sanitizers loader
 * @description Imports project-specific sanitizer extensions so they are
 *              applied to standard security queries.
 * @kind problem
 * @id go/agentlens/custom-sanitizer-loader
 * @tags security
 */
import go
import AgentLensSanitizers

from int never
where never = -1 and never = 0
select never, "unreachable"
```

This query produces zero results by construction; its only purpose is to anchor the customization module so the CodeQL engine compiles and loads it.

### 2. Wire the config into the workflow

Edit `.github/workflows/code-scanning.yml`, the `codeql` job (lines 16-42). Add `config-file` to the `Initialize CodeQL` step:

```yaml
      - name: Initialize CodeQL
        uses: github/codeql-action/init@v3
        with:
          languages: ${{ matrix.language }}
          config-file: ./.github/codeql/codeql-config.yml
```

Do not change anything else in `code-scanning.yml`. The `javascript-typescript` matrix entry will simply ignore the Go-only sanitizer pack.

### 3. Inline annotation as safety net

Even with the model in place, add a single CodeQL suppression comment immediately above the `f.client.Do(req)` call in `internal/service/card_fetcher.go`. This is belt-and-braces: if the custom pack ever fails to load (e.g. CodeQL library version bump changes the API), the alert still won't reappear, and reviewers see the explicit reasoning right at the call site.

```go
	// codeql[go/request-forgery]: URL is sanitized by service.ValidateURL above
	// (scheme allowlist + private-IP rejection), reconstructed from validated
	// components, redirects re-validated by CheckRedirect, and connections to
	// private IPs are rejected at socket level by net.Dialer.Control.
	// See internal/service/card_fetcher_test.go for the regression suite.
	resp, err := f.client.Do(req)
```

Use the exact `// codeql[query-id]` form CodeQL recognizes. Do **not** use `// nolint`, `// lgtm`, or any other linter pragma — only the CodeQL one.

### 4. Documentation

Add a short subsection to `docs/architecture.md` under the existing security/SSRF discussion (or create one if missing) explaining:
- The four layers of SSRF defense in `card_fetcher.go`.
- Why we ship a custom CodeQL sanitizer instead of dismissing alerts.
- A pointer to `.github/codeql/sanitizers/AgentLensSanitizers.qll`.

Keep it to ~10 lines. Use Mermaid only if a diagram clarifies the data flow; otherwise plain text.

## Verification

1. **Local CodeQL run** (recommended; the agent should attempt this if the runner has the CLI):

   ```bash
   # Install CodeQL CLI per https://github.com/github/codeql-cli-binaries
   codeql database create --language=go --source-root . agentlens-db
   codeql database analyze agentlens-db \
     --format=sarif-latest \
     --output=results.sarif \
     .github/codeql/codeql-config.yml
   ```

   Inspect `results.sarif` and confirm there is **no** `go/request-forgery` finding pointing at `internal/service/card_fetcher.go:194`. If the CLI isn't available in the runner, skip this step and rely on PR-time CI.

2. **CI run** — push to a branch, open the PR, and check the "Code Scanning" workflow:
   - The `codeql` job (Go matrix entry) must complete successfully.
   - The Security tab must not report `go/request-forgery` for `card_fetcher.go`.
   - No new alerts may be introduced for any other file.

3. **Regression** — `make test` and the existing `internal/service/card_fetcher_test.go` suite must still pass unchanged. This PR must not weaken any runtime check.

4. **Lint** — `make lint` must pass; the new comment block must not violate any golangci-lint rule.

5. **Build** — `make build` must succeed.

## Pull request

- Title: `security(scanning): teach CodeQL about ValidateURL SSRF sanitizer`
- Body must include:
  - A summary of why the existing runtime guards are sufficient (link to `card_fetcher.go` and the test file).
  - The CodeQL extension point you chose (`Sanitizer`, `SanitizerGuard`, or `TaintTracking::DefaultTaintSanitizer`) and why.
  - The link to the dismissed alert this PR closes for good.
  - Confirmation that no production code was changed beyond the single suppression comment block.
  - Output of `make test` and `make lint`.

## Things NOT to do

- **Do not weaken or remove any runtime SSRF check.** The model is additive — the runtime guards are still the actual security boundary.
- **Do not add a domain allowlist.** Importing arbitrary public agent cards is the entire point of the feature.
- **Do not dismiss the alert in the Security UI.** Dismissals don't survive refactors, don't appear in code review, and don't help new contributors. The whole point of this PR is to make the suppression an artifact in the repo.
- **Do not blanket-disable `go/request-forgery`** anywhere — neither in `codeql-config.yml` (`query-filters`) nor anywhere else. We want it active for every other file.
- **Do not modify `internal/service/card_fetcher.go` beyond adding the codeql annotation comment.** No refactoring, no signature changes, no new helpers.
- **Do not touch the existing test file** other than (optionally) adding a one-line comment pointing to this spec near the top.
- **Do not introduce a new top-level directory.** Everything lives under `.github/codeql/` and (one comment) `internal/service/card_fetcher.go`.
- **Do not bump CodeQL action versions** (`github/codeql-action/init@v3`, `analyze@v3`) — that is a separate concern.
