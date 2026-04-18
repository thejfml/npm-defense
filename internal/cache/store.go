package cache

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Store provides a filesystem-based cache for registry metadata.
type Store struct {
	dir string // Cache directory path
}

// CachedData wraps cached content with metadata.
type CachedData struct {
	CachedAt time.Time       `json:"cached_at"`
	Data     json.RawMessage `json:"data"`
}

// New creates a new cache store at the specified directory.
// If dir is empty, uses XDG_CACHE_HOME/npm-defense or ~/.cache/npm-defense.
func New(dir string) (*Store, error) {
	if dir == "" {
		var err error
		dir, err = defaultCacheDir()
		if err != nil {
			return nil, fmt.Errorf("determining cache directory: %w", err)
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}

	return &Store{dir: dir}, nil
}

// Get retrieves cached data for a package version.
// Returns nil if not found or expired (based on ttl).
// ttl of 0 means never expire (for immutable data).
func (s *Store) Get(pkg, version string, ttl time.Duration) ([]byte, error) {
	path := s.keyPath(pkg, version)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Not found, not an error
		}
		return nil, fmt.Errorf("reading cache file: %w", err)
	}

	var cached CachedData
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("parsing cached data: %w", err)
	}

	// Check TTL
	if ttl > 0 && time.Since(cached.CachedAt) > ttl {
		// Expired, treat as cache miss
		return nil, nil
	}

	return cached.Data, nil
}

// Put stores data in the cache for a package version.
func (s *Store) Put(pkg, version string, data []byte) error {
	path := s.keyPath(pkg, version)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating cache subdirectory: %w", err)
	}

	// Wrap data with cache metadata
	cached := CachedData{
		CachedAt: time.Now(),
		Data:     data,
	}

	jsonData, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("marshaling cache data: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, jsonData, 0644); err != nil {
		return fmt.Errorf("writing temp cache file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Clean up temp file on failure
		return fmt.Errorf("renaming cache file: %w", err)
	}

	return nil
}

// keyPath generates the filesystem path for a cache key.
// Format: <cache-dir>/<url-encoded-name>@<version>.json
// Example: ~/.cache/npm-defense/@babel%2Fcore@7.24.0.json
func (s *Store) keyPath(pkg, version string) string {
	// URL-encode the package name to handle scoped packages (@babel/core → @babel%2Fcore)
	encodedPkg := url.PathEscape(pkg)

	// For scoped packages, we need to preserve the @ at the start
	// url.PathEscape encodes @ as %40, but we want @babel%2Fcore not %40babel%2Fcore
	if strings.HasPrefix(pkg, "@") && strings.HasPrefix(encodedPkg, "%40") {
		encodedPkg = "@" + strings.TrimPrefix(encodedPkg, "%40")
	}

	filename := fmt.Sprintf("%s@%s.json", encodedPkg, version)
	return filepath.Join(s.dir, filename)
}

// defaultCacheDir returns the default cache directory path.
// Respects XDG_CACHE_HOME, falls back to ~/.cache/npm-defense.
func defaultCacheDir() (string, error) {
	// Check XDG_CACHE_HOME first
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return filepath.Join(xdgCache, "npm-defense"), nil
	}

	// Fall back to ~/.cache/npm-defense
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	return filepath.Join(homeDir, ".cache", "npm-defense"), nil
}

// Dir returns the cache directory path.
func (s *Store) Dir() string {
	return s.dir
}
