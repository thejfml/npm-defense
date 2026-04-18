# Case study: axios supply chain compromise, March 31, 2026

This document captures the axios/plain-crypto-js attack as a worked example
of what npm-defense must catch. It serves two purposes:

1. A grounding scenario when designing rules: any change that would cause
   v0.1 to miss this attack is probably a mistake.
2. A regression test: we keep a snapshot of the malicious registry metadata
   in `testdata/` and assert that the tool catches it.

## Attack timeline

- **March 30, 2026, 05:57 UTC** — `plain-crypto-js@4.2.0` published. Clean.
  A typosquat-style name for crypto-js, with no payload. This was staging:
  giving the package a non-zero history so "brand-new package" alarms
  wouldn't fire as loudly.
- **March 30, 2026, 23:59 UTC** — `plain-crypto-js@4.2.1` published with
  malicious postinstall payload.
- **March 31, 2026, 00:21 UTC** — `axios@1.14.1` published via the
  compromised `jasonsaayman` account, injecting `plain-crypto-js@^4.2.1` as
  a runtime dependency.
- **March 31, 2026, 01:00 UTC** — `axios@0.30.4` published, same pattern, on
  the legacy branch.
- **March 31, 2026, 03:15–03:29 UTC** — Malicious versions removed from npm.

Live window: roughly 2–3 hours.

## How the attack worked mechanically

1. Attacker compromised the maintainer's npm publishing credentials (a
   long-lived NPM_TOKEN, despite OIDC Trusted Publishing being configured —
   when both are present, npm used the token).
2. Published two new axios versions with `plain-crypto-js@^4.2.1` added to
   the dependency list in `package.json`.
3. plain-crypto-js itself declared `"postinstall": "node setup.js"`.
4. setup.js was a double-obfuscated dropper (reversed Base64 with padding
   substitution, then XOR with key `OrDeR_7077`).
5. On install, the dropper detected OS (Windows/macOS/Linux), downloaded a
   platform-specific RAT from `sfrclak[.]com:8000`, and self-deleted —
   including overwriting its own `package.json` with a clean version for
   anti-forensics.

The key insight: the attacker did not modify axios source code. They only
added a dependency. The malicious behavior lived entirely inside
plain-crypto-js's postinstall hook.

## What signals were detectable from static metadata alone

Without executing any code, these facts were all visible in the npm
registry:

1. axios@1.14.1's dependency list included `plain-crypto-js`, which
   axios@1.14.0's did not.
2. plain-crypto-js@4.2.1 was published hours before axios@1.14.1.
3. plain-crypto-js@4.2.1 declared a postinstall script.
4. setup.js contained suspicious patterns (high-entropy strings,
   `String.fromCharCode`, `replaceAll` used for character substitution).
5. The axios maintainer account's registered email had recently changed to
   `ifstap@proton.me` (publicly visible if the API exposes it — needs
   verification for the rule).

Signals 1–4 would have been caught by rules R1, R2, R3, R4. Signal 5 is the
target of R6.

## What v0.1 would NOT have detected

- The actual C2 callback to sfrclak.com:8000 — static analysis can't
  observe runtime behavior. But this is fine: we'd have already flagged the
  package as high-risk from signals 1–4.
- If the attacker had existed as a clean maintainer for 6 months before
  weaponizing — R2 wouldn't fire on the package's age.
- If the payload had been inlined into axios itself rather than a separate
  dependency — R1 wouldn't fire; R3 would (if axios gained an install
  script), R4 might. Weaker overall but not blind.

## Expected scan output on this case

When `npm-defense scan` is run against a project that has just had its
lockfile updated from axios@1.14.0 to axios@1.14.1, the output should be
roughly:

```
[HIGH]   axios@1.14.1
         New transitive dependency 'plain-crypto-js@4.2.1' was added to this
         package's dependencies (not present in axios@1.14.0).
         - plain-crypto-js@4.2.1 was first published 2 hours before axios@1.14.1.
         - plain-crypto-js@4.2.1 declares a postinstall script: "node setup.js"
         - setup.js contains high-entropy strings and obfuscation patterns
           consistent with a dropper.
         Rules fired: R1 (dep-diff), R2 (package-age), R3 (install-script),
                      R4 (install-script-content)
         Recommended action: do not install. Pin axios to 1.14.0.
```

If the scan produces less than this, we've regressed. This is the tool's
single most important regression test.

## Sources

For posterity, here's where this analysis draws from (not committed to repo;
recorded here for reference):

- Elastic Security Labs technical breakdown (April 1, 2026)
- Socket's detection announcement (March 31, 2026)
- Snyk blog post
- Google Threat Intelligence Group attribution to UNC1069
- Microsoft Threat Intelligence write-up
- Huntress incident post
- SANS Institute briefing
