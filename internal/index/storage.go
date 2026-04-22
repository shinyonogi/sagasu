package index

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	indexdb "github.com/shinyonogi/sagasu/internal/index/sqlc"
	"github.com/shinyonogi/sagasu/internal/tokenizer"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const sqliteDriverName = "sqlite"
const ftsSchemaSQL = `
CREATE VIRTUAL TABLE IF NOT EXISTS fts_chunks USING fts5(
  key UNINDEXED,
  document_path,
  file_name,
  content
);
`

var expectedFTSColumns = []string{"key", "document_path", "file_name", "content"}

//go:embed sql/schema.sql
var schemaSQL string

func SearchStored(path string, query string, extFilters []string, limit int) ([]SearchResult, error) {
	parsed := parseSearchQuery(query)
	if !parsed.IsPhrase && len(parsed.Tokens) == 0 {
		return nil, nil
	}

	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		return nil, err
	}

	normalizedExts := normalizeExtSlice(extFilters)

	if limit <= 0 {
		limit = 20
	}

	return searchFTS(db, parsed, normalizedExts, limit)
}

func LoadDocuments(path string) (map[string]Document, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		return nil, err
	}

	documents, err := indexdb.New(db).ListDocuments(context.Background())
	if err != nil {
		return nil, fmt.Errorf("query documents: %w", err)
	}

	loaded := make(map[string]Document, len(documents))
	for _, document := range documents {
		loaded[document.Path] = Document{
			Path:     document.Path,
			Ext:      document.Ext,
			Modified: document.Modified,
		}
	}

	return loaded, nil
}

func LoadStats(path string) (IndexStats, error) {
	db, err := openDB(path)
	if err != nil {
		return IndexStats{}, err
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		return IndexStats{}, err
	}

	stats := IndexStats{Path: path}

	if info, err := os.Stat(path); err == nil {
		stats.SizeBytes = info.Size()
	}

	row := db.QueryRow(`
SELECT
  (SELECT COUNT(*) FROM documents),
  (SELECT COUNT(*) FROM chunks),
  (SELECT COUNT(*) FROM postings),
  COALESCE((SELECT MAX(modified) FROM documents), 0)
`)
	if err := row.Scan(&stats.Documents, &stats.Chunks, &stats.Terms, &stats.LastModified); err != nil {
		return IndexStats{}, fmt.Errorf("query index stats: %w", err)
	}

	rows, err := db.Query(`
SELECT ext, COUNT(*) AS count
FROM documents
GROUP BY ext
ORDER BY count DESC, ext ASC
`)
	if err != nil {
		return IndexStats{}, fmt.Errorf("query extension stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var extStat ExtStat
		if err := rows.Scan(&extStat.Ext, &extStat.Count); err != nil {
			return IndexStats{}, fmt.Errorf("scan extension stats: %w", err)
		}
		stats.Exts = append(stats.Exts, extStat)
	}

	if err := rows.Err(); err != nil {
		return IndexStats{}, fmt.Errorf("iterate extension stats: %w", err)
	}

	return stats, nil
}

func LoadEmbeddings(path string, model string) ([]Embedding, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		return nil, err
	}

	query := `
SELECT chunk_key, model, dimensions, vector, updated_at
FROM embeddings`
	args := []any{}
	if strings.TrimSpace(model) != "" {
		query += ` WHERE model = ?`
		args = append(args, model)
	}
	query += ` ORDER BY model ASC, chunk_key ASC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query embeddings: %w", err)
	}
	defer rows.Close()

	embeddings := make([]Embedding, 0)
	for rows.Next() {
		var embedding Embedding
		if err := rows.Scan(
			&embedding.ChunkKey,
			&embedding.Model,
			&embedding.Dimensions,
			&embedding.Vector,
			&embedding.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan embeddings: %w", err)
		}
		embeddings = append(embeddings, embedding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate embeddings: %w", err)
	}

	return embeddings, nil
}

func LoadSearchResultsByChunkKeys(path string, chunkKeys []string, extFilters []string) ([]SearchResult, error) {
	if len(chunkKeys) == 0 {
		return nil, nil
	}

	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		return nil, err
	}

	normalizedExts := normalizeExtSlice(extFilters)
	args := make([]any, 0, len(chunkKeys)+len(normalizedExts))
	query := `
SELECT
  c.key,
  c.document_path,
  c.line_number,
  c.content,
  d.path,
  d.ext,
  d.modified
FROM chunks c
JOIN documents d ON d.path = c.document_path
WHERE c.key IN (` + placeholderList(len(chunkKeys)) + `)`
	for _, key := range chunkKeys {
		args = append(args, key)
	}
	if len(normalizedExts) > 0 {
		query += ` AND d.ext IN (` + placeholderList(len(normalizedExts)) + `)`
		for _, ext := range normalizedExts {
			args = append(args, ext)
		}
	}
	query += ` ORDER BY d.path ASC, c.line_number ASC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query chunks by keys: %w", err)
	}
	defer rows.Close()

	results := make([]SearchResult, 0, len(chunkKeys))
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(
			&result.Chunk.Key,
			&result.Chunk.DocumentPath,
			&result.Chunk.LineNumber,
			&result.Chunk.Content,
			&result.Document.Path,
			&result.Document.Ext,
			&result.Document.Modified,
		); err != nil {
			return nil, fmt.Errorf("scan chunk by key: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chunks by keys: %w", err)
	}

	return results, nil
}

