package lockfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thejfml/npm-defense/internal/types"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		lockfile    string
		wantErr     bool
		wantCount   int
		checkPkg    func(t *testing.T, packages []types.Package)
	}{
		{
			name:      "minimal lockfile",
			lockfile:  "../../testdata/fixtures/minimal.json",
			wantErr:   false,
			wantCount: 1,
			checkPkg: func(t *testing.T, packages []types.Package) {
				require.Len(t, packages, 1)

				pkg := packages[0]
				require.Equal(t, "lodash", pkg.Name)
				require.Equal(t, "4.17.21", pkg.Version)
				require.True(t, pkg.IsDirect, "lodash should be direct dependency")
				require.Equal(t, []string{"lodash"}, pkg.Path)
			},
		},
		{
			name:     "unsupported lockfile version",
			lockfile: "testdata/lockfile-v1.json",
			wantErr:  true,
		},
		{
			name:     "nonexistent file",
			lockfile: "testdata/does-not-exist.json",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file if needed
			if tt.name == "unsupported lockfile version" {
				tempDir := t.TempDir()
				lockfilePath := filepath.Join(tempDir, "package-lock.json")
				err := os.WriteFile(lockfilePath, []byte(`{
					"name": "test",
					"version": "1.0.0",
					"lockfileVersion": 1,
					"dependencies": {}
				}`), 0644)
				require.NoError(t, err)
				tt.lockfile = lockfilePath
			}

			packages, err := Parse(tt.lockfile)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.wantCount > 0 {
				require.Len(t, packages, tt.wantCount)
			}
			if tt.checkPkg != nil {
				tt.checkPkg(t, packages)
			}
		})
	}
}

func TestExtractPackageName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"node_modules/axios", "axios"},
		{"node_modules/@babel/core", "@babel/core"},
		{"node_modules/axios/node_modules/follow-redirects", "follow-redirects"},
		{"node_modules/@babel/core/node_modules/@babel/helper", "@babel/helper"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractPackageName(tt.path)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildDependencyPath(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"node_modules/axios", []string{"axios"}},
		{"node_modules/axios/node_modules/follow-redirects", []string{"axios", "follow-redirects"}},
		{"node_modules/@babel/core", []string{"@babel/core"}},
		{"node_modules/a/node_modules/b/node_modules/c", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := buildDependencyPath(tt.path)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFindLockfile(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string)
		wantFile string
		wantErr  bool
	}{
		{
			name: "finds package-lock.json",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644)
			},
			wantFile: "package-lock.json",
		},
		{
			name: "finds npm-shrinkwrap.json",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "npm-shrinkwrap.json"), []byte("{}"), 0644)
			},
			wantFile: "npm-shrinkwrap.json",
		},
		{
			name: "prefers package-lock.json over shrinkwrap",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644)
				os.WriteFile(filepath.Join(dir, "npm-shrinkwrap.json"), []byte("{}"), 0644)
			},
			wantFile: "package-lock.json",
		},
		{
			name:    "no lockfile",
			setup:   func(dir string) {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(dir)

			got, err := FindLockfile(dir)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, filepath.Join(dir, tt.wantFile), got)
		})
	}
}
