# npm-defense

Offline-first, self-hosted static analyzer for npm supply-chain attacks.

> ️**Pre-alpha.** This project is in design phase. Nothing works yet.

## What it does

Scans your project's lockfile and flags dependencies whose characteristics
match known supply-chain attack patterns: new transitive dependencies
appearing out of nowhere, suspiciously fresh packages, install scripts with
obfuscated content, typosquat-adjacent names, and unusual maintainer
changes.

## What makes it different

- **Runs locally.** Your lockfile never leaves your machine.
- **No telemetry.** Zero network calls except to the npm registry itself.
- **Single binary.** No runtime, no daemon, no services.
- **Auditable.** Small codebase by design. You can read the whole thing.

## What it is not

It is not a replacement for Socket, Snyk, or Aikido if your threat model
requires real-time threat intelligence. Commercial tools have dedicated
research teams and will catch novel attacks faster. npm-defense is for
people who prioritize local execution and auditability over bleeding-edge
detection.

## Status

See `docs/design/` for the current design documents. Implementation has
not started yet.

## License

TBD before first commit.