func SaveEmbeddings(path string, embeddings []Embedding) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		return err
	}

	ctx := context.Background()
	return transact(ctx, db, func(_ *indexdb.Queries, dbtx indexdb.DBTX) error {
		for _, embedding := range embeddings {
			if _, err := dbtx.ExecContext(
				ctx,
				`INSERT INTO embeddings(chunk_key, model, dimensions, vector, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(chunk_key, model) DO UPDATE SET
  dimensions = excluded.dimensions,
  vector = excluded.vector,
  updated_at = excluded.updated_at`,
				embedding.ChunkKey,
				embedding.Model,
				embedding.Dimensions,
				embedding.Vector,
				embedding.UpdatedAt,
			); err != nil {
				return fmt.Errorf("save embedding %q/%q: %w", embedding.ChunkKey, embedding.Model, err)
			}
		}
		return nil
	})
}

func Doctor(path string) (DoctorReport, error) {
	documents, err := LoadDocuments(path)
	if err != nil {
		return DoctorReport{}, err
	}

	report := DoctorReport{
		Path:      path,
		Healthy:   true,
		Documents: len(documents),
	}

	for docPath, document := range documents {
		info, err := os.Stat(docPath)
		if err != nil {
			if os.IsNotExist(err) {
				report.MissingFiles = append(report.MissingFiles, docPath)
				report.Problems = append(report.Problems, fmt.Sprintf("missing file: %s", docPath))
			} else {
				report.UnreadableFiles = append(report.UnreadableFiles, docPath)
				report.Problems = append(report.Problems, fmt.Sprintf("unreadable file: %s", docPath))
			}
			report.Healthy = false
			continue
		}

		if info.ModTime().Unix() != document.Modified {
			report.StaleFiles = append(report.StaleFiles, docPath)
			report.Problems = append(report.Problems, fmt.Sprintf("stale file: %s", docPath))
			report.Healthy = false
		}
	}

	sort.Strings(report.MissingFiles)
	sort.Strings(report.StaleFiles)
	sort.Strings(report.UnreadableFiles)
	sort.Strings(report.Problems)

	return report, nil
}

