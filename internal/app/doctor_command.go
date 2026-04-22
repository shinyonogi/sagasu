package app

import (
	"encoding/json"
	"github.com/shinyonogi/sagasu/internal/index"
	"github.com/shinyonogi/sagasu/internal/output"
	"os"
)

type DoctorOptions struct {
	JSON bool
}

func RunDoctor(indexPath string, options DoctorOptions) error {
	report, err := index.Doctor(indexPath)
	if err != nil {
		return err
	}

	if options.JSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}

	output.NewPrinter().PrintDoctorReport(report)
	return nil
}
