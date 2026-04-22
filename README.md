# sagasu

`sagasu` is a local full-text search CLI for source repositories.

It indexes text-based project files into a SQLite database, then searches the stored index with a fast CLI-oriented workflow. The current implementation is aimed at local code search for development projects.

## Features

- SQLite-backed local index
- Incremental reindexing based on file modification time
- Search by token with extension filters
- Context lines around matches
- Human-friendly CLI output
- `--json` and `--count` output modes
- Index status / metadata command
- Rebuild and doctor commands for recovery / diagnostics
- Config-based include / exclude filtering
- Shell completion generation
- Quoted phrase search backed by SQLite FTS

## Supported Files

The indexer currently includes these extensions:

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

The crawler skips common generated or dependency directories:

- `.git`
- `node_modules`
- `dist`
- `build`
- `vendor`

## Setup

This repo uses [`mise`](https://mise.jdx.dev/) to pin the Go version and provide common tasks.

```bash
mise install
```

Or if you want to use Go directly, the current repo version is Go `1.25.0`.

## Common Commands

Using `mise`:

```bash
mise run install
mise run test
mise run index
mise run search
mise run fmt
```

To use the CLI as `sagasu ...`, install it first:

```bash
mise run install
```

That installs the binary into `GOBIN` or `$(go env GOPATH)/bin`. If that directory is on your `PATH`, you can run:

```bash
sagasu index .
sagasu search hello
sagasu status
sagasu doctor
```

Using `go run` directly during development:

```bash
go run ./cmd/sagasu index .
go run ./cmd/sagasu search hello
go run ./cmd/sagasu status
go run ./cmd/sagasu doctor
```

## CLI Usage

### Build an index

```bash
go run ./cmd/sagasu index .
go run ./cmd/sagasu index ./cmd ./internal
```

By default the index is stored at:

```text
.sagasu-index.sqlite
```

You can override that with:

```bash
go run ./cmd/sagasu index . --index-path /tmp/sagasu.sqlite
```

JSON summary output:

```bash
go run ./cmd/sagasu index . --json
```

Rebuild the index from scratch:

```bash
go run ./cmd/sagasu rebuild .
```

### Search

Basic search:

```bash
go run ./cmd/sagasu search sqlc
```

Phrase search:

```bash
go run ./cmd/sagasu search '"hello world"'
```

Limit result count:

```bash
go run ./cmd/sagasu search sqlc --limit 5
```

Filter by extension:

```bash
go run ./cmd/sagasu search sqlc --ext go
go run ./cmd/sagasu search sqlc --ext .go --ext md
```

Show context lines:

```bash
go run ./cmd/sagasu search sqlc -C 2
go run ./cmd/sagasu search sqlc --context 3
```

Count-only output:

```bash
go run ./cmd/sagasu search sqlc --count
```

Path-only output:

```bash
go run ./cmd/sagasu search sqlc --path-only
```

Unique files with matches:

```bash
go run ./cmd/sagasu search sqlc --files-with-matches
```

JSON output:

```bash
go run ./cmd/sagasu search sqlc --json --limit 10
```

Note: `--json`, `--count`, `--path-only`, and `--files-with-matches` are mutually exclusive output modes.

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

You can override the config path:

```bash
go run ./cmd/sagasu index . --config /path/to/sagasu.json
```

### Status / Info

Show index metadata:

```bash
go run ./cmd/sagasu status
go run ./cmd/sagasu info
go run ./cmd/sagasu status --json
```

This prints:

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
go run ./cmd/sagasu doctor
go run ./cmd/sagasu doctor --json
```

`doctor` reports:

- missing files still referenced by the index
- stale files whose current mtime no longer matches the indexed value
- unreadable files

If the report is noisy or the index is clearly stale, run:

```bash
go run ./cmd/sagasu rebuild .
```

### Completion

Generate shell completion scripts:

```bash
go run ./cmd/sagasu completion bash
go run ./cmd/sagasu completion zsh
go run ./cmd/sagasu completion fish
go run ./cmd/sagasu completion powershell
```

## Development Notes

Current indexing flow:

1. Walk supported files under the target roots.
2. Skip unchanged files using saved `modified` timestamps.
3. Rebuild only changed files.
4. Delete removed files from the SQLite index.
5. Search directly from SQLite instead of loading the full index into memory.

## Testing

Run the full test suite with:

```bash
go test ./...
```

The project currently has:

- unit tests for tokenizer, filetype, chunker, crawler, builder, output helpers
- storage tests for SQLite persistence and search
- integration tests for indexing, search modes, status output, and incremental updates

## Current Limitations

- Search is token-based, not phrase-based
- Ranking is still simple term-frequency scoring
- No FTS5 yet
- No shell completion yet
- No config file for include/exclude patterns yet
- No machine-readable output for `index` yet
