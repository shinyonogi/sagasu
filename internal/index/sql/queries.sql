-- name: ClearPostings :exec
DELETE FROM postings;

-- name: ClearChunks :exec
DELETE FROM chunks;

-- name: ClearDocuments :exec
DELETE FROM documents;

-- name: DeleteDocument :exec
DELETE FROM documents
WHERE path = ?;

-- name: CreateDocument :exec
INSERT INTO documents(path, ext, modified)
VALUES (?, ?, ?);

-- name: CreateChunk :exec
INSERT INTO chunks(key, document_path, line_number, content)
VALUES (?, ?, ?, ?);

-- name: CreatePosting :exec
INSERT INTO postings(term, chunk_key, tf)
VALUES (?, ?, ?);

-- name: ListDocuments :many
SELECT path, ext, modified
FROM documents;

-- name: ListChunks :many
SELECT key, document_path, line_number, content
FROM chunks;

-- name: ListPostings :many
SELECT term, chunk_key, tf
FROM postings;

-- name: SearchByTerms :many
SELECT
  c.key,
  c.document_path,
  c.line_number,
  c.content,
  d.path,
  d.ext,
  d.modified,
  SUM(p.tf) AS score
FROM postings p
JOIN chunks c ON c.key = p.chunk_key
JOIN documents d ON d.path = c.document_path
WHERE p.term IN (sqlc.slice('terms'))
GROUP BY c.key, c.document_path, c.line_number, c.content, d.path, d.ext, d.modified
ORDER BY score DESC, d.path ASC, c.line_number ASC
LIMIT ?;

-- name: SearchByTermsAndExts :many
SELECT
  c.key,
  c.document_path,
  c.line_number,
  c.content,
  d.path,
  d.ext,
  d.modified,
  SUM(p.tf) AS score
FROM postings p
JOIN chunks c ON c.key = p.chunk_key
JOIN documents d ON d.path = c.document_path
WHERE p.term IN (sqlc.slice('terms'))
  AND d.ext IN (sqlc.slice('exts'))
GROUP BY c.key, c.document_path, c.line_number, c.content, d.path, d.ext, d.modified
ORDER BY score DESC, d.path ASC, c.line_number ASC
LIMIT ?;
