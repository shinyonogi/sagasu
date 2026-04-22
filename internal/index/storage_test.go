package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApplyChangesSearchAndStats(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, "index.sqlite")

	goFile := filepath.Join(root, "main.go")
	mdFile := filepath.Join(root, "README.md")
	mustWriteIndexFile(t, goFile, "package main\nfunc main() { hello() }\n")
	mustWriteIndexFile(t, mdFile, "# hello docs\nhello world\n")

	builder := NewBuilder()
	changed := NewInvertedIndex()
	if err := builder.AddFileWithModified(changed, goFile, time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatalf("AddFileWithModified(go) error = %v", err)
	}
	if err := builder.AddFileWithModified(changed, mdFile, time.Unix(1_700_000_100, 0)); err != nil {
		t.Fatalf("AddFileWithModified(md) error = %v", err)
	}

	if err := ApplyChanges(dbPath, changed, nil); err != nil {
		t.Fatalf("ApplyChanges() error = %v", err)
	}

	documents, err := LoadDocuments(dbPath)
	if err != nil {
		t.Fatalf("LoadDocuments() error = %v", err)
	}
	if len(documents) != 2 {
		t.Fatalf("len(documents) = %d, want 2", len(documents))
	}

	results, err := SearchStored(dbPath, "hello", nil, 10)
	if err != nil {
		t.Fatalf("SearchStored() error = %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}

	goOnly, err := SearchStored(dbPath, "hello", []string{".go"}, 10)
	if err != nil {
		t.Fatalf("SearchStored(.go) error = %v", err)
	}
	if len(goOnly) != 1 {
		t.Fatalf("len(goOnly) = %d, want 1", len(goOnly))
	}
	if goOnly[0].Document.Ext != "go" {
		t.Fatalf("goOnly[0].Document.Ext = %q, want %q", goOnly[0].Document.Ext, "go")
	}

	stats, err := LoadStats(dbPath)
	if err != nil {
		t.Fatalf("LoadStats() error = %v", err)
	}
	if stats.Documents != 2 {
		t.Fatalf("stats.Documents = %d, want 2", stats.Documents)
	}
	if stats.Chunks != 4 {
		t.Fatalf("stats.Chunks = %d, want 4", stats.Chunks)
	}
	if stats.Terms == 0 {
		t.Fatalf("stats.Terms = 0, want > 0")
	}
	if len(stats.Exts) != 2 {
		t.Fatalf("len(stats.Exts) = %d, want 2", len(stats.Exts))
	}

	phraseResults, err := SearchStored(dbPath, `"hello world"`, nil, 10)
	if err != nil {
		t.Fatalf("SearchStored(phrase) error = %v", err)
	}
	if len(phraseResults) != 1 {
		t.Fatalf("len(phraseResults) = %d, want 1", len(phraseResults))
	}
	if phraseResults[0].Document.Ext != "md" {
		t.Fatalf("phraseResults[0].Document.Ext = %q, want %q", phraseResults[0].Document.Ext, "md")
	}
}

func TestApplyChangesDeletesDocuments(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, "index.sqlite")
	goFile := filepath.Join(root, "main.go")
	mustWriteIndexFile(t, goFile, "package main\nfunc main() { hello() }\n")

	builder := NewBuilder()
	initial := NewInvertedIndex()
	if err := builder.AddFileWithModified(initial, goFile, time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatalf("AddFileWithModified() error = %v", err)
	}
	if err := ApplyChanges(dbPath, initial, nil); err != nil {
		t.Fatalf("ApplyChanges(initial) error = %v", err)
	}

	if err := ApplyChanges(dbPath, NewInvertedIndex(), []string{goFile}); err != nil {
		t.Fatalf("ApplyChanges(delete) error = %v", err)
	}

	documents, err := LoadDocuments(dbPath)
	if err != nil {
		t.Fatalf("LoadDocuments() error = %v", err)
	}
	if len(documents) != 0 {
		t.Fatalf("len(documents) = %d, want 0", len(documents))
	}
}

func mustWriteIndexFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
