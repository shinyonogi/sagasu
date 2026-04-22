package index

type Document struct {
	Path     string `json:"path"`
	Ext      string `json:"ext"`
	Modified int64  `json:"modified"`
}

type Chunk struct {
	Key          string `json:"key"`
	DocumentPath string `json:"document_path"`
	LineNumber   int    `json:"line_number"`
	Content      string `json:"content"`
}

type Posting struct {
	ChunkKey string `json:"chunk_key"`
	TF       int    `json:"tf"`
}

type SearchResult struct {
	Chunk    Chunk    `json:"chunk"`
	Document Document `json:"document"`
	Score    int      `json:"score"`
}

type InvertedIndex struct {
	Documents map[string]Document  `json:"documents"`
	Chunks    map[string]Chunk     `json:"chunks"`
	Terms     map[string][]Posting `json:"terms"`
}

func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		Documents: map[string]Document{},
		Chunks:    map[string]Chunk{},
		Terms:     map[string][]Posting{},
	}
}