func ApplyChanges(path string, changed *InvertedIndex, deletedPaths []string) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		return err
	}

	ctx := context.Background()
	return transact(ctx, db, func(queries *indexdb.Queries, dbtx indexdb.DBTX) error {
		for _, documentPath := range deletedPaths {
			if err := deleteEmbeddingsByDocumentPath(ctx, dbtx, documentPath); err != nil {
				return err
			}
			if err := deleteFTSByDocumentPath(ctx, dbtx, documentPath); err != nil {
				return err
			}
			if err := queries.DeleteDocument(ctx, documentPath); err != nil {
				return fmt.Errorf("delete document %q: %w", documentPath, err)
			}
		}

		for _, document := range changed.Documents {
			if err := deleteEmbeddingsByDocumentPath(ctx, dbtx, document.Path); err != nil {
				return err
			}
			if err := deleteFTSByDocumentPath(ctx, dbtx, document.Path); err != nil {
				return err
			}
			if err := queries.DeleteDocument(ctx, document.Path); err != nil {
				return fmt.Errorf("replace document %q: %w", document.Path, err)
			}
		}

		if err := insertDocuments(ctx, queries, changed.Documents); err != nil {
			return err
		}

		if err := insertChunks(ctx, dbtx, queries, changed.Chunks); err != nil {
			return err
		}

		if err := insertTerms(ctx, queries, changed.Terms); err != nil {
			return err
		}

		return nil
	})
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		return nil, fmt.Errorf("open index database: %w", err)
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	return db, nil
}

func normalizeExtSlice(extFilters []string) []string {
	normalizedMap := normalizeExtFilters(extFilters)
	if len(normalizedMap) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(normalizedMap))
	for ext := range normalizedMap {
		normalized = append(normalized, ext)
	}

	return normalized
}

func normalizeExtFilters(extFilters []string) map[string]struct{} {
	allowedExt := map[string]struct{}{}
	for _, ext := range extFilters {
		normalized := strings.TrimPrefix(strings.ToLower(ext), ".")
		if normalized == "" {
			continue
		}
		allowedExt[normalized] = struct{}{}
	}
	return allowedExt
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("migrate index database: %w", err)
	}
	if err := migrateFTS(db); err != nil {
		return err
	}

	return nil
}

func migrateFTS(db *sql.DB) error {
	exists, err := ftsTableExists(db)
	if err != nil {
		return err
	}

	if !exists {
		if _, err := db.Exec(ftsSchemaSQL); err != nil {
			return fmt.Errorf("migrate fts schema: %w", err)
		}
		return nil
	}

	columns, err := ftsColumnNames(db)
	if err != nil {
		return err
	}
	if equalStringSlices(columns, expectedFTSColumns) {
		return nil
	}

	if _, err := db.Exec(`DROP TABLE fts_chunks`); err != nil {
		return fmt.Errorf("drop incompatible fts schema: %w", err)
	}
	if _, err := db.Exec(ftsSchemaSQL); err != nil {
		return fmt.Errorf("recreate fts schema: %w", err)
	}
	if err := rebuildFTSChunks(db); err != nil {
		return err
	}

	return nil
}

type parsedSearchQuery struct {
	IsPhrase bool
	Phrase   string
	Tokens   []string
}

func parseSearchQuery(query string) parsedSearchQuery {
	trimmed := strings.TrimSpace(query)
	if len(trimmed) >= 2 && strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
		phrase := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		return parsedSearchQuery{
			IsPhrase: phrase != "",
			Phrase:   phrase,
		}
	}

	return parsedSearchQuery{
		Tokens: tokenizer.Tokenize(query),
	}
}

