# Architecture

Decisions-and-interfaces reference for v0.1. Rationale lives in design
discussion, not here. If you're changing one of these decisions, update this
doc in the same commit.

## Data flow

```
lockfile → parser → dep set → enricher → rules engine → findings → renderer → stdout
                                 ↓↑
                              cache (filesystem)
                                 ↑
                          npm registry (network)
```

## Package layout

```
cmd/npm-defense/          main, flag parsing, wiring
internal/lockfile/        parsers for package-lock.json, yarn.lock (later)
internal/registry/        npm registry client
internal/cache/           filesystem cache
internal/enrich/          merges lockfile entries + registry data
internal/rules/           one file per rule (r1_depdiff.go, r2_age.go, ...)
internal/rules/engine.go  rule runner
internal/report/          finding types + renderers (human, json)
internal/popular/         embedded top-10k popular package list for R5
testdata/                 fixtures incl. axios-march-2026 snapshot
```

No `pkg/`. Nothing is a public library in v0.1.

## Core types

```go
// lockfile
type Package struct {
    Name         string
    Version      string
    Resolved     string   // tarball URL from lockfile
    Integrity    string   // sha512 from lockfile
    DeclaredDeps map[string]string // name → version constraint
    IsDirect     bool     // top-level dep vs transitive
    Path         []string // dep path from root, e.g. ["axios", "plain-crypto-js"]
}

// enriched with registry data
type EnrichedPackage struct {
    Package
    FirstPublished  time.Time
    LatestPublisher string       // _npmUser of this version
    PriorVersion    *VersionInfo // highest semver < this version, from registry
    Scripts         map[string]string // preinstall/install/postinstall
    PriorPublishers []string     // publishers of prior N versions, for R6
}

type VersionInfo struct {
    Version      string
    DeclaredDeps map[string]string
    Publisher    string
    Published    time.Time
}

// findings
type Finding struct {
    Package   string   // "plain-crypto-js@4.2.1"
    Path      []string // ["axios@1.14.1", "plain-crypto-js@4.2.1"]
    Severity  Severity // low | medium | high
    RulesFired []string // ["R1", "R2", "R3"]
    Details   []string  // human-readable lines, one per rule hit
}

type Severity int
const (
    SeverityLow Severity = iota
    SeverityMedium
    SeverityHigh
)
```

## Rule interface

```go
type Rule interface {
    ID() string            // "R1"
    Name() string          // "dep-diff"
    Check(ctx Context, pkg EnrichedPackage) []RuleHit
}

type RuleHit struct {
    Severity Severity
    Detail   string // one-line human-readable explanation
}

type Context struct {
    Cache    cache.Store
    Registry registry.Client
    Popular  popular.List  // for R5
    Logger   *slog.Logger
}
```

Rules do not read state from disk directly. All I/O goes through `Context`.
This matters for testability.

## Key design decisions

### D1. R1 comparison baseline
Compare current version's declared deps against the **highest published
semver strictly less than the current version** (fetched from registry,
cached). Pre-releases excluded unless current is also a pre-release.

### D2. Registry concurrency
Bounded worker pool. Default 10 concurrent requests. `--concurrency N`
overrides. Use `golang.org/x/sync/errgroup` with a semaphore channel.

### D3. Cache format
JSON files on disk under `$XDG_CACHE_HOME/npm-defense/` (fallback
`~/.cache/npm-defense/`). One file per `<name>@<version>.json`. URL-encode
names with slashes (scoped packages). Immutable fields never expire.
Mutable fields (latest-version list, maintainer list) have 24h TTL stored
alongside the data.

### D4. JSON output schema
Top-level object:
```json
{
  "schema": "npm-defense/v1",
  "scanned_at": "2026-04-18T14:30:00Z",
  "lockfile": "package-lock.json",
  "summary": {"high": 1, "medium": 2, "low": 0},
  "findings": [
    {
      "package": "plain-crypto-js@4.2.1",
      "path": ["axios@1.14.1", "plain-crypto-js@4.2.1"],
      "severity": "high",
      "rules_fired": ["R1", "R2", "R3", "R4"],
      "details": ["...", "..."]
    }
  ]
}
```
Additive changes (new fields) are allowed in v1. Removing/renaming fields
requires bumping to `v2`.

