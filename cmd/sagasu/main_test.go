package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchRejectsJSONAndCountTogether(t *testing.T) {
	t.Parallel()

	cmd := buildRootCommand()
	cmd.SetArgs([]string{"search", "hello", "--json", "--count"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("Execute() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "--json and --count cannot be used together") {
		t.Fatalf("Execute() error = %q, want json/count validation", err)
	}
}

func TestInfoAliasWorks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	indexPath := filepath.Join(root, "index.sqlite")
	if err := os.WriteFile(indexPath, []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := buildRootCommand()
	cmd.SetArgs([]string{"info", "--index-path", indexPath})

	output := captureCommandStdout(t, func() error {
		return cmd.Execute()
	})

	if !strings.Contains(output, "STATUS") {
		t.Fatalf("output missing STATUS header: %s", output)
	}
}

func captureCommandStdout(t *testing.T, fn func() error) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer r.Close()

	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	if err := fn(); err != nil {
		t.Fatalf("command error = %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("stdout close error = %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}

	return buf.String()
}
