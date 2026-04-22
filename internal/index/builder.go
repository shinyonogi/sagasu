package index

import (
	"fmt"
	"github.com/shinyonogi/sagasu/internal/chunker"
	"github.com/shinyonogi/sagasu/internal/extractor"
	"github.com/shinyonogi/sagasu/internal/tokenizer"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Builder struct {
	extractor extractor.TextExtractor
	chunker   chunker.LineChunker
}

func NewBuilder() Builder {
	return Builder{
		extractor: extractor.TextExtractor{},
		chunker:   chunker.LineChunker{},
	}
}

func (b Builder) AddFileWithModified(idx *InvertedIndex, path string, modified time.Time) error {
	content, err := b.extractor.Extract(path)
	if err != nil {
		return err
	}

	modifiedUnix := modified.Unix()
	if modified.IsZero() {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		modifiedUnix = info.ModTime().Unix()
	}

	document := Document{
		Path:     path,
		Ext:      strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."),
		Modified: modifiedUnix,
	}

	idx.Documents[document.Path] = document

	lineChunks := b.chunker.Chunk(content)
	for _, lc := range lineChunks {
		chunkKey := buildChunkKey(document.Path, lc.LineNumber)

		ch := Chunk{
			Key:          chunkKey,
			DocumentPath: document.Path,
			LineNumber:   lc.LineNumber,
			Content:      lc.Content,
		}

		idx.Chunks[ch.Key] = ch

		tfMap := map[string]int{}
		for _, token := range tokenizer.Tokenize(ch.Content) {
			tfMap[token]++
		}

		for term, tf := range tfMap {
			idx.Terms[term] = append(idx.Terms[term], Posting{
				ChunkKey: ch.Key,
				TF:       tf,
			})
		}
	}

	return nil
}

func (b Builder) BuildFile(path string, modified time.Time) (*InvertedIndex, error) {
	idx := NewInvertedIndex()
	if err := b.AddFileWithModified(idx, path, modified); err != nil {
		return nil, err
	}
	return idx, nil
}

func buildChunkKey(path string, lineNumber int) string {
	return fmt.Sprintf("%s:%d", path, lineNumber)
}
