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

type ExtStat struct {
	Ext   string `json:"ext"`
	Count int    `json:"count"`
}

type IndexStats struct {
	Path         string    `json:"path"`
	SizeBytes    int64     `json:"size_bytes"`
	Documents    int       `json:"documents"`
	Chunks       int       `json:"chunks"`
	Terms        int       `json:"terms"`
	LastModified int64     `json:"last_modified"`
	Exts         []ExtStat `json:"exts"`
}

type DoctorReport struct {
	Path            string   `json:"path"`
	Healthy         bool     `json:"healthy"`
	Documents       int      `json:"documents"`
	MissingFiles    []string `json:"missing_files"`
	StaleFiles      []string `json:"stale_files"`
	UnreadableFiles []string `json:"unreadable_files"`
	Problems        []string `json:"problems"`
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