func searchFTS(db *sql.DB, parsed parsedSearchQuery, normalizedExts []string, limit int) ([]SearchResult, error) {
	matchQuery, searchTerms := buildFTSMatchQuery(parsed)
	if matchQuery == "" {
		return nil, nil
	}

	candidateLimit := limit
	if candidateLimit < 100 {
		candidateLimit = 100
	}
	if boostedLimit := limit * 8; boostedLimit > candidateLimit {
		candidateLimit = boostedLimit
	}

	args := []any{matchQuery}
	query := `
SELECT
  c.key,
  c.document_path,
  c.line_number,
  c.content,
  d.path,
  d.ext,
  d.modified,
  bm25(fts_chunks, 0.0, 0.4, 1.6, 1.0) AS bm25_score
FROM fts_chunks f
JOIN chunks c ON c.key = f.key
JOIN documents d ON d.path = c.document_path
WHERE fts_chunks MATCH ?`

	if len(normalizedExts) > 0 {
		query += ` AND d.ext IN (` + placeholderList(len(normalizedExts)) + `)`
		for _, ext := range normalizedExts {
			args = append(args, ext)
		}
	}

	query += `
ORDER BY bm25_score ASC, d.path ASC, c.line_number ASC
LIMIT ?`
	args = append(args, candidateLimit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search fts: %w", err)
	}
	defer rows.Close()

	results := make([]SearchResult, 0)
	for rows.Next() {
		var result SearchResult
		var bm25Score float64
		if err := rows.Scan(
			&result.Chunk.Key,
			&result.Chunk.DocumentPath,
			&result.Chunk.LineNumber,
			&result.Chunk.Content,
			&result.Document.Path,
			&result.Document.Ext,
			&result.Document.Modified,
			&bm25Score,
		); err != nil {
			return nil, fmt.Errorf("scan fts result: %w", err)
		}
		result = rankSearchResult(result, parsed, searchTerms, bm25Score)
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fts results: %w", err)
	}

	results = filterPathOnlyDuplicates(results, searchTerms)

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			if results[i].Document.Path == results[j].Document.Path {
				return results[i].Chunk.LineNumber < results[j].Chunk.LineNumber
			}
			return results[i].Document.Path < results[j].Document.Path
		}
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func placeholderList(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func transact(ctx context.Context, db *sql.DB, fn func(*indexdb.Queries, indexdb.DBTX) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := fn(indexdb.New(db).WithTx(tx), tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	committed = true
	return nil
}

func insertDocuments(ctx context.Context, queries *indexdb.Queries, documents map[string]Document) error {
	for _, document := range documents {
		err := queries.CreateDocument(ctx, indexdb.CreateDocumentParams{
			Path:     document.Path,
			Ext:      document.Ext,
			Modified: document.Modified,
		})
		if err != nil {
			return fmt.Errorf("insert document %q: %w", document.Path, err)
		}
	}

	return nil
}

func insertChunks(ctx context.Context, dbtx indexdb.DBTX, queries *indexdb.Queries, chunks map[string]Chunk) error {
	for _, chunk := range chunks {
		err := queries.CreateChunk(ctx, indexdb.CreateChunkParams{
			Key:          chunk.Key,
			DocumentPath: chunk.DocumentPath,
			LineNumber:   int64(chunk.LineNumber),
			Content:      chunk.Content,
		})
		if err != nil {
			return fmt.Errorf("insert chunk %q: %w", chunk.Key, err)
		}
		if err := insertFTSChunk(ctx, dbtx, chunk); err != nil {
			return err
		}
	}

	return nil
}

func insertTerms(ctx context.Context, queries *indexdb.Queries, terms map[string][]Posting) error {
	for term, postings := range terms {
		for _, posting := range postings {
			err := queries.CreatePosting(ctx, indexdb.CreatePostingParams{
				Term:     term,
				ChunkKey: posting.ChunkKey,
				Tf:       int64(posting.TF),
			})
			if err != nil {
				return fmt.Errorf("insert posting %q/%q: %w", term, posting.ChunkKey, err)
			}
		}
	}

	return nil
}

func insertFTSChunk(ctx context.Context, dbtx indexdb.DBTX, chunk Chunk) error {
	_, err := dbtx.ExecContext(
		ctx,
		`INSERT INTO fts_chunks(key, document_path, file_name, content) VALUES (?, ?, ?, ?)`,
		chunk.Key,
		chunk.DocumentPath,
		filepath.Base(chunk.DocumentPath),
		chunk.Content,
	)
	if err != nil {
		return fmt.Errorf("insert fts chunk %q: %w", chunk.Key, err)
	}
	return nil
}

func deleteFTSByDocumentPath(ctx context.Context, dbtx indexdb.DBTX, documentPath string) error {
	if _, err := dbtx.ExecContext(ctx, `DELETE FROM fts_chunks WHERE document_path = ?`, documentPath); err != nil {
		return fmt.Errorf("delete fts rows for %q: %w", documentPath, err)
	}
	return nil
}

func deleteEmbeddingsByDocumentPath(ctx context.Context, dbtx indexdb.DBTX, documentPath string) error {
	if _, err := dbtx.ExecContext(ctx, `DELETE FROM embeddings WHERE chunk_key IN (SELECT key FROM chunks WHERE document_path = ?)`, documentPath); err != nil {
		return fmt.Errorf("delete embeddings for %q: %w", documentPath, err)
	}
	return nil
}

func buildFTSMatchQuery(parsed parsedSearchQuery) (string, []string) {
	if parsed.IsPhrase {
		phrase := strings.TrimSpace(parsed.Phrase)
		if phrase == "" {
			return "", nil
		}
		return `"` + strings.ReplaceAll(phrase, `"`, `""`) + `"`, tokenizer.Tokenize(phrase)
	}

	tokens := uniqueSearchTokens(parsed.Tokens)
	if len(tokens) == 0 {
		return "", nil
	}

	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		parts = append(parts, strconv.Quote(token))
	}
	return strings.Join(parts, " OR "), tokens
}

func rankSearchResult(result SearchResult, parsed parsedSearchQuery, searchTerms []string, bm25Score float64) SearchResult {
	coverage := computeCoverage(result, searchTerms)
	result.MatchedTerms = coverage.matched
	result.TotalTerms = coverage.total
	result.QueryCoverage = coverage.ratio

	baseScore := normalizeBM25(bm25Score)
	coverageBoost := coverage.ratio * 3.0
	pathBoost := computePathBoost(result, parsed, searchTerms)
	exactBoost := computeExactBoost(result, parsed)

	result.LexicalScore = baseScore
	result.CoverageScore = coverageBoost
	result.PathScore = pathBoost
	result.ExactScore = exactBoost
	result.Score = baseScore + coverageBoost + pathBoost + exactBoost
	return result
}

type coverageStats struct {
	matched int
	total   int
	ratio   float64
}

func computeCoverage(result SearchResult, searchTerms []string) coverageStats {
	terms := uniqueSearchTokens(searchTerms)
	if len(terms) == 0 {
		return coverageStats{}
	}

	searchable := tokenizer.Tokenize(result.Document.Path + " " + result.Chunk.Content)
	present := make(map[string]struct{}, len(searchable))
	for _, token := range searchable {
		present[token] = struct{}{}
	}

	matched := 0
	for _, term := range terms {
		if _, ok := present[term]; ok {
			matched++
		}
	}

	return coverageStats{
		matched: matched,
		total:   len(terms),
		ratio:   float64(matched) / float64(len(terms)),
	}
}

func computePathBoost(result SearchResult, parsed parsedSearchQuery, searchTerms []string) float64 {
	path := strings.ToLower(result.Document.Path)
	fileName := strings.ToLower(filepath.Base(result.Document.Path))
	boost := 0.0

	if parsed.IsPhrase {
		phrase := strings.ToLower(strings.TrimSpace(parsed.Phrase))
		if phrase == "" {
			return 0
		}
		if strings.Contains(fileName, phrase) {
			return 1.2
		}
		if strings.Contains(path, phrase) {
			return 0.6
		}
		return 0
	}

	for _, term := range uniqueSearchTokens(searchTerms) {
		switch {
		case strings.Contains(fileName, term):
			boost += 0.35
		case strings.Contains(path, term):
			boost += 0.15
		}
	}

	if boost > 1.2 {
		boost = 1.2
	}
	return boost
}

func computeExactBoost(result SearchResult, parsed parsedSearchQuery) float64 {
	content := strings.ToLower(result.Chunk.Content)
	path := strings.ToLower(result.Document.Path)

	if parsed.IsPhrase {
		phrase := strings.ToLower(strings.TrimSpace(parsed.Phrase))
		if phrase == "" {
			return 0
		}
		switch {
		case strings.Contains(content, phrase):
			return 1.5
		case strings.Contains(path, phrase):
			return 0.75
		default:
			return 0
		}
	}

	raw := strings.ToLower(strings.TrimSpace(strings.Join(uniqueSearchTokens(parsed.Tokens), " ")))
	if raw == "" || !strings.Contains(raw, " ") {
		return 0
	}

	switch {
	case strings.Contains(content, raw):
		return 0.75
	case strings.Contains(path, raw):
		return 0.35
	default:
		return 0
	}
}

func normalizeBM25(score float64) float64 {
	if score < 0 {
		return -score
	}
	return 1 / (1 + score)
}

func uniqueSearchTokens(tokens []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		unique = append(unique, token)
	}
	return unique
}

