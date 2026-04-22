package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaProviderEmbed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/embed" {
			t.Fatalf("path = %s, want /api/embed", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if body["model"] != "embeddinggemma" {
			t.Fatalf("model = %v, want embeddinggemma", body["model"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embeddings": [][]float32{
				{0.1, 0.2},
				{0.3, 0.4},
			},
		})
	}))
	defer server.Close()

	provider, err := NewProvider(Config{
		Provider: "ollama",
		Model:    "embeddinggemma",
		BaseURL:  server.URL,
	})
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	got, err := provider.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if len(got[0]) != 2 {
		t.Fatalf("len(got[0]) = %d, want 2", len(got[0]))
	}
}
