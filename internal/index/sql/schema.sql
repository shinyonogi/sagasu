CREATE TABLE IF NOT EXISTS documents (
  path TEXT PRIMARY KEY,
  ext TEXT NOT NULL,
  modified INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS chunks (
  key TEXT PRIMARY KEY,
  document_path TEXT NOT NULL,
  line_number INTEGER NOT NULL,
  content TEXT NOT NULL,
  FOREIGN KEY(document_path) REFERENCES documents(path) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS postings (
  term TEXT NOT NULL,
  chunk_key TEXT NOT NULL,
  tf INTEGER NOT NULL,
  PRIMARY KEY(term, chunk_key),
  FOREIGN KEY(chunk_key) REFERENCES chunks(key) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS embeddings (
  chunk_key TEXT NOT NULL,
  model TEXT NOT NULL,
  dimensions INTEGER NOT NULL,
  vector BLOB NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY(chunk_key, model),
  FOREIGN KEY(chunk_key) REFERENCES chunks(key) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_postings_term ON postings(term);
CREATE INDEX IF NOT EXISTS idx_chunks_document_path ON chunks(document_path);
CREATE INDEX IF NOT EXISTS idx_embeddings_model ON embeddings(model);