func filterPathOnlyDuplicates(results []SearchResult, searchTerms []string) []SearchResult {
	if len(results) == 0 {
		return results
	}

	hasContentMatch := make(map[string]bool, len(results))
	for _, result := range results {
		if countMatchedTerms(result.Chunk.Content, searchTerms) > 0 {
			hasContentMatch[result.Document.Path] = true
		}
	}

	filtered := make([]SearchResult, 0, len(results))
	keptPathOnly := map[string]struct{}{}
	for _, result := range results {
		contentMatches := countMatchedTerms(result.Chunk.Content, searchTerms)
		if contentMatches > 0 {
			filtered = append(filtered, result)
			continue
		}

		if hasContentMatch[result.Document.Path] {
			continue
		}

		if _, ok := keptPathOnly[result.Document.Path]; ok {
			continue
		}
		keptPathOnly[result.Document.Path] = struct{}{}
		filtered = append(filtered, result)
	}

	return filtered
}

func countMatchedTerms(text string, searchTerms []string) int {
	terms := uniqueSearchTokens(searchTerms)
	if len(terms) == 0 {
		return 0
	}

	searchable := tokenizer.Tokenize(text)
	present := make(map[string]struct{}, len(searchable))
	for _, token := range searchable {
		present[token] = struct{}{}
	}

	matched := 0
	for _, term := range terms {
		if _, ok := present[term]; ok {
			matched++
		}
	}
	return matched
}

