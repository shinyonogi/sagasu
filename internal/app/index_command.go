package app

import (
	"context"
	"github.com/shinyonogi/sagasu/internal/config"
	"github.com/shinyonogi/sagasu/internal/crawler"
	"github.com/shinyonogi/sagasu/internal/embedding"
	"github.com/shinyonogi/sagasu/internal/index"
	"github.com/shinyonogi/sagasu/internal/output"
	"runtime"
	"sort"
	"sync"
	"time"
)

type IndexOptions struct {
	ConfigPath        string
	JSON              bool
	EnableSemantic    bool
	EmbeddingProvider string
	EmbeddingModel    string
	OllamaURL         string
}

func RunIndex(roots []string, indexPath string, options IndexOptions) error {
	return runIndex(roots, indexPath, false, options)
}

func RunRebuild(roots []string, indexPath string, options IndexOptions) error {
	return runIndex(roots, indexPath, true, options)
}

func runIndex(roots []string, indexPath string, forceRebuild bool, options IndexOptions) error {
	printer := output.NewPrinter()
	cfg, err := config.Load(options.ConfigPath)
	if err != nil {
		return err
	}

	files, err := crawler.CollectFiles(roots, crawler.Options{
		IncludePatterns: cfg.Include,
		ExcludePatterns: cfg.Exclude,
		IgnoreDirs:      cfg.IgnoreDirs,
	})
	if err != nil {
		return err
	}

	existingDocuments := map[string]index.Document{}
	existingDocuments, err = index.LoadDocuments(indexPath)
	if err != nil {
		return err
	}

	changed := index.NewInvertedIndex()
	builder := index.NewBuilder()
	seen := make(map[string]struct{}, len(files))
	changedFiles := make([]crawler.FileEntry, 0, len(files))

	for _, file := range files {
		seen[file.Path] = struct{}{}

		if !forceRebuild && isUnchanged(file, existingDocuments) {
			continue
		}

		changedFiles = append(changedFiles, file)
	}

	if err := buildChangedFiles(changed, builder, changedFiles); err != nil {
		return err
	}

	var deletedPaths []string
	if forceRebuild {
		for path := range existingDocuments {
			deletedPaths = append(deletedPaths, path)
		}
	} else {
		for path := range existingDocuments {
			if _, ok := seen[path]; !ok {
				deletedPaths = append(deletedPaths, path)
			}
		}
	}

	if err := index.ApplyChanges(indexPath, changed, deletedPaths); err != nil {
		return err
	}

	if options.EnableSemantic {
		if err := indexChangedEmbeddings(indexPath, changed, options); err != nil {
			return err
		}
	}

	summary := output.IndexSummary{
		IndexPath: indexPath,
		Scanned:   len(files),
		Changed:   len(changedFiles),
		Skipped:   len(files) - len(changedFiles),
		Deleted:   len(deletedPaths),
		Chunks:    len(changed.Chunks),
		Terms:     len(changed.Terms),
	}

	if options.JSON {
		return printer.PrintJSON(summary)
	}
	printer.PrintIndexSummary(summary)

	return nil
}

func indexChangedEmbeddings(indexPath string, changed *index.InvertedIndex, options IndexOptions) error {
	if len(changed.Chunks) == 0 {
		return nil
	}

	embeddingConfig := normalizeEmbeddingOptions(options.EmbeddingProvider, options.EmbeddingModel, options.OllamaURL)
	provider, err := embedding.NewProvider(embedding.Config{
		Provider: embeddingConfig.Provider,
		Model:    embeddingConfig.Model,
		BaseURL:  embeddingConfig.BaseURL,
	})
	if err != nil {
		return err
	}

	chunks := make([]index.Chunk, 0, len(changed.Chunks))
	for _, chunk := range changed.Chunks {
		chunks = append(chunks, chunk)
	}
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Key < chunks[j].Key
	})

	existingEmbeddings, err := index.LoadEmbeddings(indexPath, embeddingConfig.Model)
	if err != nil {
		return err
	}
	existingKeys := make(map[string]struct{}, len(existingEmbeddings))
	for _, item := range existingEmbeddings {
		existingKeys[item.ChunkKey] = struct{}{}
	}

	pendingChunks := make([]index.Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		if _, ok := existingKeys[chunk.Key]; ok {
			continue
		}
		pendingChunks = append(pendingChunks, chunk)
	}
	if len(pendingChunks) == 0 {
		return nil
	}

	texts := make([]string, 0, len(pendingChunks))
	for _, chunk := range pendingChunks {
		texts = append(texts, chunk.Content)
	}

	vectors, err := provider.Embed(context.Background(), texts)
	if err != nil {
		return err
	}
	if len(vectors) != len(pendingChunks) {
		return nil
	}

	embeddingsToSave := make([]index.Embedding, 0, len(pendingChunks))
	for i, vector := range vectors {
		embeddingsToSave = append(embeddingsToSave, index.Embedding{
			ChunkKey:   pendingChunks[i].Key,
			Model:      embeddingConfig.Model,
			Dimensions: len(vector),
			Vector:     index.EncodeFloat32Vector(vector),
			UpdatedAt:  time.Now().Unix(),
		})
	}

	return index.SaveEmbeddings(indexPath, embeddingsToSave)
}

func isUnchanged(file crawler.FileEntry, existingDocuments map[string]index.Document) bool {
	document, ok := existingDocuments[file.Path]
	if !ok {
		return false
	}

	return file.Modified.Unix() == document.Modified
}

func buildChangedFiles(changed *index.InvertedIndex, builder index.Builder, files []crawler.FileEntry) error {
	if len(files) == 0 {
		return nil
	}

	workerCount := runtime.GOMAXPROCS(0)
	if workerCount > len(files) {
		workerCount = len(files)
	}

	jobs := make(chan crawler.FileEntry, len(files))
	results := make(chan *index.InvertedIndex, workerCount)
	errCh := make(chan error, 1)

	var workers sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for file := range jobs {
				idx, err := builder.BuildFile(file.Path, file.Modified)
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
				results <- idx
			}
		}()
	}

	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	go func() {
		workers.Wait()
		close(results)
	}()

	for partial := range results {
		mergeIndex(changed, partial)
	}

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func mergeIndex(dst *index.InvertedIndex, src *index.InvertedIndex) {
	for path, document := range src.Documents {
		dst.Documents[path] = document
	}

	for key, chunk := range src.Chunks {
		dst.Chunks[key] = chunk
	}

	for term, postings := range src.Terms {
		dst.Terms[term] = append(dst.Terms[term], postings...)
	}
}
