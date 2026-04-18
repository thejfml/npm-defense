package lockfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thejfml/npm-defense/internal/types"
)

// PackageLockV2 represents the structure of package-lock.json v2/v3.
type PackageLockV2 struct {
	Name            string                       `json:"name"`
	Version         string                       `json:"version"`
	LockfileVersion int                          `json:"lockfileVersion"`
	Requires        bool                         `json:"requires"`
	Packages        map[string]PackageLockEntry  `json:"packages"`
	Dependencies    map[string]DependencyEntry   `json:"dependencies,omitempty"` // v2 includes this
}

// PackageLockEntry represents a package entry in the "packages" map.
type PackageLockEntry struct {
	Version      string            `json:"version,omitempty"`
	Resolved     string            `json:"resolved,omitempty"`
	Integrity    string            `json:"integrity,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
	Link         bool              `json:"link,omitempty"`
}

// DependencyEntry represents an entry in the "dependencies" map (v2 compat).
type DependencyEntry struct {
	Version  string            `json:"version"`
	Resolved string            `json:"resolved,omitempty"`
	Dependencies map[string]DependencyEntry `json:"dependencies,omitempty"`
}

// Parse reads and parses a package-lock.json file.
func Parse(lockfilePath string) ([]types.Package, error) {
	data, err := os.ReadFile(lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("reading lockfile: %w", err)
	}

	var lockfile PackageLockV2
	if err := json.Unmarshal(data, &lockfile); err != nil {
		return nil, fmt.Errorf("parsing lockfile JSON: %w", err)
	}

	// Check lockfile version
	if lockfile.LockfileVersion < 2 || lockfile.LockfileVersion > 3 {
		return nil, fmt.Errorf("unsupported lockfileVersion %d (only v2 and v3 supported)", lockfile.LockfileVersion)
	}

	return parsePackages(lockfile)
}

// parsePackages extracts packages from the lockfile and builds dependency paths.
func parsePackages(lockfile PackageLockV2) ([]types.Package, error) {
	var packages []types.Package

	// The root package is at key ""
	rootEntry, hasRoot := lockfile.Packages[""]
	if !hasRoot {
		return nil, fmt.Errorf("lockfile missing root package entry")
	}

	// Build set of direct dependencies from root
	directDeps := make(map[string]bool)
	for name := range rootEntry.Dependencies {
		directDeps[name] = true
	}
	for name := range rootEntry.DevDependencies {
		directDeps[name] = true
	}

	// Parse all packages (skip root "")
	for pkgPath, entry := range lockfile.Packages {
		if pkgPath == "" {
			continue // Skip root
		}
		if entry.Link {
			continue // Skip workspace links
		}

		pkg := parsePackageEntry(pkgPath, entry, directDeps)
		packages = append(packages, pkg)
	}

	return packages, nil
}

// parsePackageEntry converts a PackageLockEntry to types.Package.
func parsePackageEntry(pkgPath string, entry PackageLockEntry, directDeps map[string]bool) types.Package {
	// Extract package name from path
	// Format: "node_modules/axios" or "node_modules/axios/node_modules/follow-redirects"
	name := extractPackageName(pkgPath)

	// Determine if this is a direct dependency
	isDirect := directDeps[name]

	// Build dependency path
	// For "node_modules/axios/node_modules/follow-redirects", path is ["axios", "follow-redirects"]
	depPath := buildDependencyPath(pkgPath)

	return types.Package{
		Name:         name,
		Version:      entry.Version,
		Resolved:     entry.Resolved,
		Integrity:    entry.Integrity,
		DeclaredDeps: entry.Dependencies,
		IsDirect:     isDirect,
		Path:         depPath,
	}
}

// extractPackageName extracts the package name from a lockfile path.
// "node_modules/axios" → "axios"
// "node_modules/@babel/core" → "@babel/core"
// "node_modules/axios/node_modules/follow-redirects" → "follow-redirects"
func extractPackageName(pkgPath string) string {
	// Split by "/node_modules/" to find nested packages
	parts := strings.Split(pkgPath, "/node_modules/")

	// Get the last part
	lastPart := parts[len(parts)-1]

	// If the last part still has "node_modules/" prefix, strip it
	lastPart = strings.TrimPrefix(lastPart, "node_modules/")

	return lastPart
}

// buildDependencyPath builds the dependency path from root.
// "node_modules/axios" → ["axios"]
// "node_modules/axios/node_modules/follow-redirects" → ["axios", "follow-redirects"]
func buildDependencyPath(pkgPath string) []string {
	// Split by "/node_modules/" to find nested packages
	parts := strings.Split(pkgPath, "/node_modules/")

	var path []string
	for _, part := range parts {
		// Strip leading "node_modules/" if present
		part = strings.TrimPrefix(part, "node_modules/")
		if part != "" {
			path = append(path, part)
		}
	}

	return path
}

// FindLockfile searches for package-lock.json or npm-shrinkwrap.json in the given directory.
func FindLockfile(dir string) (string, error) {
	// Try package-lock.json first
	lockfilePath := filepath.Join(dir, "package-lock.json")
	if _, err := os.Stat(lockfilePath); err == nil {
		return lockfilePath, nil
	}

	// Try npm-shrinkwrap.json
	shrinkwrapPath := filepath.Join(dir, "npm-shrinkwrap.json")
	if _, err := os.Stat(shrinkwrapPath); err == nil {
		return shrinkwrapPath, nil
	}

	return "", fmt.Errorf("no lockfile found in %s (tried package-lock.json, npm-shrinkwrap.json)", dir)
}
