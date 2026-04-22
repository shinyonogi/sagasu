package app

import (
	"encoding/json"
	"fmt"
	"github.com/shinyonogi/sagasu/internal/index"
	"github.com/shinyonogi/sagasu/internal/output"
	"os"
)

type SearchOptions struct {
	ExtFilters []string
	Limit      int
	JSON       bool
	Count      bool
	Context    int
}

type SearchOutput struct {
	Query   string               `json:"query"`
	Exts    []string             `json:"exts,omitempty"`
	Count   int                  `json:"count"`
	Results []index.SearchResult `json:"results"`
}

func RunSearch(query string, indexPath string, options SearchOptions) error {
	if options.JSON && options.Count {
		return fmt.Errorf("--json and --count cannot be used together")
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
