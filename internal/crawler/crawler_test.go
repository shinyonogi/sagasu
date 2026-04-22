package crawler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectFilesFiltersAndIgnores(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "main.go"), "package main\n")
	mustWriteFile(t, filepath.Join(root, "README.md"), "# hello\n")
	mustWriteFile(t, filepath.Join(root, "notes.txt"), "hi\n")
	mustWriteFile(t, filepath.Join(root, "archive.zip"), "nope\n")
	mustWriteFile(t, filepath.Join(root, "node_modules", "dep.js"), "console.log('x')\n")
	mustWriteFile(t, filepath.Join(root, ".git", "config"), "[core]\n")

	got, err := CollectFiles([]string{root}, Options{})
	if err != nil {
		t.Fatalf("CollectFiles() error = %v", err)
	}

	paths := map[string]struct{}{}
	for _, entry := range got {
		paths[filepath.Base(entry.Path)] = struct{}{}
	}

	for _, name := range []string{"main.go", "README.md", "notes.txt"} {
		if _, ok := paths[name]; !ok {
			t.Fatalf("expected %q to be collected, got %#v", name, paths)
		}
	}

	for _, name := range []string{"archive.zip", "dep.js", "config"} {
		if _, ok := paths[name]; ok {
			t.Fatalf("did not expect %q to be collected, got %#v", name, paths)
		}
	}
}

func TestCollectFilesIncludeExcludePatterns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "cmd", "main.go"), "package main\n")
	mustWriteFile(t, filepath.Join(root, "internal", "service.go"), "package internal\n")
	mustWriteFile(t, filepath.Join(root, "internal", "service_test.go"), "package internal\n")

	got, err := CollectFiles([]string{root}, Options{
		IncludePatterns: []string{"internal/**/*.go"},
		ExcludePatterns: []string{"**/*_test.go"},
	})
	if err != nil {
		t.Fatalf("CollectFiles() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if filepath.Base(got[0].Path) != "service.go" {
		t.Fatalf("got[0].Path = %q, want service.go", got[0].Path)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
