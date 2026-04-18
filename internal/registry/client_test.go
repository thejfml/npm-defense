package registry

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thejfml/npm-defense/internal/cache"
)

func TestGetPackageMetadata(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantErr    bool
		checkCache bool
	}{
		{
			name:       "successful fetch",
			statusCode: 200,
			response: `{
				"name": "axios",
				"versions": {
					"1.14.0": {
						"name": "axios",
						"version": "1.14.0",
						"dependencies": {}
					},
					"1.14.1": {
						"name": "axios",
						"version": "1.14.1",
						"dependencies": {
							"plain-crypto-js": "^4.2.1"
						}
					}
				},
				"time": {
					"created": "2014-08-06T16:27:52.214Z",
					"modified": "2026-03-31T00:21:00.000Z",
					"1.14.0": "2026-03-30T12:00:00.000Z",
					"1.14.1": "2026-03-31T00:21:00.000Z"
				}
			}`,
			wantErr:    false,
			checkCache: true,
		},
		{
			name:       "404 not found",
			statusCode: 404,
			response:   `{"error":"Not found"}`,
			wantErr:    true,
		},
		{
			name:       "invalid json",
			statusCode: 200,
			response:   `{invalid json`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "GET", r.Method)
				require.Contains(t, r.Header.Get("User-Agent"), "npm-defense")

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			// Create client with test server
			cacheStore, _ := cache.New(t.TempDir())
			client := NewClient(
				cacheStore,
				WithRegistry(server.URL),
				WithHTTPClient(server.Client()),
			)

			// Test fetch
			pkg, err := client.GetPackageMetadata("axios")

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, pkg)
			require.Equal(t, "axios", pkg.Name)
			require.Len(t, pkg.Versions, 2)

			// Verify cache if requested
			if tt.checkCache {
				cached, err := cacheStore.Get("axios", "all", 24*time.Hour)
				require.NoError(t, err)
				require.NotNil(t, cached, "data should be cached")
			}
		})
	}
}

func TestGetVersionMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)
		require.Equal(t, "/axios/1.14.0", r.URL.Path)

		w.WriteHeader(200)
		w.Write([]byte(`{
			"name": "axios",
			"version": "1.14.0",
			"dependencies": {
				"follow-redirects": "^1.15.0"
			},
			"scripts": {},
			"_npmUser": {
				"name": "jasonsaayman",
				"email": "jason@example.com"
			},
			"dist": {
				"tarball": "https://registry.npmjs.org/axios/-/axios-1.14.0.tgz",
				"integrity": "sha512-..."
			}
		}`))
	}))
	defer server.Close()

	cacheStore, _ := cache.New(t.TempDir())
	client := NewClient(
		cacheStore,
		WithRegistry(server.URL),
		WithHTTPClient(server.Client()),
	)

	// Test fetch
	meta, err := client.GetVersionMetadata("axios", "1.14.0")
	require.NoError(t, err)
	require.NotNil(t, meta)
	require.Equal(t, "axios", meta.Name)
	require.Equal(t, "1.14.0", meta.Version)
	require.Equal(t, "jasonsaayman", meta.NPMUser.Name)

	// Verify immutable cache (TTL=0)
	cached, err := cacheStore.Get("axios", "1.14.0", 0)
	require.NoError(t, err)
	require.NotNil(t, cached, "version metadata should be cached")
}

func TestCacheHit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
		w.Write([]byte(`{"name":"axios","versions":{},"time":{"created":"2014-08-06T16:27:52.214Z","modified":"2026-03-31T00:21:00.000Z"}}`))
	}))
	defer server.Close()

	cacheStore, _ := cache.New(t.TempDir())
	client := NewClient(
		cacheStore,
		WithRegistry(server.URL),
		WithHTTPClient(server.Client()),
	)

	// First call - cache miss, fetches from network
	_, err := client.GetPackageMetadata("axios")
	require.NoError(t, err)
	require.Equal(t, 1, callCount, "should fetch from network on first call")

	// Second call - cache hit, no network request
	_, err = client.GetPackageMetadata("axios")
	require.NoError(t, err)
	require.Equal(t, 1, callCount, "should not fetch again, cache hit")
}

func TestOfflineMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make network requests in offline mode")
	}))
	defer server.Close()

	cacheStore, _ := cache.New(t.TempDir())
	client := NewClient(
		cacheStore,
		WithRegistry(server.URL),
		WithHTTPClient(server.Client()),
		WithOffline(true),
	)

	// Should error since cache is empty and we're offline
	_, err := client.GetPackageMetadata("axios")
	require.Error(t, err)
	require.Contains(t, err.Error(), "offline mode")
}

func TestRetryOn5xx(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			w.WriteHeader(500) // Fail first 2 attempts
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"name":"axios","versions":{},"time":{"created":"2014-08-06T16:27:52.214Z","modified":"2026-03-31T00:21:00.000Z"}}`))
	}))
	defer server.Close()

	cacheStore, _ := cache.New(t.TempDir())
	client := NewClient(
		cacheStore,
		WithRegistry(server.URL),
		WithHTTPClient(server.Client()),
	)

	// Should succeed after retries
	_, err := client.GetPackageMetadata("axios")
	require.NoError(t, err)
	require.Equal(t, 3, attemptCount, "should retry on 5xx errors")
}

func TestNoRetryOn404(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"Not found"}`))
	}))
	defer server.Close()

	cacheStore, _ := cache.New(t.TempDir())
	client := NewClient(
		cacheStore,
		WithRegistry(server.URL),
		WithHTTPClient(server.Client()),
	)

	// Should not retry on 404
	_, err := client.GetPackageMetadata("axios")
	require.Error(t, err)
	require.Equal(t, 1, attemptCount, "should not retry on 404")
}
