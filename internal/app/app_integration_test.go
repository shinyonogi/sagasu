package app

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunIndexSearchAndStatus(t *testing.T) {
	root := t.TempDir()
	indexPath := filepath.Join(root, "index.sqlite")

	mustWriteAppFile(t, filepath.Join(root, "main.go"), "package main\nfunc main() { println(\"hello\") }\n")
	mustWriteAppFile(t, filepath.Join(root, "README.md"), "# hello\n")

	if err := RunIndex([]string{root}, indexPath); err != nil {
		t.Fatalf("RunIndex() error = %v", err)
	}

	countOutput := captureStdout(t, func() {
		err := RunSearch("hello", indexPath, SearchOptions{Count: true})
		if err != nil {
			t.Fatalf("RunSearch(count) error = %v", err)
		}
	})
	if strings.TrimSpace(countOutput) != "2" {
		t.Fatalf("count output = %q, want %q", countOutput, "2")
	}

	jsonOutput := captureStdout(t, func() {
		err := RunSearch("hello", indexPath, SearchOptions{JSON: true, Limit: 1})
		if err != nil {
			t.Fatalf("RunSearch(json) error = %v", err)
		}
	})
	if !strings.Contains(jsonOutput, `"query": "hello"`) {
		t.Fatalf("json output missing query: %s", jsonOutput)
	}
	if !strings.Contains(jsonOutput, `"results"`) {
		t.Fatalf("json output missing results: %s", jsonOutput)
	}

	statusOutput := captureStdout(t, func() {
		err := RunStatus(indexPath)
		if err != nil {
			t.Fatalf("RunStatus() error = %v", err)
		}
	})
	if !strings.Contains(statusOutput, "STATUS") {
		t.Fatalf("status output missing header: %s", statusOutput)
	}
	if !strings.Contains(statusOutput, "docs") {
		t.Fatalf("status output missing docs line: %s", statusOutput)
	}
}

func TestRunSearchContextAndValidation(t *testing.T) {
	root := t.TempDir()
	indexPath := filepath.Join(root, "index.sqlite")

	mustWriteAppFile(t, filepath.Join(root, "main.go"), "package main\n// hello\nfunc main() { println(\"hello\") }\n")

	if err := RunIndex([]string{root}, indexPath); err != nil {
		t.Fatalf("RunIndex() error = %v", err)
	}

	contextOutput := captureStdout(t, func() {
		err := RunSearch("hello", indexPath, SearchOptions{Context: 1, Limit: 1})
		if err != nil {
			t.Fatalf("RunSearch(context) error = %v", err)
		}
	})
	if !strings.Contains(contextOutput, "|") {
		t.Fatalf("context output missing context lines: %s", contextOutput)
	}

	err := RunSearch("hello", indexPath, SearchOptions{JSON: true, Count: true})
	if err == nil {
		t.Fatalf("RunSearch(json+count) error = nil, want error")
	}
}

func TestRunIndexIncrementalUpdate(t *testing.T) {
	root := t.TempDir()
	indexPath := filepath.Join(root, "index.sqlite")
	path := filepath.Join(root, "main.go")

	mustWriteAppFile(t, path, "package main\nfunc main() { println(\"hello\") }\n")
	if err := RunIndex([]string{root}, indexPath); err != nil {
		t.Fatalf("RunIndex(first) error = %v", err)
	}

	mustWriteAppFile(t, path, "package main\nfunc main() { println(\"updated\") }\n")
	updatedTime := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, updatedTime, updatedTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}
	if err := RunIndex([]string{root}, indexPath); err != nil {
		t.Fatalf("RunIndex(second) error = %v", err)
	}

	countOutput := captureStdout(t, func() {
		err := RunSearch("updated", indexPath, SearchOptions{Count: true})
		if err != nil {
			t.Fatalf("RunSearch(updated) error = %v", err)
		}
	})
	if strings.TrimSpace(countOutput) != "1" {
		t.Fatalf("updated count output = %q, want %q", countOutput, "1")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer r.Close()

	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("stdout close error = %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy() error = %v", err)
	}

	return buf.String()
}

func mustWriteAppFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