### D5. Finding subject
Findings are pinned to the **suspicious package itself**, not the package
that pulled it in. The `path` field shows the dependency chain. axios
compromise produces one finding on `plain-crypto-js@4.2.1` with path
`["axios@1.14.1", "plain-crypto-js@4.2.1"]`.

### D6. Exit codes
- `0` — scan completed, no findings at or above `--fail-on` threshold
- `1` — scan completed, findings at or above threshold
- `2` — scan failed (parse error, network error without `--offline`, etc.)
- `3` — misuse (bad flags, missing lockfile)

Default `--fail-on` is `high`.

### D7. Offline behavior
Default: fail hard (exit 2) if registry is unreachable. With `--offline`,
run against cache only; every finding annotated with cache age; summary
includes `"cache_only": true` in JSON output.

### D8. Tarball fetching for R4
R4 requires the contents of script files referenced from `package.json`
(e.g. `"postinstall": "node setup.js"` — the script string is in the
manifest, but `setup.js` itself lives in the tarball).

- Tarballs are fetched **only** for packages that have already triggered at
  least one other rule (R1, R2, R3, R5, or R6) in the current scan.
- Allowed hosts extend to registry tarball URLs
  (`registry.npmjs.org/<pkg>/-/<tarball>`) and their redirect targets.
- Tarballs are extracted in-memory; never written to disk outside the
  cache directory.
- Only files referenced from `package.json`'s `scripts` field are read.
  Other tarball contents are ignored.
- Tarball contents are **not** cached in v0.1 (cache stores registry JSON
  only). Re-fetches happen on every scan. Revisit if this becomes a
  performance problem.
- The rules engine runs in two passes: pass 1 runs R1, R2, R3, R5, R6 on
  all packages; pass 2 runs R4 on packages with at least one pass-1 hit.

## CLI surface

```
npm-defense scan [path]
    [--json]
    [--fail-on=low|medium|high]     default: high
    [--concurrency=N]               default: 10
    [--offline]                     default: false
    [--cache-dir=PATH]              default: $XDG_CACHE_HOME/npm-defense
    [--no-cache]                    bypass cache for this run

npm-defense explain <rule-id>
    Prints rule description, what it catches, what it misses.

npm-defense version
    Prints version and commit hash.
```

## Network policy

Allowed network destinations:
- `registry.npmjs.org` — package metadata (all scans).
- Registry tarball URLs and their redirect targets — only when triggered
  by D8 (pass-2 R4 on packages with pass-1 hits).

Any code path that calls out anywhere else is a bug. A test asserts this
by wrapping the HTTP client and failing on unexpected hosts.

## Concurrency & state

- No global state. `Context` is passed explicitly.
- Cache writes are atomic (write to temp file, rename). No locking needed;
  last-write-wins is acceptable because cached data is deterministic per
  `pkg@version`.
- The rules engine runs in two passes (see D8). Pass 1: R1, R2, R3, R5, R6
  run sequentially per package, packages concurrently within the worker
  pool. Pass 2: R4 runs on packages with at least one pass-1 hit, also
  concurrent within the worker pool.

## Testing strategy

- Table-driven tests for every rule.
- `testdata/axios-march-2026/` contains a frozen snapshot of registry
  responses for axios 1.14.0, 1.14.1, and plain-crypto-js 4.2.0, 4.2.1. A
  single integration test runs the full pipeline against a synthetic
  lockfile and asserts the expected finding.
- Network-free by default. Registry client has a `Transport` field
  injectable in tests; production uses `http.DefaultTransport`, tests use
  a `fstest`-backed fake that serves from `testdata/`.
- Three real-world projects (names TBD) in a separate `testdata/real/`
  directory, used by a `go test -tags=realworld` build tag for
  false-positive regression testing. Not run by default CI.

## Deferred (explicitly not in v0.1)

- yarn.lock and pnpm-lock.yaml parsers
- git-log-based prior lockfile diffing (D1 fallback)
- Rule configuration / user-defined rules
- SARIF / SBOM output formats
- Auto-remediation suggestions beyond "pin to prior version"
- Daemon / watch mode
- MCP server interface
