package app

import (
	"encoding/json"
	"fmt"
	"github.com/shinyonogi/sagasu/internal/index"
	"github.com/shinyonogi/sagasu/internal/output"
	"os"
	"sort"
)

type SearchOptions struct {
	ExtFilters []string
	Limit      int
	JSON       bool
	Count      bool
	Context    int
	PathOnly   bool
	FilesOnly  bool
}

type SearchOutput struct {
	Query   string               `json:"query"`
	Exts    []string             `json:"exts,omitempty"`
	Count   int                  `json:"count"`
	Results []index.SearchResult `json:"results"`
}

func RunSearch(query string, indexPath string, options SearchOptions) error {
	if err := validateSearchOptions(options); err != nil {
		return err
	}

	results, err := index.SearchStored(indexPath, query, options.ExtFilters, options.Limit)
	if err != nil {
		return err
	}

	if options.Count {
		fmt.Println(len(results))
		return nil
	}

	if options.JSON {
		return printSearchJSON(query, options.ExtFilters, results)
	}

	if options.PathOnly {
		printResultPaths(results)
		return nil
	}

	if options.FilesOnly {
		printUniqueFilePaths(results)
		return nil
	}

	output.NewPrinter().PrintSearchResults(query, options.ExtFilters, options.Context, results)

	return nil
}

func printSearchJSON(query string, extFilters []string, results []index.SearchResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	return encoder.Encode(SearchOutput{
		Query:   query,
		Exts:    normalizeSearchExts(extFilters),
		Count:   len(results),
		Results: results,
	})
}

func validateSearchOptions(options SearchOptions) error {
	modeCount := 0
	for _, enabled := range []bool{options.JSON, options.Count, options.PathOnly, options.FilesOnly} {
		if enabled {
			modeCount++
		}
	}
	if modeCount > 1 {
		return fmt.Errorf("--json, --count, --path-only, and --files-with-matches are mutually exclusive")
	}
	return nil
}

func printResultPaths(results []index.SearchResult) {
	for _, result := range results {
		fmt.Printf("%s:%d\n", result.Document.Path, result.Chunk.LineNumber+1)
	}
}

func printUniqueFilePaths(results []index.SearchResult) {
	seen := map[string]struct{}{}
	paths := make([]string, 0, len(results))
	for _, result := range results {
		if _, ok := seen[result.Document.Path]; ok {
			continue
		}
		seen[result.Document.Path] = struct{}{}
		paths = append(paths, result.Document.Path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fmt.Println(path)
	}
}

func normalizeSearchExts(extFilters []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(extFilters))
	for _, ext := range extFilters {
		if ext == "" {
			continue
		}
		if ext[0] == '.' {
			ext = ext[1:]
		}
		if ext == "" {
			continue
		}
		if _, ok := seen[ext]; ok {
			continue
		}
		seen[ext] = struct{}{}
		normalized = append(normalized, ext)
	}
	return normalized
}
