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
	Chunk         Chunk    `json:"chunk"`
	Document      Document `json:"document"`
	Score         float64  `json:"score"`
	LexicalScore  float64  `json:"lexical_score,omitempty"`
	SemanticScore float64  `json:"semantic_score,omitempty"`
	CoverageScore float64  `json:"coverage_score,omitempty"`
	PathScore     float64  `json:"path_score,omitempty"`
	ExactScore    float64  `json:"exact_score,omitempty"`
	MatchedTerms  int      `json:"matched_terms,omitempty"`
	TotalTerms    int      `json:"total_terms,omitempty"`
	QueryCoverage float64  `json:"query_coverage,omitempty"`
}

type Embedding struct {
	ChunkKey   string `json:"chunk_key"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
	Vector     []byte `json:"vector"`
	UpdatedAt  int64  `json:"updated_at"`
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
