package chunker

import "strings"

type Chunk struct {
	LineNumber int
	Content    string
}

type LineChunker struct{}

func (c LineChunker) Chunk(content string) []Chunk {
	lines := strings.Split(content, "\n")
	chunks := make([]Chunk, 0, len(lines))

	for i, line := range lines {
		if line == "" {
			continue
		}

		chunks = append(chunks, Chunk{
			LineNumber: i,
			Content:    line,
		})
	}

	return chunks
}
