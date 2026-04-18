package registry

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// PackageMetadata represents the full package document from npm registry.
// GET /package returns all versions and metadata.
type PackageMetadata struct {
	Name     string                        `json:"name"`
	Versions map[string]*VersionMetadata   `json:"versions"`
	Time     TimeMetadata                  `json:"time"`
}

// VersionMetadata represents metadata for a specific package version.
type VersionMetadata struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Scripts      map[string]string `json:"scripts,omitempty"`
	NPMUser      *NPMUser          `json:"_npmUser,omitempty"`
	Dist         *DistMetadata     `json:"dist,omitempty"`
}

// NPMUser represents the user who published a version.
type NPMUser struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// DistMetadata contains distribution/tarball information.
type DistMetadata struct {
	Tarball   string `json:"tarball"`
	Integrity string `json:"integrity,omitempty"`
}

// TimeMetadata contains timestamps for package versions.
type TimeMetadata struct {
	Created  time.Time          `json:"created"`
	Modified time.Time          `json:"modified"`
	Versions map[string]time.Time `json:"-"` // Versions published times (unmarshal manually)
}

// UnmarshalJSON custom unmarshaler for TimeMetadata to handle version timestamps.
func (t *TimeMetadata) UnmarshalJSON(data []byte) error {
	// Parse as map to get all fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Extract created and modified
	if created, ok := raw["created"].(string); ok {
		if parsedTime, err := time.Parse(time.RFC3339, created); err == nil {
			t.Created = parsedTime
		}
	}
	if modified, ok := raw["modified"].(string); ok {
		if parsedTime, err := time.Parse(time.RFC3339, modified); err == nil {
			t.Modified = parsedTime
		}
	}

	// Extract version timestamps (keys are version strings)
	t.Versions = make(map[string]time.Time)
	for key, val := range raw {
		if key != "created" && key != "modified" {
			if timeStr, ok := val.(string); ok {
				if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
					t.Versions[key] = parsedTime
				}
			}
		}
	}

	return nil
}

// FindPriorVersion finds the highest semver version strictly less than the current version.
// Excludes pre-release versions unless current is also a pre-release.
func FindPriorVersion(current string, allVersions []string) (string, error) {
	if current == "" {
		return "", fmt.Errorf("current version is empty")
	}

	// Normalize version to semver format (v prefix)
	if !strings.HasPrefix(current, "v") {
		current = "v" + current
	}

	// Filter and normalize all versions
	var candidates []string
	currentIsPrerelease := semver.Prerelease(current) != ""

	for _, v := range allVersions {
		if !strings.HasPrefix(v, "v") {
			v = "v" + v
		}

		// Skip invalid semver
		if !semver.IsValid(v) {
			continue
		}

		// Skip if >= current
		if semver.Compare(v, current) >= 0 {
			continue
		}

		// Skip pre-releases unless current is also pre-release
		if !currentIsPrerelease && semver.Prerelease(v) != "" {
			continue
		}

		candidates = append(candidates, v)
	}

	if len(candidates) == 0 {
		return "", nil // No prior version found
	}

	// Sort and return highest
	sort.Slice(candidates, func(i, j int) bool {
		return semver.Compare(candidates[i], candidates[j]) < 0
	})

	highest := candidates[len(candidates)-1]
	// Remove v prefix before returning
	return strings.TrimPrefix(highest, "v"), nil
}

// GetPublishers returns the publishers of the last N versions.
func GetPublishers(pkg *PackageMetadata, lastN int) []string {
	if pkg == nil || len(pkg.Versions) == 0 {
		return nil
	}

	// Get all versions sorted
	var versions []string
	for v := range pkg.Versions {
		versions = append(versions, v)
	}

	sort.Slice(versions, func(i, j int) bool {
		vi, vj := versions[i], versions[j]
		if !strings.HasPrefix(vi, "v") {
			vi = "v" + vi
		}
		if !strings.HasPrefix(vj, "v") {
			vj = "v" + vj
		}
		return semver.Compare(vi, vj) < 0
	})

	// Get last N versions
	start := len(versions) - lastN
	if start < 0 {
		start = 0
	}

	var publishers []string
	seen := make(map[string]bool)

	for i := start; i < len(versions); i++ {
		ver := versions[i]
		if meta := pkg.Versions[ver]; meta != nil && meta.NPMUser != nil {
			publisher := meta.NPMUser.Name
			if publisher != "" && !seen[publisher] {
				publishers = append(publishers, publisher)
				seen[publisher] = true
			}
		}
	}

	return publishers
}
