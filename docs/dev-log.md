# Development Log

## 2026-04-18 - Session 1: Phases 0-2 Complete

### What was done
- **Phase 0: Scaffolding** (commit 6873359)
  - Init Go module github.com/thejfml/npm-defense
  - Created directory structure (cmd, internal, testdata)
  - Added cobra + testify deps
  - Defined core types: Package, EnrichedPackage, Finding, Severity
  - Stub CLI with scan/explain/version commands

- **Phase 1: Lockfile Parser + Cache** (commit 42b17c4)
  - Lockfile parser for package-lock.json v2/v3 (npm 7+)
  - Dependency path computation (e.g., ["axios", "plain-crypto-js"])
  - Direct vs transitive dependency marking
  - XDG cache implementation with URL-encoded keys (@babel%2Fcore)
  - TTL support (immutable=0, mutable=24h)
  - Atomic writes (temp + rename)
  - All tests passing

- **Phase 2: Registry Client** (commit 425176b)
  - HTTP client with cache integration
  - GET /package (all versions, 24h TTL)
  - GET /package/version (specific version, no TTL - immutable)
  - Retry logic: 3x exponential backoff on 5xx/429
  - 30s timeout, offline mode support
  - FindPriorVersion: highest semver < current, skip pre-releases
  - GetPublishers: extract last N version publishers
  - All tests passing (cache hit/miss, retry, semver)

### What's in progress
Nothing - clean state at end of Phase 2.

### What's next
- **Phase 3: Enrichment Layer** (4-5h estimated)
  - Merge lockfile + registry data
  - Extract FirstPublished, LatestPublisher, PriorVersion, Scripts, PriorPublishers
  - Worker pool concurrency via registry semaphore
  - Context wiring (Cache, Registry, Logger)

### Non-obvious decisions
1. **devDependencies as direct deps**: In parsePackages, devDependencies are marked as direct (IsDirect=true) because from a supply-chain perspective, they're explicitly declared in package.json and just as dangerous as regular deps.

2. **TTL decision by caller**: Cache TTL is set by the calling function, not stored with data. GetVersionMetadata uses TTL=0 (immutable), GetPackageMetadata uses 24h (mutable).

3. **DependencyEntry unused**: Defined for v2 compat but not used in parsing. We only parse from "packages" map, not "dependencies" map.

4. **npm v6 lockfiles**: Explicitly NOT supported in v0.1 (only v2/v3, lockfileVersion 2-3). npm v7 released Nov 2020, 5+ years old. Can add v6 in v0.2 if users request.

### Context at session end
- 122k/200k tokens (61%)
- 3 commits, all tests passing
- GitHub issue #1 created with full plan
- Ready for Phase 3 next session
