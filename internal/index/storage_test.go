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

func TestSearchStoredPrefersHigherQueryCoverage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, "index.sqlite")

	fullMatchPath := filepath.Join(root, "full.go")
	partialMatchPath := filepath.Join(root, "partial.go")
	mustWriteIndexFile(t, fullMatchPath, "package main\nfunc main() { open sqlite database }\n")
	mustWriteIndexFile(t, partialMatchPath, "package main\nfunc main() { sqlite helper }\n")

	builder := NewBuilder()
	changed := NewInvertedIndex()
	if err := builder.AddFileWithModified(changed, fullMatchPath, time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatalf("AddFileWithModified(full) error = %v", err)
	}
	if err := builder.AddFileWithModified(changed, partialMatchPath, time.Unix(1_700_000_100, 0)); err != nil {
		t.Fatalf("AddFileWithModified(partial) error = %v", err)
	}

	if err := ApplyChanges(dbPath, changed, nil); err != nil {
		t.Fatalf("ApplyChanges() error = %v", err)
	}

	results, err := SearchStored(dbPath, "open sqlite", nil, 10)
	if err != nil {
		t.Fatalf("SearchStored() error = %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("len(results) = %d, want at least 2", len(results))
	}
	if results[0].Document.Path != fullMatchPath {
		t.Fatalf("results[0].Document.Path = %q, want %q", results[0].Document.Path, fullMatchPath)
	}
	if results[0].MatchedTerms != 2 {
		t.Fatalf("results[0].MatchedTerms = %d, want %d", results[0].MatchedTerms, 2)
	}
	if results[0].CoverageScore <= results[1].CoverageScore {
		t.Fatalf("results[0].CoverageScore = %f, want greater than %f", results[0].CoverageScore, results[1].CoverageScore)
	}
	if results[0].LexicalScore <= 0 {
		t.Fatalf("results[0].LexicalScore = %f, want > 0", results[0].LexicalScore)
	}
	if results[1].MatchedTerms >= results[0].MatchedTerms {
		t.Fatalf("results[1].MatchedTerms = %d, want less than %d", results[1].MatchedTerms, results[0].MatchedTerms)
	}
}

func TestSearchStoredFindsFilenameMatches(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, "index.sqlite")

	storagePath := filepath.Join(root, "storage.go")
	otherPath := filepath.Join(root, "main.go")
	mustWriteIndexFile(t, storagePath, "package index\nfunc SaveIndex() {}\n")
	mustWriteIndexFile(t, otherPath, "package main\nfunc main() {}\n")

	builder := NewBuilder()
	changed := NewInvertedIndex()
	if err := builder.AddFileWithModified(changed, storagePath, time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatalf("AddFileWithModified(storage) error = %v", err)
	}
	if err := builder.AddFileWithModified(changed, otherPath, time.Unix(1_700_000_100, 0)); err != nil {
		t.Fatalf("AddFileWithModified(other) error = %v", err)
	}

	if err := ApplyChanges(dbPath, changed, nil); err != nil {
		t.Fatalf("ApplyChanges() error = %v", err)
	}

	results, err := SearchStored(dbPath, "storage", nil, 10)
	if err != nil {
		t.Fatalf("SearchStored() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("len(results) = 0, want at least 1")
	}
	if results[0].Document.Path != storagePath {
		t.Fatalf("results[0].Document.Path = %q, want %q", results[0].Document.Path, storagePath)
	}
	if results[0].PathScore <= 0 {
		t.Fatalf("results[0].PathScore = %f, want > 0", results[0].PathScore)
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