func ftsTableExists(db *sql.DB) (bool, error) {
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'fts_chunks'`).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check fts table: %w", err)
	}
	return true, nil
}

func ftsColumnNames(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`PRAGMA table_info(fts_chunks)`)
	if err != nil {
		return nil, fmt.Errorf("query fts table info: %w", err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("scan fts table info: %w", err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fts table info: %w", err)
	}
	return columns, nil
}

func equalStringSlices(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func rebuildFTSChunks(db *sql.DB) error {
	rows, err := db.Query(`SELECT key, document_path, content FROM chunks`)
	if err != nil {
		return fmt.Errorf("query chunks for fts rebuild: %w", err)
	}
	defer rows.Close()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin fts rebuild transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for rows.Next() {
		var key string
		var documentPath string
		var content string
		if err := rows.Scan(&key, &documentPath, &content); err != nil {
			return fmt.Errorf("scan chunk for fts rebuild: %w", err)
		}
		if _, err := tx.Exec(
			`INSERT INTO fts_chunks(key, document_path, file_name, content) VALUES (?, ?, ?, ?)`,
			key,
			documentPath,
			filepath.Base(documentPath),
			content,
		); err != nil {
			return fmt.Errorf("rebuild fts chunk %q: %w", key, err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate chunks for fts rebuild: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit fts rebuild transaction: %w", err)
	}
	committed = true
	return nil
}
