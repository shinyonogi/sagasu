package index

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

type fakeProvider struct {
	vectors [][]float32
}

func (p fakeProvider) Embed(context.Context, []string) ([][]float32, error) {
	return p.vectors, nil
}

func TestHybridSearcherMergesSemanticScores(t *testing.T) {
	t.Parallel()

	lexical := []SearchResult{
		{
			Chunk:    Chunk{Key: "a", LineNumber: 1},
			Document: Document{Path: "/tmp/a.go"},
			Score:    2.5, LexicalScore: 2.0, CoverageScore: 0.5,
		},
	}
	semantic := []SearchResult{
		{
			Chunk:         Chunk{Key: "a", LineNumber: 1},
			Document:      Document{Path: "/tmp/a.go"},
			SemanticScore: 1.25,
		},
		{
			Chunk:         Chunk{Key: "b", LineNumber: 2},
			Document:      Document{Path: "/tmp/b.go"},
			SemanticScore: 0.8,
		},
	}

	got := mergeSearchResults(lexical, semantic, 10, 2.0)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Chunk.Key != "a" {
		t.Fatalf("got[0].Chunk.Key = %q, want %q", got[0].Chunk.Key, "a")
	}
	if got[0].SemanticScore != 1.25 {
		t.Fatalf("got[0].SemanticScore = %f, want %f", got[0].SemanticScore, 1.25)
	}
	if got[0].Score <= lexical[0].Score {
		t.Fatalf("got[0].Score = %f, want greater than %f", got[0].Score, lexical[0].Score)
	}
	if got[1].Chunk.Key != "b" {
		t.Fatalf("got[1].Chunk.Key = %q, want %q", got[1].Chunk.Key, "b")
	}
}

func TestMergeSearchResultsRespectsSemanticWeight(t *testing.T) {
	t.Parallel()

	lexical := []SearchResult{
		{
			Chunk:         Chunk{Key: "a", LineNumber: 1},
			Document:      Document{Path: "/tmp/a.go"},
			LexicalScore:  1,
			CoverageScore: 0.5,
			Score:         1.5,
		},
	}
	semantic := []SearchResult{
		{
			Chunk:         Chunk{Key: "a", LineNumber: 1},
			Document:      Document{Path: "/tmp/a.go"},
			SemanticScore: 0.75,
		},
	}

	got := mergeSearchResults(lexical, semantic, 10, 4.0)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Score != 4.5 {
		t.Fatalf("got[0].Score = %f, want 4.5", got[0].Score)
	}
}

func TestSemanticIndexSearcherSearchSemantic(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, "index.sqlite")
	goFile := filepath.Join(root, "main.go")
	mustWriteIndexFile(t, goFile, "package main\nfunc main() { hello world }\n")

	builder := NewBuilder()
	changed := NewInvertedIndex()
	if err := builder.AddFileWithModified(changed, goFile, time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatalf("AddFileWithModified() error = %v", err)
	}
	if err := ApplyChanges(dbPath, changed, nil); err != nil {
		t.Fatalf("ApplyChanges() error = %v", err)
	}

	var chunkKey string
	for key := range changed.Chunks {
		chunkKey = key
		break
	}

	if err := SaveEmbeddings(dbPath, []Embedding{
		{
			ChunkKey:   chunkKey,
			Model:      "test-model",
			Dimensions: 2,
			Vector:     EncodeFloat32Vector([]float32{1, 0}),
			UpdatedAt:  time.Now().Unix(),
		},
	}); err != nil {
		t.Fatalf("SaveEmbeddings() error = %v", err)
	}

	searcher := SemanticIndexSearcher{
		IndexPath: dbPath,
		Provider:  fakeProvider{vectors: [][]float32{{1, 0}}},
	}

	results, err := searcher.SearchSemantic("hello", SearchOptions{
		Limit:          5,
		EnableSemantic: true,
		EmbeddingModel: "test-model",
	})
	if err != nil {
		t.Fatalf("SearchSemantic() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].SemanticScore <= 0 {
		t.Fatalf("results[0].SemanticScore = %f, want > 0", results[0].SemanticScore)
	}
}
