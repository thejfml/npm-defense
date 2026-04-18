package types

import "time"

// Package represents a package from the lockfile.
type Package struct {
	Name         string            // Package name (e.g., "axios" or "@babel/core")
	Version      string            // Exact version from lockfile (e.g., "1.14.1")
	Resolved     string            // Tarball URL from lockfile
	Integrity    string            // sha512 integrity hash from lockfile
	DeclaredDeps map[string]string // Dependencies declared in package.json (name → version constraint)
	IsDirect     bool              // True if this is a direct dependency, false if transitive
	Path         []string          // Dependency path from root (e.g., ["axios", "plain-crypto-js"])
}

// VersionInfo contains metadata about a specific package version.
type VersionInfo struct {
	Version      string            // Version string (e.g., "1.14.0")
	DeclaredDeps map[string]string // Dependencies declared for this version
	Publisher    string            // _npmUser.name who published this version
	Published    time.Time         // When this version was published
}

// EnrichedPackage combines lockfile data with registry metadata.
type EnrichedPackage struct {
	Package
	FirstPublished  time.Time     // When the package was first published (time.created)
	LatestPublisher string        // Publisher (_npmUser.name) of this specific version
	PriorVersion    *VersionInfo  // Metadata for the highest semver version < current version
	Scripts         map[string]string // Install scripts (preinstall/install/postinstall only)
	PriorPublishers []string      // Publishers of the last N versions (for R6)
}

// Severity represents finding severity levels.
type Severity int

const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
)

// String returns the string representation of the severity.
func (s Severity) String() string {
	switch s {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	default:
		return "unknown"
	}
}

// Finding represents a security finding for a package.
type Finding struct {
	Package    string   // Package identifier (e.g., "plain-crypto-js@4.2.1")
	Path       []string // Dependency path (e.g., ["axios@1.14.1", "plain-crypto-js@4.2.1"])
	Severity   Severity // Aggregated severity (max of all rules that fired)
	RulesFired []string // Rule IDs that fired (e.g., ["R1", "R2", "R3"])
	Details    []string // Human-readable details, one per rule hit
}

// RuleHit represents a single rule firing on a package.
type RuleHit struct {
	Severity Severity // Base severity for this rule hit
	Detail   string   // One-line human-readable explanation
}
