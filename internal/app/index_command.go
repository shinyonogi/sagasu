package app

import (
	"github.com/shinyonogi/sagasu/internal/crawler"
	"github.com/shinyonogi/sagasu/internal/index"
	"github.com/shinyonogi/sagasu/internal/output"
	"runtime"
	"sync"
)

func RunIndex(roots []string, indexPath string) error {
	printer := output.NewPrinter()
	files, err := crawler.CollectFiles(roots)
	if err != nil {
		return err
	}

	existingDocuments, err := index.LoadDocuments(indexPath)
	if err != nil {
		return err
	}

	changed := index.NewInvertedIndex()
	builder := index.NewBuilder()
	seen := make(map[string]struct{}, len(files))
	changedFiles := make([]crawler.FileEntry, 0, len(files))

	for _, file := range files {
		seen[file.Path] = struct{}{}

		if isUnchanged(file, existingDocuments) {
			continue
		}

		changedFiles = append(changedFiles, file)
	}

	if err := buildChangedFiles(changed, builder, changedFiles); err != nil {
		return err
	}

	var deletedPaths []string
	for path := range existingDocuments {
		if _, ok := seen[path]; !ok {
			deletedPaths = append(deletedPaths, path)
		}
	}

	if err := index.ApplyChanges(indexPath, changed, deletedPaths); err != nil {
		return err
	}

	printer.PrintIndexSummary(output.IndexSummary{
		IndexPath: indexPath,
		Scanned:   len(files),
		Changed:   len(changedFiles),
		Skipped:   len(files) - len(changedFiles),
		Deleted:   len(deletedPaths),
		Chunks:    len(changed.Chunks),
		Terms:     len(changed.Terms),
	})

	return nil
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
