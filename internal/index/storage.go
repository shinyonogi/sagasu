package index

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	indexdb "github.com/shinyonogi/sagasu/internal/index/sqlc"
	"github.com/shinyonogi/sagasu/internal/tokenizer"
	_ "modernc.org/sqlite"
	"strings"
)

const sqliteDriverName = "sqlite"

//go:embed sql/schema.sql
var schemaSQL string

func SearchStored(path string, query string, extFilters []string, limit int) ([]SearchResult, error) {
	tokens := tokenizer.Tokenize(query)
	if len(tokens) == 0 {
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

	if len(normalizedExts) > 0 {
		rows, err := queries.SearchByTermsAndExts(ctx, indexdb.SearchByTermsAndExtsParams{
			Terms: tokens,
			Exts:  normalizedExts,
			Limit: int64(limit),
		})
		if err != nil {
			return nil, fmt.Errorf("search stored index: %w", err)
		}
		return mapSearchByTermsAndExtsRows(rows), nil
	}

	rows, err := queries.SearchByTerms(ctx, indexdb.SearchByTermsParams{
		Terms: tokens,
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
	return transact(ctx, db, func(queries *indexdb.Queries) error {
		for _, documentPath := range deletedPaths {
			if err := queries.DeleteDocument(ctx, documentPath); err != nil {
				return fmt.Errorf("delete document %q: %w", documentPath, err)
			}
		}

		for _, document := range changed.Documents {
			if err := queries.DeleteDocument(ctx, document.Path); err != nil {
				return fmt.Errorf("replace document %q: %w", document.Path, err)
			}
		}

		if err := insertDocuments(ctx, queries, changed.Documents); err != nil {
			return err
		}

		if err := insertChunks(ctx, queries, changed.Chunks); err != nil {
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

	return nil
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

func transact(ctx context.Context, db *sql.DB, fn func(*indexdb.Queries) error) error {
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

	if err := fn(indexdb.New(db).WithTx(tx)); err != nil {
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

func insertChunks(ctx context.Context, queries *indexdb.Queries, chunks map[string]Chunk) error {
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
