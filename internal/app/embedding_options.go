package app

import "github.com/shinyonogi/sagasu/internal/embedding"

type embeddingOptions struct {
	Provider string
	Model    string
	BaseURL  string
}

func normalizeEmbeddingOptions(provider string, model string, baseURL string) embeddingOptions {
	if provider == "" {
		provider = embedding.DefaultProvider
	}
	if model == "" {
		model = embedding.DefaultModel
	}
	if baseURL == "" {
		baseURL = embedding.DefaultOllama
	}
	return embeddingOptions{
		Provider: provider,
		Model:    model,
		BaseURL:  baseURL,
	}
}
