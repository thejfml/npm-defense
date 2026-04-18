package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/thejfml/npm-defense/internal/cache"
)

const (
	defaultRegistry = "https://registry.npmjs.org"
	defaultTimeout  = 30 * time.Second
	defaultRetries  = 3
	userAgent       = "npm-defense/v0.1 (+github.com/thejfml/npm-defense)"
)

// Client provides access to npm registry with caching and retry logic.
type Client struct {
	httpClient *http.Client
	cache      *cache.Store
	registry   string
	offline    bool
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client (for testing).
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithRegistry sets a custom registry URL.
func WithRegistry(registry string) ClientOption {
	return func(c *Client) {
		c.registry = registry
	}
}

// WithOffline enables offline mode (cache-only).
func WithOffline(offline bool) ClientOption {
	return func(c *Client) {
		c.offline = offline
	}
}

// NewClient creates a new registry client.
func NewClient(cacheStore *cache.Store, opts ...ClientOption) *Client {
	client := &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		cache:    cacheStore,
		registry: defaultRegistry,
		offline:  false,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// GetPackageMetadata fetches full package metadata (all versions).
// Uses 24h cache TTL since the versions list is mutable.
func (c *Client) GetPackageMetadata(name string) (*PackageMetadata, error) {
	// Check cache (24h TTL for mutable data)
	cached, err := c.cache.Get(name, "all", 24*time.Hour)
	if err == nil && cached != nil {
		var pkg PackageMetadata
		if err := json.Unmarshal(cached, &pkg); err == nil {
			return &pkg, nil
		}
	}

	// Offline mode: error if cache miss
	if c.offline {
		return nil, fmt.Errorf("offline mode: no cached data for %s", name)
	}

	// Fetch from registry
	escapedName := url.PathEscape(name)
	endpoint := fmt.Sprintf("%s/%s", c.registry, escapedName)

	data, err := c.fetchWithRetry(endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetching package metadata: %w", err)
	}

	// Parse response
	var pkg PackageMetadata
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parsing package metadata: %w", err)
	}

	// Cache the response
	if err := c.cache.Put(name, "all", data); err != nil {
		// Log but don't fail on cache write error
		// TODO: add logging when logger is available
	}

	return &pkg, nil
}

// GetVersionMetadata fetches metadata for a specific version.
// Uses 0 TTL (never expires) since version metadata is immutable.
func (c *Client) GetVersionMetadata(name, version string) (*VersionMetadata, error) {
	// Check cache (no TTL for immutable data)
	cached, err := c.cache.Get(name, version, 0)
	if err == nil && cached != nil {
		var meta VersionMetadata
		if err := json.Unmarshal(cached, &meta); err == nil {
			return &meta, nil
		}
	}

	// Offline mode: error if cache miss
	if c.offline {
		return nil, fmt.Errorf("offline mode: no cached data for %s@%s", name, version)
	}

	// Fetch from registry
	escapedName := url.PathEscape(name)
	endpoint := fmt.Sprintf("%s/%s/%s", c.registry, escapedName, version)

	data, err := c.fetchWithRetry(endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetching version metadata: %w", err)
	}

	// Parse response
	var meta VersionMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing version metadata: %w", err)
	}

	// Cache the response
	if err := c.cache.Put(name, version, data); err != nil {
		// Log but don't fail on cache write error
	}

	return &meta, nil
}

// fetchWithRetry performs an HTTP GET with exponential backoff retry.
func (c *Client) fetchWithRetry(url string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < defaultRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			time.Sleep(backoff)
		}

		data, err := c.fetch(url)
		if err == nil {
			return data, nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryable(err) {
			return nil, lastErr
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", defaultRetries, lastErr)
}

// fetch performs a single HTTP GET request.
func (c *Client) fetch(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			URL:        url,
		}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	return data, nil
}

// isRetryable determines if an error should trigger a retry.
func isRetryable(err error) bool {
	if httpErr, ok := err.(*HTTPError); ok {
		// Retry on 5xx server errors and 429 rate limiting
		return httpErr.StatusCode >= 500 || httpErr.StatusCode == 429
	}

	// Retry on network errors
	return true
}

// HTTPError represents an HTTP error response.
type HTTPError struct {
	StatusCode int
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.URL)
}
