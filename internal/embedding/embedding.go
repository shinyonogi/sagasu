package embedding

import "context"

const (
	DefaultProvider = "ollama"
	DefaultModel    = "embeddinggemma"
	DefaultOllama   = "http://localhost:11434"
)

type Provider interface {
	Embed(ctx context.Context, input []string) ([][]float32, error)
}

type Config struct {
	Provider string
	Model    string
	BaseURL  string
}
