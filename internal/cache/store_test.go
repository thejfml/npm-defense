package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("creates cache directory", func(t *testing.T) {
		dir := t.TempDir()
		cacheDir := filepath.Join(dir, "cache")

		store, err := New(cacheDir)
		require.NoError(t, err)
		require.Equal(t, cacheDir, store.Dir())

		// Verify directory was created
		info, err := os.Stat(cacheDir)
		require.NoError(t, err)
		require.True(t, info.IsDir())
	})

	t.Run("uses default cache dir when empty", func(t *testing.T) {
		store, err := New("")
		require.NoError(t, err)
		require.NotEmpty(t, store.Dir())
		require.Contains(t, store.Dir(), "npm-defense")
	})
}

func TestPutAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	require.NoError(t, err)

	t.Run("put and get data", func(t *testing.T) {
		pkg := "axios"
		version := "1.14.0"
		data := []byte(`{"name":"axios","version":"1.14.0"}`)

		// Put data
		err := store.Put(pkg, version, data)
		require.NoError(t, err)

		// Get data (no TTL)
		got, err := store.Get(pkg, version, 0)
		require.NoError(t, err)
		require.Equal(t, data, got)
	})

	t.Run("get non-existent key returns nil", func(t *testing.T) {
		got, err := store.Get("nonexistent", "1.0.0", 0)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	t.Run("expired data returns nil", func(t *testing.T) {
		pkg := "short-lived"
		version := "1.0.0"
		data := []byte(`{"test":"data"}`)

		err := store.Put(pkg, version, data)
		require.NoError(t, err)

		// Sleep to ensure expiry
		time.Sleep(10 * time.Millisecond)

		// Get with very short TTL should return nil (expired)
		got, err := store.Get(pkg, version, 1*time.Millisecond)
		require.NoError(t, err)
		require.Nil(t, got, "expired data should return nil")
	})

	t.Run("scoped package names", func(t *testing.T) {
		pkg := "@babel/core"
		version := "7.24.0"
		data := []byte(`{"name":"@babel/core","version":"7.24.0"}`)

		err := store.Put(pkg, version, data)
		require.NoError(t, err)

		got, err := store.Get(pkg, version, 0)
		require.NoError(t, err)
		require.Equal(t, data, got)

		// Verify file was created with correct name
		expectedPath := store.keyPath(pkg, version)
		require.Contains(t, expectedPath, "@babel%2Fcore@7.24.0.json")

		_, err = os.Stat(expectedPath)
		require.NoError(t, err, "cache file should exist")
	})
}

func TestKeyPath(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	require.NoError(t, err)

	tests := []struct {
		pkg     string
		version string
		want    string
	}{
		{"axios", "1.14.0", "axios@1.14.0.json"},
		{"@babel/core", "7.24.0", "@babel%2Fcore@7.24.0.json"},
		{"lodash", "4.17.21", "lodash@4.17.21.json"},
	}

	for _, tt := range tests {
		t.Run(tt.pkg, func(t *testing.T) {
			got := store.keyPath(tt.pkg, tt.version)
			require.Equal(t, filepath.Join(dir, tt.want), got)
		})
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir)
	require.NoError(t, err)

	pkg := "test-pkg"
	version := "1.0.0"
	data := []byte(`{"test":"data"}`)

	err = store.Put(pkg, version, data)
	require.NoError(t, err)

	// Verify no .tmp file left behind
	tmpPath := store.keyPath(pkg, version) + ".tmp"
	_, err = os.Stat(tmpPath)
	require.True(t, os.IsNotExist(err), "temp file should not exist after successful write")

	// Verify data is correct
	got, err := store.Get(pkg, version, 0)
	require.NoError(t, err)
	require.Equal(t, data, got)
}
