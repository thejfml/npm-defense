# npm-defense

An offline-first, self-hosted static analyzer for npm dependencies. Scans a
project's lockfile and flags packages whose characteristics match known
supply-chain attack patterns. Designed to be run locally — no telemetry, no
uploading of dependency graphs to a third party.

## Why this exists

npm supply-chain attacks are frequent and increasingly sophisticated (Shai-Hulud
worm, Nx/s1ngularity, tj-actions, axios/plain-crypto-js in March 2026).
Commercial tools (Socket, Snyk, Aikido) exist but are SaaS-first and require
sending dependency data off-machine. There's a real niche for a self-hosted,
no-telemetry tool aimed at privacy-conscious developers and small teams.

## Scope discipline

This is a deliberately small v0.1. The goal is to ship something useful in
2–4 weekends, then decide what to build next based on real use. See
`docs/design/v0.1-scope.md` for the exact feature list.

### In scope for v0.1

- CLI that takes a `package-lock.json` (or `npm-shrinkwrap.json`, or `yarn.lock`
  — to be decided) and produces a human-readable report.
- A fixed set of six detection rules (see `docs/design/rules.md`).
- Local cache of package metadata fetched from the npm registry.
- Exit code reflects findings severity (for CI use).

### Explicitly out of scope for v0.1

- npm proxy or install-time interception.
- Sandboxed execution of install scripts.
- Web UI or dashboard.
- Real-time feed ingestion (OSV, GHSA).
- pnpm support (npm + yarn classic only for v0.1).
- Auto-remediation / auto-pinning.
- Any form of telemetry, even opt-in.

These are deferred, not rejected. Design v0.1 so they can be added without
rewriting the core.

## Tech stack

- **Language**: Go (1.22+). Chosen for single-binary distribution, strong
  stdlib for HTTP/filesystem/concurrency, and maintainer familiarity.
- **CLI framework**: `spf13/cobra` unless a simpler alternative emerges.
- **HTTP**: stdlib `net/http`. No third-party client.
- **Cache**: local filesystem cache under `~/.cache/npm-defense/` (respect
  `XDG_CACHE_HOME`). JSON files keyed by package@version. No database.
- **Tests**: stdlib `testing` + `testify/require` for assertions. Table-driven
  tests are the default pattern.

## Conventions

- **Modules**: `github.com/thejfml/npm-defense`.
- **Package layout**: standard Go layout. `cmd/npm-defense/` for the binary,
  `internal/` for everything else. Avoid `pkg/` — nothing in this repo is
  intended as a public library in v0.1.
- **Errors**: wrap with `fmt.Errorf("doing X: %w", err)`. No `errors.New` for
  anything a user might see; give context.
- **Logging**: `log/slog` with text handler by default, JSON via `--log-json`.
  Logs go to stderr, report output to stdout.
- **No global state**. Pass dependencies explicitly. Makes testing easier and
  will matter when we add the daemon later.
- **File naming**: `snake_case.go` for files, `CamelCase` for exported,
  `camelCase` for unexported. Standard Go.
- **Comments**: doc comments on every exported identifier. Inline comments
  only where the code isn't self-explanatory.

## Working agreements for Claude Code

- **Ask before adding dependencies.** Default to stdlib. If a third-party
  library seems necessary, propose it with a one-line rationale before adding
  it to `go.mod`.
- **Ask before changing scope.** If a task implies going beyond v0.1 scope
  (see above), surface that and wait for confirmation.
- **Small, reviewable commits.** One logical change per commit. Commit
  messages: `area: short imperative summary` (e.g., `rules: add package-age
  heuristic`).
- **Tests alongside code.** Every new rule gets a table-driven test with at
  least one positive case (rule fires) and one negative case (rule doesn't
  fire on benign input). Real-world samples go in `testdata/`.
- **When uncertain, propose options.** If there's a meaningful design
  decision (cache format, rule severity levels, report schema), lay out 2–3
  options with tradeoffs rather than picking silently.
- **Respect the "no telemetry" principle absolutely.** No version-check
  pings, no "phone home" anything, no automatic updates. If something looks
  like it might send data off-machine, stop and ask.

## Entry points for deeper context

- `docs/design/v0.1-scope.md` — what v0.1 does and doesn't do, in detail.
- `docs/design/rules.md` — the six detection rules, what each catches, what
  each misses.
- `docs/design/case-studies/axios-march-2026.md` — the axios compromise as a
  worked example of what the tool should catch.
- `docs/design/architecture.md` — decisions and interfaces: data flow,
  package layout, core types, rule interface, CLI surface.

## Non-goals, stated plainly

This tool will not be faster or more accurate than Socket or Snyk at
detecting novel attacks. Its value is: local-only execution, no telemetry,
zero data leaves the machine, free, and good enough to catch the majority
of attacks that follow known playbooks. If you need cutting-edge threat
intel, use a commercial tool. If you want a tool you can audit line-by-line
and run air-gapped, that's what this is for.
