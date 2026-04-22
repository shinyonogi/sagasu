package app

import (
	"github.com/shinyonogi/sagasu/internal/index"
	"github.com/shinyonogi/sagasu/internal/output"
)

func RunSearch(query string, indexPath string, extFilters []string, limit int) error {
	results, err := index.SearchStored(indexPath, query, extFilters, limit)
	if err != nil {
		return err
	}

	output.NewPrinter().PrintSearchResults(query, extFilters, results)

	return nil
}
