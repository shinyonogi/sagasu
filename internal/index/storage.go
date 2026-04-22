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
	"sort"
	"strings"
)

const sqliteDriverName = "sqlite"
const ftsSchemaSQL = `
CREATE VIRTUAL TABLE IF NOT EXISTS fts_chunks USING fts5(
  key UNINDEXED,
  document_path UNINDEXED,
  content
);
`

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

	ctx := context.Background()
	queries := indexdb.New(db)
	normalizedExts := normalizeExtSlice(extFilters)

	if limit <= 0 {
		limit = 20
	}

	if parsed.IsPhrase {
		return searchPhrase(db, parsed.Phrase, normalizedExts, limit)
	}

	if len(normalizedExts) > 0 {
		rows, err := queries.SearchByTermsAndExts(ctx, indexdb.SearchByTermsAndExtsParams{
			Terms: parsed.Tokens,
			Exts:  normalizedExts,
			Limit: int64(limit),
		})
		if err != nil {
			return nil, fmt.Errorf("search stored index: %w", err)
		}
		return mapSearchByTermsAndExtsRows(rows), nil
	}

	rows, err := queries.SearchByTerms(ctx, indexdb.SearchByTermsParams{
		Terms: parsed.Tokens,
		Limit: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search stored index: %w", err)
	}

	return mapSearchByTermsRows(rows), nil
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
			if err := deleteFTSByDocumentPath(ctx, dbtx, documentPath); err != nil {
				return err
			}
			if err := queries.DeleteDocument(ctx, documentPath); err != nil {
				return fmt.Errorf("delete document %q: %w", documentPath, err)
			}
		}

		for _, document := range changed.Documents {
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
	if _, err := db.Exec(ftsSchemaSQL); err != nil {
		return fmt.Errorf("migrate fts schema: %w", err)
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

func searchPhrase(db *sql.DB, phrase string, normalizedExts []string, limit int) ([]SearchResult, error) {
	args := []any{`"` + strings.ReplaceAll(phrase, `"`, `""`) + `"`}
	query := `
SELECT
  c.key,
  c.document_path,
  c.line_number,
  c.content,
  d.path,
  d.ext,
  d.modified,
  1 AS score
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
ORDER BY d.path ASC, c.line_number ASC
LIMIT ?`
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search phrase: %w", err)
	}
	defer rows.Close()

	results := make([]SearchResult, 0)
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
			&result.Score,
		); err != nil {
			return nil, fmt.Errorf("scan phrase result: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate phrase results: %w", err)
	}

	return results, nil
}

func placeholderList(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func mapSearchByTermsRows(rows []indexdb.SearchByTermsRow) []SearchResult {
	results := make([]SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, SearchResult{
			Chunk: Chunk{
				Key:          row.Key,
				DocumentPath: row.DocumentPath,
				LineNumber:   int(row.LineNumber),
				Content:      row.Content,
			},
			Document: Document{
				Path:     row.Path,
				Ext:      row.Ext,
				Modified: row.Modified,
			},
			Score: int(row.Score.Float64),
		})
	}
	return results
}

func mapSearchByTermsAndExtsRows(rows []indexdb.SearchByTermsAndExtsRow) []SearchResult {
	results := make([]SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, SearchResult{
			Chunk: Chunk{
				Key:          row.Key,
				DocumentPath: row.DocumentPath,
				LineNumber:   int(row.LineNumber),
				Content:      row.Content,
			},
			Document: Document{
				Path:     row.Path,
				Ext:      row.Ext,
				Modified: row.Modified,
			},
			Score: int(row.Score.Float64),
		})
	}
	return results
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
	_, err := dbtx.ExecContext(ctx, `INSERT INTO fts_chunks(key, document_path, content) VALUES (?, ?, ?)`, chunk.Key, chunk.DocumentPath, chunk.Content)
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
