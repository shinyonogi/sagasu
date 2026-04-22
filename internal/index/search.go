package index

import (
	"context"
	"fmt"
	"github.com/shinyonogi/sagasu/internal/embedding"
	"math"
	"sort"
)

type SearchOptions struct {
	ExtFilters     []string
	Limit          int
	EnableSemantic bool
	EmbeddingModel string
	SemanticWeight float64
}

type Searcher interface {
	Search(query string, options SearchOptions) ([]SearchResult, error)
}

type SemanticSearcher interface {
	SearchSemantic(query string, options SearchOptions) ([]SearchResult, error)
}

type LexicalSearcher struct {
	IndexPath string
}

func (s LexicalSearcher) Search(query string, options SearchOptions) ([]SearchResult, error) {
	return SearchStored(s.IndexPath, query, options.ExtFilters, options.Limit)
}

type NoopSemanticSearcher struct{}

func (NoopSemanticSearcher) SearchSemantic(string, SearchOptions) ([]SearchResult, error) {
	return nil, nil
}

type HybridSearcher struct {
	Lexical  Searcher
	Semantic SemanticSearcher
}

func NewHybridSearcher(indexPath string, provider embedding.Provider) HybridSearcher {
	return HybridSearcher{
		Lexical: LexicalSearcher{IndexPath: indexPath},
		Semantic: SemanticIndexSearcher{
			IndexPath: indexPath,
			Provider:  provider,
		},
	}
}

func (s HybridSearcher) Search(query string, options SearchOptions) ([]SearchResult, error) {
	if s.Lexical == nil {
		return nil, fmt.Errorf("lexical searcher is required")
	}

	lexicalResults, err := s.Lexical.Search(query, options)
	if err != nil {
		return nil, err
	}

	if !options.EnableSemantic || s.Semantic == nil {
		return lexicalResults, nil
	}

	semanticResults, err := s.Semantic.SearchSemantic(query, options)
	if err != nil {
		return nil, err
	}

	return mergeSearchResults(lexicalResults, semanticResults, options.Limit, options.SemanticWeight), nil
}

type SemanticIndexSearcher struct {
	IndexPath string
	Provider  embedding.Provider
}

func (s SemanticIndexSearcher) SearchSemantic(query string, options SearchOptions) ([]SearchResult, error) {
	if s.Provider == nil {
		return nil, nil
	}

	embeddings, err := LoadEmbeddings(s.IndexPath, options.EmbeddingModel)
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, nil
	}

	vectors := make([][]float32, 0, len(embeddings))
	keys := make([]string, 0, len(embeddings))
	for _, item := range embeddings {
		vector, err := DecodeFloat32Vector(item.Vector)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, vector)
		keys = append(keys, item.ChunkKey)
	}

	queryEmbeddings, err := s.Provider.Embed(context.Background(), []string{query})
	if err != nil {
		return nil, err
	}
	if len(queryEmbeddings) == 0 {
		return nil, nil
	}

	type semanticCandidate struct {
		key   string
		score float64
	}

	candidates := make([]semanticCandidate, 0, len(vectors))
	for i, vector := range vectors {
		score := cosineSimilarity(queryEmbeddings[0], vector)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, semanticCandidate{
			key:   keys[i],
			score: score,
		})
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].key < candidates[j].key
		}
		return candidates[i].score > candidates[j].score
	})

	candidateLimit := options.Limit
	if candidateLimit <= 0 {
		candidateLimit = 20
	}
	if boosted := options.Limit * 5; boosted > candidateLimit {
		candidateLimit = boosted
	}
	if candidateLimit > len(candidates) {
		candidateLimit = len(candidates)
	}
	candidates = candidates[:candidateLimit]

	chunkKeys := make([]string, 0, len(candidates))
	scoreByKey := make(map[string]float64, len(candidates))
	for _, candidate := range candidates {
		chunkKeys = append(chunkKeys, candidate.key)
		scoreByKey[candidate.key] = normalizeSemanticScore(candidate.score)
	}

	results, err := LoadSearchResultsByChunkKeys(s.IndexPath, chunkKeys, options.ExtFilters)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].SemanticScore = scoreByKey[results[i].Chunk.Key]
		results[i].Score = combineScores(results[i], options.SemanticWeight)
	}

	return results, nil
}

func mergeSearchResults(lexicalResults []SearchResult, semanticResults []SearchResult, limit int, semanticWeight float64) []SearchResult {
	merged := make(map[string]SearchResult, len(lexicalResults)+len(semanticResults))

	for _, result := range lexicalResults {
		merged[result.Chunk.Key] = result
	}

	for _, semantic := range semanticResults {
		existing, ok := merged[semantic.Chunk.Key]
		if !ok {
			semantic.Score = combineScores(semantic, semanticWeight)
			merged[semantic.Chunk.Key] = semantic
			continue
		}

		existing.SemanticScore = semantic.SemanticScore
		existing.Score = combineScores(existing, semanticWeight)
		merged[semantic.Chunk.Key] = existing
	}

	results := make([]SearchResult, 0, len(merged))
	for _, result := range merged {
		results = append(results, result)
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			if results[i].Document.Path == results[j].Document.Path {
				return results[i].Chunk.LineNumber < results[j].Chunk.LineNumber
			}
			return results[i].Document.Path < results[j].Document.Path
		}
		return results[i].Score > results[j].Score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

func combineScores(result SearchResult, semanticWeight float64) float64 {
	if semanticWeight == 0 {
		semanticWeight = 2.0
	}
	return result.LexicalScore + result.CoverageScore + result.PathScore + result.ExactScore + result.SemanticScore*semanticWeight
}

func normalizeSemanticScore(score float64) float64 {
	return (score + 1) / 2
}

func cosineSimilarity(left []float32, right []float32) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}

	var dot float64
	var leftNorm float64
	var rightNorm float64
	for i := range left {
		l := float64(left[i])
		r := float64(right[i])
		dot += l * r
		leftNorm += l * l
		rightNorm += r * r
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}
