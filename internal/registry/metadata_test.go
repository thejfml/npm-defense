package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindPriorVersion(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		allVersions []string
		want        string
		wantErr     bool
	}{
		{
			name:        "finds prior version",
			current:     "1.14.1",
			allVersions: []string{"1.14.0", "1.14.1", "1.14.2"},
			want:        "1.14.0",
		},
		{
			name:        "no prior version exists",
			current:     "1.0.0",
			allVersions: []string{"1.0.0", "1.0.1", "1.0.2"},
			want:        "",
		},
		{
			name:        "skips pre-releases",
			current:     "2.0.0",
			allVersions: []string{"1.9.0", "1.9.1-beta", "2.0.0-rc1", "2.0.0"},
			want:        "1.9.0",
		},
		{
			name:        "includes pre-releases if current is pre-release",
			current:     "2.0.0-rc2",
			allVersions: []string{"1.9.0", "2.0.0-rc1", "2.0.0-rc2"},
			want:        "2.0.0-rc1",
		},
		{
			name:        "handles v prefix in input",
			current:     "v1.14.1",
			allVersions: []string{"v1.14.0", "v1.14.1"},
			want:        "1.14.0",
		},
		{
			name:        "finds highest prior among many",
			current:     "5.0.0",
			allVersions: []string{"1.0.0", "2.0.0", "3.0.0", "4.0.0", "4.9.9", "5.0.0"},
			want:        "4.9.9",
		},
		{
			name:        "empty current version",
			current:     "",
			allVersions: []string{"1.0.0"},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindPriorVersion(tt.current, tt.allVersions)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGetPublishers(t *testing.T) {
	tests := []struct {
		name  string
		pkg   *PackageMetadata
		lastN int
		want  []string
	}{
		{
			name: "gets publishers from last N versions",
			pkg: &PackageMetadata{
				Versions: map[string]*VersionMetadata{
					"1.0.0": {NPMUser: &NPMUser{Name: "alice"}},
					"1.1.0": {NPMUser: &NPMUser{Name: "alice"}},
					"1.2.0": {NPMUser: &NPMUser{Name: "bob"}},
					"2.0.0": {NPMUser: &NPMUser{Name: "charlie"}},
				},
			},
			lastN: 2,
			want:  []string{"bob", "charlie"},
		},
		{
			name: "deduplicates publishers",
			pkg: &PackageMetadata{
				Versions: map[string]*VersionMetadata{
					"1.0.0": {NPMUser: &NPMUser{Name: "alice"}},
					"1.1.0": {NPMUser: &NPMUser{Name: "alice"}},
					"1.2.0": {NPMUser: &NPMUser{Name: "alice"}},
				},
			},
			lastN: 3,
			want:  []string{"alice"},
		},
		{
			name: "handles more requested than available",
			pkg: &PackageMetadata{
				Versions: map[string]*VersionMetadata{
					"1.0.0": {NPMUser: &NPMUser{Name: "alice"}},
					"1.1.0": {NPMUser: &NPMUser{Name: "bob"}},
				},
			},
			lastN: 10,
			want:  []string{"alice", "bob"},
		},
		{
			name:  "nil package",
			pkg:   nil,
			lastN: 5,
			want:  nil,
		},
		{
			name: "missing npm user data",
			pkg: &PackageMetadata{
				Versions: map[string]*VersionMetadata{
					"1.0.0": {NPMUser: nil},
					"1.1.0": {NPMUser: &NPMUser{Name: "alice"}},
				},
			},
			lastN: 2,
			want:  []string{"alice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPublishers(tt.pkg, tt.lastN)
			require.Equal(t, tt.want, got)
		})
	}
}
