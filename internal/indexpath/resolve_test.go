package indexpath

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveForRootsExplicit(t *testing.T) {
	t.Parallel()

	got, err := ResolveForRoots("/tmp/custom.sqlite", []string{"."})
	if err != nil {
		t.Fatalf("ResolveForRoots() error = %v", err)
	}
	if got != "/tmp/custom.sqlite" {
		t.Fatalf("ResolveForRoots() = %q, want %q", got, "/tmp/custom.sqlite")
	}
}

func TestResolveForRootsManagedPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	got, err := ResolveForRoots("", []string{root})
	if err != nil {
		t.Fatalf("ResolveForRoots() error = %v", err)
	}
	if !strings.HasSuffix(got, ".sqlite") {
		t.Fatalf("ResolveForRoots() = %q, want sqlite path", got)
	}
	if !strings.Contains(filepath.Base(got), filepath.Base(root)) {
		t.Fatalf("ResolveForRoots() = %q, want basename %q in path", got, filepath.Base(root))
	}
}
