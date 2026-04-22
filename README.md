# sagasu

`sagasu` is a local full-text search CLI for source repositories.

It indexes text-based project files into a SQLite database, then searches the stored index with a fast CLI-oriented workflow. By default, indexes are stored in a managed global cache directory so you can search a repository from anywhere.

## Install

### Homebrew

```bash
brew install shinyonogi/tap/sagasu
```

### Build From Source

Requires Go `1.25.0`.

```bash
git clone https://github.com/shinyonogi/sagasu.git
cd sagasu
go install ./cmd/sagasu
```

If `GOBIN` or `$(go env GOPATH)/bin` is on your `PATH`, the CLI is available as:

```bash
sagasu --help
```

## Features

- SQLite-backed local index with managed global storage
- Incremental reindexing based on file modification time
- Search by token, phrase, and extension filter
- Optional local semantic search via Ollama embeddings
- Context lines around matches
- Human-friendly CLI output
- Machine-readable JSON and count modes
- Status, rebuild, and doctor commands
- Config-based include / exclude filtering
- Shell completion generation

## Usage

### Index

Build an index for a repository:

```bash
sagasu index .
sagasu index /path/to/repo
```

By default the index is stored in a managed global cache path:

```text
~/.cache/sagasu/indexes/<name>-<hash>.sqlite
```

Override the storage path explicitly:

```bash
sagasu index /path/to/repo --index-path /tmp/sagasu.sqlite
```

Print the indexing summary as JSON:

```bash
sagasu index /path/to/repo --json
```

Rebuild the index from scratch:

```bash
sagasu rebuild /path/to/repo
```

Build semantic embeddings with local Ollama:

```bash
sagasu index /path/to/repo --semantic
```

Use a different local embedding model:

```bash
sagasu index /path/to/repo --semantic --embedding-model all-minilm
```

### Search

Search from the repository root:

```bash
sagasu search sqlc
```

Search a repository from anywhere:

```bash
sagasu search sqlc --root /path/to/repo
```

Phrase search:

```bash
sagasu search '"hello world"' --root /path/to/repo
```

Limit results:

```bash
sagasu search sqlc --limit 5
```

Filter by extension:

```bash
sagasu search sqlc --ext go
sagasu search sqlc --ext .go --ext md
```

Show context lines:

```bash
sagasu search sqlc -C 2
sagasu search sqlc --context 3
```

Output modes:

```bash
sagasu search sqlc --count
sagasu search sqlc --json
sagasu search sqlc --path-only
sagasu search sqlc --files-with-matches
```

`--json`, `--count`, `--path-only`, and `--files-with-matches` are mutually exclusive.

### Semantic Search

Semantic search is optional and uses a local Ollama server.

Start Ollama and pull an embedding model:

```bash
ollama pull embeddinggemma
```

Index the repository with semantic embeddings:

```bash
sagasu index /path/to/repo --semantic
```

Run hybrid search:

```bash
sagasu search "database connection pool" --semantic --root /path/to/repo
```

Tune semantic influence:

```bash
sagasu search "database connection pool" --semantic --semantic-weight 3.0 --root /path/to/repo
```

Use a custom Ollama endpoint:

```bash
sagasu search "storage layer" --semantic --ollama-url http://localhost:11434 --root /path/to/repo
```

### Status

Show index metadata:

```bash
sagasu status --root /path/to/repo
sagasu info --root /path/to/repo
sagasu status --root /path/to/repo --json
```

This includes:

- index file path
- file size
- document count
- chunk count
- posting count
- last indexed update time
- extension breakdown

### Doctor

Check whether the stored index is still in sync with the working tree:

```bash
sagasu doctor --root /path/to/repo
sagasu doctor --root /path/to/repo --json
```

`doctor` reports:

- missing files still referenced by the index
- stale files whose current mtime no longer matches the indexed value
- unreadable files

If the index is stale or broken:

```bash
sagasu rebuild /path/to/repo
```

### Config

`sagasu` can load include / exclude settings from a JSON config file.

By default it looks for:

```text
.sagasu.json
```

Example:

```json
{
  "include": ["internal/**/*.go", "cmd/**/*.go"],
  "exclude": ["**/*_test.go"],
  "ignore_dirs": ["tmp", "coverage"]
}
```

Use a custom config:

```bash
sagasu index /path/to/repo --config /path/to/sagasu.json
sagasu rebuild /path/to/repo --config /path/to/sagasu.json
```

### Completion

Generate shell completion scripts:

```bash
sagasu completion bash
sagasu completion zsh
sagasu completion fish
sagasu completion powershell
```

## Supported Files

The indexer currently includes:

- `.txt`
- `.md`
- `.go`
- `.ts`
- `.tsx`
- `.js`
- `.jsx`
- `.json`
- `.yaml`
- `.yml`
- `.tf`
- `.proto`

The crawler skips these directories by default:

- `.git`
- `node_modules`
- `dist`
- `build`
- `vendor`

## Limitations

- Semantic search requires a local Ollama server and a pulled embedding model
- Hybrid ranking is still heuristic and not yet benchmark-tuned
- Phrase search is available, but query language is still minimal
- No advanced boolean query syntax yet
- No machine-readable output for `doctor` exit codes yet
- Config format is JSON only
