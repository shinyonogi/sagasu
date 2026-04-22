package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "main.go")
	content := "package main\nfunc main() { hello_hello() }\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	modified := time.Unix(1_700_000_000, 0)
	got, err := NewBuilder().BuildFile(path, modified)
	if err != nil {
		t.Fatalf("BuildFile() error = %v", err)
	}

	if len(got.Documents) != 1 {
		t.Fatalf("len(Documents) = %d, want 1", len(got.Documents))
	}

	document := got.Documents[path]
	if document.Ext != "go" {
		t.Fatalf("document.Ext = %q, want %q", document.Ext, "go")
	}
	if document.Modified != modified.Unix() {
		t.Fatalf("document.Modified = %d, want %d", document.Modified, modified.Unix())
	}

	if len(got.Chunks) != 2 {
		t.Fatalf("len(Chunks) = %d, want 2", len(got.Chunks))
	}

	postings := got.Terms["hello_hello"]
	if len(postings) != 1 {
		t.Fatalf("len(Terms[hello_hello]) = %d, want 1", len(postings))
	}
	if postings[0].TF != 1 {
		t.Fatalf("posting.TF = %d, want 1", postings[0].TF)
	}
}
