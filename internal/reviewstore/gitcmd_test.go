package reviewstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/martinhg/capiko-ai/internal/rdd"
)

// --- Output-parsing unit tests (no git binary invoked) ---

func TestParseLsTree(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		want    []rdd.PathMode
		wantErr bool
	}{
		{
			name: "single entry",
			out:  "100644 blob a1b2c3d4e5f6\tinternal/reviewstore/lock.go",
			want: []rdd.PathMode{{Path: "internal/reviewstore/lock.go", Mode: "100644"}},
		},
		{
			name: "multiple entries",
			out: "100644 blob aaa\tREADME.md\n" +
				"100755 blob bbb\tscripts/run.sh\n",
			want: []rdd.PathMode{
				{Path: "README.md", Mode: "100644"},
				{Path: "scripts/run.sh", Mode: "100755"},
			},
		},
		{
			name: "empty output",
			out:  "",
			want: nil,
		},
		{
			name:    "missing tab separator",
			out:     "100644 blob aaa README.md",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLsTree(tt.out)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseLsTree() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseLsTree() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseDiffTree(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		want    []rdd.PathMode
		wantErr bool
	}{
		{
			name: "single modified file",
			out:  ":100644 100644 aaa bbb M\tinternal/rdd/identity.go",
			want: []rdd.PathMode{{Path: "internal/rdd/identity.go", Mode: "100644"}},
		},
		{
			name: "multiple changed files",
			out: ":100644 100644 aaa bbb M\tfile1.go\n" +
				":000000 100644 000 ccc A\tfile2.go\n",
			want: []rdd.PathMode{
				{Path: "file1.go", Mode: "100644"},
				{Path: "file2.go", Mode: "100644"},
			},
		},
		{
			name: "empty output",
			out:  "",
			want: nil,
		},
		{
			name:    "missing tab separator",
			out:     ":100644 100644 aaa bbb M file1.go",
			wantErr: true,
		},
		{
			name:    "too few metadata fields",
			out:     ":100644\tfile1.go",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDiffTree(tt.out)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseDiffTree() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDiffTree() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// --- Seam-swap tests: each git*/ seam is a package-level var, swappable
// without a real git binary or repository. ---

func TestGitSeams_Swappable(t *testing.T) {
	wantErr := errors.New("boom")

	t.Run("gitCommonDir", func(t *testing.T) {
		original := gitCommonDir
		t.Cleanup(func() { gitCommonDir = original })
		gitCommonDir = func(string) (string, error) { return "", wantErr }
		if _, err := gitCommonDir("x"); !errors.Is(err, wantErr) {
			t.Errorf("gitCommonDir() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("gitWriteTree", func(t *testing.T) {
		original := gitWriteTree
		t.Cleanup(func() { gitWriteTree = original })
		gitWriteTree = func(string) (string, error) { return "", wantErr }
		if _, err := gitWriteTree("x"); !errors.Is(err, wantErr) {
			t.Errorf("gitWriteTree() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("gitRevParseTree", func(t *testing.T) {
		original := gitRevParseTree
		t.Cleanup(func() { gitRevParseTree = original })
		gitRevParseTree = func(string, string) (string, error) { return "", wantErr }
		if _, err := gitRevParseTree("x", "HEAD"); !errors.Is(err, wantErr) {
			t.Errorf("gitRevParseTree() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("gitLsTree", func(t *testing.T) {
		original := gitLsTree
		t.Cleanup(func() { gitLsTree = original })
		gitLsTree = func(string, string) ([]rdd.PathMode, error) { return nil, wantErr }
		if _, err := gitLsTree("x", "deadbeef"); !errors.Is(err, wantErr) {
			t.Errorf("gitLsTree() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("gitDiffTree", func(t *testing.T) {
		original := gitDiffTree
		t.Cleanup(func() { gitDiffTree = original })
		gitDiffTree = func(string, string, string) ([]rdd.PathMode, error) { return nil, wantErr }
		if _, err := gitDiffTree("x", "a", "b"); !errors.Is(err, wantErr) {
			t.Errorf("gitDiffTree() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("gitRemoteOriginURL", func(t *testing.T) {
		original := gitRemoteOriginURL
		t.Cleanup(func() { gitRemoteOriginURL = original })
		gitRemoteOriginURL = func(string) (string, error) { return "", wantErr }
		if _, err := gitRemoteOriginURL("x"); !errors.Is(err, wantErr) {
			t.Errorf("gitRemoteOriginURL() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("gitIsBareRepo", func(t *testing.T) {
		original := gitIsBareRepo
		t.Cleanup(func() { gitIsBareRepo = original })
		gitIsBareRepo = func(string) (bool, error) { return false, wantErr }
		if _, err := gitIsBareRepo("x"); !errors.Is(err, wantErr) {
			t.Errorf("gitIsBareRepo() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("gitIsShallowRepo", func(t *testing.T) {
		original := gitIsShallowRepo
		t.Cleanup(func() { gitIsShallowRepo = original })
		gitIsShallowRepo = func(string) (bool, error) { return false, wantErr }
		if _, err := gitIsShallowRepo("x"); !errors.Is(err, wantErr) {
			t.Errorf("gitIsShallowRepo() error = %v, want %v", err, wantErr)
		}
	})
}
