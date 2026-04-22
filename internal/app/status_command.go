package app

import (
	"github.com/shinyonogi/sagasu/internal/index"
	"github.com/shinyonogi/sagasu/internal/output"
)

func RunStatus(indexPath string) error {
	stats, err := index.LoadStats(indexPath)
	if err != nil {
		return err
	}

	output.NewPrinter().PrintIndexStats(stats)
	return nil
}
