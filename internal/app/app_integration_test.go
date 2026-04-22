package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

	if err := RunIndex([]string{root}, indexPath, IndexOptions{}); err != nil {
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
	if !strings.Contains(jsonOutput, `"lexical_score"`) {
		t.Fatalf("json output missing lexical_score: %s", jsonOutput)
	}

	statusOutput := captureStdout(t, func() {
		err := RunStatus(indexPath, StatusOptions{})
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

	statusJSONOutput := captureStdout(t, func() {
		err := RunStatus(indexPath, StatusOptions{JSON: true})
		if err != nil {
			t.Fatalf("RunStatus(json) error = %v", err)
		}
	})
	if !strings.Contains(statusJSONOutput, `"documents": 2`) {
		t.Fatalf("status json output missing documents count: %s", statusJSONOutput)
	}
	if !strings.Contains(statusJSONOutput, `"exts"`) {
		t.Fatalf("status json output missing exts: %s", statusJSONOutput)
	}
}

func TestRunSearchContextAndValidation(t *testing.T) {
	root := t.TempDir()
	indexPath := filepath.Join(root, "index.sqlite")

	mustWriteAppFile(t, filepath.Join(root, "main.go"), "package main\n// hello\nfunc main() { println(\"hello\") }\n")

	if err := RunIndex([]string{root}, indexPath, IndexOptions{}); err != nil {
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

	pathOnlyOutput := captureStdout(t, func() {
		err := RunSearch("hello", indexPath, SearchOptions{PathOnly: true})
		if err != nil {
			t.Fatalf("RunSearch(path-only) error = %v", err)
		}
	})
	if !strings.Contains(pathOnlyOutput, ":2") {
		t.Fatalf("path-only output missing path:line: %s", pathOnlyOutput)
	}

	filesOnlyOutput := captureStdout(t, func() {
		err := RunSearch("hello", indexPath, SearchOptions{FilesOnly: true})
		if err != nil {
			t.Fatalf("RunSearch(files-only) error = %v", err)
		}
	})
	if got := strings.Count(strings.TrimSpace(filesOnlyOutput), "\n"); got != 0 {
		t.Fatalf("files-only output should contain one unique path, got: %s", filesOnlyOutput)
	}

	phraseOutput := captureStdout(t, func() {
		err := RunSearch(`"hello"`, indexPath, SearchOptions{Count: true})
		if err != nil {
			t.Fatalf("RunSearch(phrase) error = %v", err)
		}
	})
	if strings.TrimSpace(phraseOutput) != "2" {
		t.Fatalf("phrase output = %q, want %q", phraseOutput, "2")
	}
}

func TestRunIndexIncrementalUpdate(t *testing.T) {
	root := t.TempDir()
	indexPath := filepath.Join(root, "index.sqlite")
	path := filepath.Join(root, "main.go")

	mustWriteAppFile(t, path, "package main\nfunc main() { println(\"hello\") }\n")
	if err := RunIndex([]string{root}, indexPath, IndexOptions{}); err != nil {
		t.Fatalf("RunIndex(first) error = %v", err)
	}

	mustWriteAppFile(t, path, "package main\nfunc main() { println(\"updated\") }\n")
	updatedTime := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, updatedTime, updatedTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}
	if err := RunIndex([]string{root}, indexPath, IndexOptions{}); err != nil {
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

func TestRunRebuildAndDoctor(t *testing.T) {
	root := t.TempDir()
	indexPath := filepath.Join(root, "index.sqlite")
	path := filepath.Join(root, "main.go")

	mustWriteAppFile(t, path, "package main\nfunc main() { println(\"hello\") }\n")
	if err := RunIndex([]string{root}, indexPath, IndexOptions{}); err != nil {
		t.Fatalf("RunIndex() error = %v", err)
	}

	mustWriteAppFile(t, path, "package main\nfunc main() { println(\"rebuilt\") }\n")
	updatedTime := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, updatedTime, updatedTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}
	if err := RunRebuild([]string{root}, indexPath, IndexOptions{}); err != nil {
		t.Fatalf("RunRebuild() error = %v", err)
	}

	countOutput := captureStdout(t, func() {
		err := RunSearch("rebuilt", indexPath, SearchOptions{Count: true})
		if err != nil {
			t.Fatalf("RunSearch(rebuilt) error = %v", err)
		}
	})
	if strings.TrimSpace(countOutput) != "1" {
		t.Fatalf("rebuilt count output = %q, want %q", countOutput, "1")
	}

	doctorOutput := captureStdout(t, func() {
		err := RunDoctor(indexPath, DoctorOptions{})
		if err != nil {
			t.Fatalf("RunDoctor() error = %v", err)
		}
	})
	if !strings.Contains(doctorOutput, "DOCTOR") {
		t.Fatalf("doctor output missing header: %s", doctorOutput)
	}
	if !strings.Contains(doctorOutput, "healthy") {
		t.Fatalf("doctor output missing healthy status: %s", doctorOutput)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	doctorJSONOutput := captureStdout(t, func() {
		err := RunDoctor(indexPath, DoctorOptions{JSON: true})
		if err != nil {
			t.Fatalf("RunDoctor(json) error = %v", err)
		}
	})
	if !strings.Contains(doctorJSONOutput, `"healthy": false`) {
		t.Fatalf("doctor json output missing unhealthy state: %s", doctorJSONOutput)
	}
	if !strings.Contains(doctorJSONOutput, `"missing_files"`) {
		t.Fatalf("doctor json output missing missing_files: %s", doctorJSONOutput)
	}
}

func TestRunIndexWithConfig(t *testing.T) {
	root := t.TempDir()
	indexPath := filepath.Join(root, "index.sqlite")
	configPath := filepath.Join(root, ".sagasu.json")

	mustWriteAppFile(t, filepath.Join(root, "cmd", "main.go"), "package main\nfunc main() { println(\"hello\") }\n")
	mustWriteAppFile(t, filepath.Join(root, "internal", "service.go"), "package internal\nfunc service() { println(\"hello\") }\n")
	mustWriteAppFile(t, filepath.Join(root, "internal", "service_test.go"), "package internal\n")
	mustWriteAppFile(t, configPath, "{\n  \"include\": [\"internal/**/*.go\"],\n  \"exclude\": [\"**/*_test.go\"]\n}\n")

	indexJSONOutput := captureStdout(t, func() {
		err := RunIndex([]string{root}, indexPath, IndexOptions{ConfigPath: configPath, JSON: true})
		if err != nil {
			t.Fatalf("RunIndex(config) error = %v", err)
		}
	})
	if !strings.Contains(indexJSONOutput, `"scanned"`) {
		t.Fatalf("index json output missing summary fields: %s", indexJSONOutput)
	}

	countOutput := captureStdout(t, func() {
		err := RunSearch("service", indexPath, SearchOptions{Count: true})
		if err != nil {
			t.Fatalf("RunSearch(service) error = %v", err)
		}
	})
	if strings.TrimSpace(countOutput) != "1" {
		t.Fatalf("service count output = %q, want %q", countOutput, "1")
	}

	testCountOutput := captureStdout(t, func() {
		err := RunSearch("package", indexPath, SearchOptions{FilesOnly: true})
		if err != nil {
			t.Fatalf("RunSearch(files-only) error = %v", err)
		}
	})
	if strings.Contains(testCountOutput, "service_test.go") {
		t.Fatalf("excluded file appeared in search output: %s", testCountOutput)
	}
}

func TestRunIndexAndSearchWithSemantic(t *testing.T) {
	root := t.TempDir()
	indexPath := filepath.Join(root, "index.sqlite")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		embeddings := make([][]float32, 0, len(payload.Input))
		for i := range payload.Input {
			if i%2 == 0 {
				embeddings = append(embeddings, []float32{1, 0})
			} else {
				embeddings = append(embeddings, []float32{0, 1})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embeddings": embeddings,
		})
	}))
	defer server.Close()

	mustWriteAppFile(t, filepath.Join(root, "main.go"), "package main\nfunc main() { hello lexical }\n")
	mustWriteAppFile(t, filepath.Join(root, "note.md"), "semantic note\n")

	if err := RunIndex([]string{root}, indexPath, IndexOptions{
		EnableSemantic: true,
		OllamaURL:      server.URL,
		EmbeddingModel: "test-model",
	}); err != nil {
		t.Fatalf("RunIndex(semantic) error = %v", err)
	}

	jsonOutput := captureStdout(t, func() {
		err := RunSearch("hello", indexPath, SearchOptions{
			JSON:           true,
			EnableSemantic: true,
			OllamaURL:      server.URL,
			EmbeddingModel: "test-model",
		})
		if err != nil {
			t.Fatalf("RunSearch(semantic) error = %v", err)
		}
	})
	if !strings.Contains(jsonOutput, `"semantic_score"`) {
		t.Fatalf("json output missing semantic_score: %s", jsonOutput)
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
