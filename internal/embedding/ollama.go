package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

type ollamaErrorResponse struct {
	Error string `json:"error"`
}

func NewProvider(config Config) (Provider, error) {
	provider := strings.TrimSpace(strings.ToLower(config.Provider))
	if provider == "" {
		provider = DefaultProvider
	}

	switch provider {
	case "ollama":
		model := strings.TrimSpace(config.Model)
		if model == "" {
			model = DefaultModel
		}
		baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
		if baseURL == "" {
			baseURL = DefaultOllama
		}
		return OllamaProvider{
			baseURL: baseURL,
			model:   model,
			client: &http.Client{
				Timeout: 60 * time.Second,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", provider)
	}
}

func (p OllamaProvider) Embed(ctx context.Context, input []string) ([][]float32, error) {
	filtered := make([]string, 0, len(input))
	for _, item := range input {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		return nil, nil
	}

	payload, err := json.Marshal(ollamaEmbedRequest{
		Model: p.model,
		Input: filtered,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ollama embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/embed", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create ollama embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request ollama embeddings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr ollamaErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
			return nil, fmt.Errorf("ollama embeddings failed: %s", apiErr.Error)
		}
		return nil, fmt.Errorf("ollama embeddings failed with status %s", resp.Status)
	}

	var decoded ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode ollama embeddings: %w", err)
	}
	if len(decoded.Embeddings) != len(filtered) {
		return nil, fmt.Errorf("ollama embeddings returned %d vectors for %d inputs", len(decoded.Embeddings), len(filtered))
	}

	return decoded.Embeddings, nil
}
