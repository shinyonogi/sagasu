# sagasu

`sagasu` is a local full-text search CLI for source repositories.

It indexes text-based project files into a SQLite database, then searches the stored index with a fast CLI-oriented workflow. By default, indexes are stored in a managed global cache directory so you can search a repository from anywhere.

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

## Homebrew Packaging

This repo now includes a basic GoReleaser setup for publishing Homebrew tap formulas:

- [.goreleaser.yaml](/Users/shiny/Dev/Projects/sagasu/.goreleaser.yaml:1)

The intended install flow is:

```bash
brew install shinyonogi/tap/sagasu
```

To make that real, you still need:

1. A tap repository such as `shinyonogi/homebrew-tap`
2. GitHub releases for `sagasu`
3. A release workflow or local `goreleaser release` run with a token that can push to the tap repo

This repo now includes:

- [.goreleaser.yaml](/Users/shiny/Dev/Projects/sagasu/.goreleaser.yaml:1)
- [.github/workflows/release.yml](/Users/shiny/Dev/Projects/sagasu/.github/workflows/release.yml:1)

The GitHub Actions release workflow expects this repository secret:

- `HOMEBREW_TAP_GITHUB_TOKEN`

Release flow:

1. Open GitHub Actions
2. Run the `release` workflow
3. Enter a version like `0.1.0` or `0.1.0-beta.1`

The workflow then:

1. Validates the version
2. Creates and pushes `v<version>` on the latest commit
3. Runs GoReleaser
4. Creates the GitHub Release
5. Uploads build artifacts
6. Updates the Homebrew tap formula

Release notes are generated automatically from commits since the previous tag. The current GoReleaser changelog config keeps the release notes focused on user-facing changes by excluding commits that start with:

- `docs:`
- `test:`
- `chore:`
- `Merge`

It also groups Conventional Commits into sections such as:

- `Features` for `feat:`
- `Bug Fixes` for `fix:`
- `Performance` for `perf:`
- `Refactors` for `refactor:`

The configuration follows GoReleaser's Homebrew tap support. GoReleaser's own docs note that its generated Homebrew formulas are meant for third-party taps rather than `homebrew/core`: [GoReleaser Homebrew Formulas](https://goreleaser.com/customization/homebrew_formulas/), [Homebrew Taps](https://docs.brew.sh/Taps)

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
~/.cache/sagasu/indexes/<name>-<hash>.sqlite
```

You can override that with:

```bash
go run ./cmd/sagasu index . --index-path /tmp/sagasu.sqlite
```

JSON summary output:

```bash
go run ./cmd/sagasu index . --json
```

If you want to search that repository later from another directory, pass its root:

```bash
sagasu search hello --root /path/to/repo
```

Rebuild the index from scratch:

```bash
go run ./cmd/sagasu rebuild .
```

### Search

Basic search:

```bash
go run ./cmd/sagasu search sqlc
sagasu search sqlc --root /path/to/repo
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

The same config path can be used with `rebuild`:

```bash
go run ./cmd/sagasu rebuild . --config /path/to/sagasu.json
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

The release pipeline also packages bash, zsh, and fish completions so they can be installed through the Homebrew formula.

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
