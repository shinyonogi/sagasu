package extractor

import (
	"github.com/shinyonogi/sagasu/internal/file"
	"os"
)

type TextExtractor struct{}

func (e TextExtractor) Supports(path string) bool {
	return file.IsAllowed(path)
}

func (e TextExtractor) Extract(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
