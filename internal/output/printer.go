package output

import (
	"encoding/json"
	"fmt"
	"github.com/shinyonogi/sagasu/internal/index"
	"github.com/shinyonogi/sagasu/internal/tokenizer"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorCyan   = "\033[36m"
	colorBlue   = "\033[94m"
	colorGreen  = "\033[92m"
	colorYellow = "\033[93m"
)

type Printer struct {
	color bool
}

type fileSnippetCache map[string][]string

type IndexSummary struct {
	IndexPath string `json:"path"`
	Scanned   int    `json:"scanned"`
	Changed   int    `json:"changed"`
	Skipped   int    `json:"skipped"`
	Deleted   int    `json:"deleted"`
	Chunks    int    `json:"chunks"`
	Terms     int    `json:"terms"`
}

func NewPrinter() Printer {
	return Printer{color: stdoutSupportsColor()}
}

func (p Printer) PrintJSON(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

func (p Printer) PrintIndexSummary(summary IndexSummary) {
	fmt.Println(p.title("INDEX"))
	fmt.Printf("%s%s\n", p.label("  database"), p.value(summary.IndexPath))
	fmt.Printf("%s%s\n", p.label("  scanned "), p.metric(summary.Scanned, "files"))
	fmt.Printf("%s%s\n", p.label("  changed "), p.metric(summary.Changed, "files"))
	fmt.Printf("%s%s\n", p.label("  skipped "), p.metric(summary.Skipped, "files"))
	fmt.Printf("%s%s\n", p.label("  deleted "), p.metric(summary.Deleted, "files"))
	fmt.Printf("%s%s\n", p.label("  chunks  "), p.metric(summary.Chunks, "indexed"))
	fmt.Printf("%s%s\n", p.label("  terms   "), p.metric(summary.Terms, "indexed"))
}

func (p Printer) PrintIndexStats(stats index.IndexStats) {
	fmt.Println(p.title("STATUS"))
	fmt.Printf("%s%s\n", p.label("  database"), p.value(stats.Path))
	fmt.Printf("%s%s\n", p.label("  size    "), p.value(formatBytes(stats.SizeBytes)))
	fmt.Printf("%s%s\n", p.label("  docs    "), p.metric(stats.Documents, "documents"))
	fmt.Printf("%s%s\n", p.label("  chunks  "), p.metric(stats.Chunks, "chunks"))
	fmt.Printf("%s%s\n", p.label("  terms   "), p.metric(stats.Terms, "postings"))
	if stats.LastModified > 0 {
		fmt.Printf("%s%s\n", p.label("  updated "), p.value(time.Unix(stats.LastModified, 0).Local().Format(time.RFC3339)))
	}
	if len(stats.Exts) == 0 {
		return
	}

	fmt.Printf("%s%s\n", p.label("  exts    "), p.muted(""))
	for _, ext := range stats.Exts {
		fmt.Printf("    %s %s\n", p.value("."+ext.Ext), p.muted(fmt.Sprintf("(%d)", ext.Count)))
	}
}

func (p Printer) PrintDoctorReport(report index.DoctorReport) {
	fmt.Println(p.title("DOCTOR"))
	fmt.Printf("%s%s\n", p.label("  database"), p.value(report.Path))
	status := "healthy"
	if !report.Healthy {
		status = "needs attention"
	}
	fmt.Printf("%s%s\n", p.label("  status  "), p.value(status))
	fmt.Printf("%s%s\n", p.label("  docs    "), p.metric(report.Documents, "documents"))
	fmt.Printf("%s%s\n", p.label("  missing "), p.metric(len(report.MissingFiles), "files"))
	fmt.Printf("%s%s\n", p.label("  stale   "), p.metric(len(report.StaleFiles), "files"))
	fmt.Printf("%s%s\n", p.label("  unread  "), p.metric(len(report.UnreadableFiles), "files"))

	if len(report.Problems) == 0 {
		fmt.Printf("%s\n", p.muted("  no problems detected"))
		return
	}

	fmt.Printf("%s\n", p.label("  issues"))
	for _, problem := range report.Problems {
		fmt.Printf("    %s\n", problem)
	}
}

func (p Printer) PrintSearchResults(query string, extFilters []string, contextLines int, results []index.SearchResult) {
	fmt.Println(p.title("SEARCH"))
	fmt.Printf("%s%s\n", p.label("  query   "), p.value(query))
	if len(extFilters) > 0 {
		fmt.Printf("%s%s\n", p.label("  ext     "), p.value(strings.Join(normalizeExts(extFilters), ", ")))
	}
	if contextLines > 0 {
		fmt.Printf("%s%s\n", p.label("  context "), p.metric(contextLines, "lines"))
	}
	fmt.Printf("%s%s\n", p.label("  results "), p.metric(len(results), "hits"))

	if len(results) == 0 {
		fmt.Printf("%s\n", p.muted("  no results"))
		return
	}

	tokens := tokenizer.Tokenize(query)
	cache := fileSnippetCache{}
	for i, result := range results {
		fmt.Println()
		fmt.Printf(
			"%s %s:%d %s%s%s\n",
			p.rank(i+1),
			p.path(result.Document.Path),
			result.Chunk.LineNumber+1,
			p.badge("score"),
			p.score(result.Score),
			colorResetIf(p.color),
		)
		p.printMatchBody(cache, result, contextLines, tokens)
	}
}

func (p Printer) title(text string) string {
	return p.styled(colorBold+colorBlue, text)
}

func (p Printer) label(text string) string {
	return p.styled(colorDim, fmt.Sprintf("%-11s", text))
}

func (p Printer) value(text string) string {
	return p.styled(colorBold, text)
}

func (p Printer) muted(text string) string {
	return p.styled(colorDim, text)
}

func (p Printer) metric(value int, unit string) string {
	return fmt.Sprintf("%s %s", p.styled(colorBold+colorCyan, fmt.Sprintf("%d", value)), p.muted(unit))
}

func (p Printer) path(text string) string {
	return p.styled(colorBold+colorCyan, text)
}

func (p Printer) badge(text string) string {
	return p.styled(colorDim, "["+text+"=")
}

func (p Printer) score(value int) string {
	return p.styled(colorGreen, fmt.Sprintf("%d]", value))
}

func (p Printer) rank(value int) string {
	return p.styled(colorYellow, fmt.Sprintf("%2d.", value))
}

func (p Printer) printMatchBody(cache fileSnippetCache, result index.SearchResult, contextLines int, tokens []string) {
	if contextLines <= 0 {
		fmt.Printf("    %s\n", p.highlight(result.Chunk.Content, tokens))
		return
	}

	lines, ok := cache[result.Document.Path]
	if !ok {
		loaded, err := readFileLines(result.Document.Path)
		if err != nil {
			fmt.Printf("    %s\n", p.highlight(result.Chunk.Content, tokens))
			return
		}
		lines = loaded
		cache[result.Document.Path] = lines
	}

	start := result.Chunk.LineNumber - contextLines
	if start < 0 {
		start = 0
	}
	end := result.Chunk.LineNumber + contextLines
	if end >= len(lines) {
		end = len(lines) - 1
	}

	width := len(fmt.Sprintf("%d", end+1))
	for lineNumber := start; lineNumber <= end; lineNumber++ {
		prefix := " "
		if lineNumber == result.Chunk.LineNumber {
			prefix = ">"
		}
		content := lines[lineNumber]
		if lineNumber == result.Chunk.LineNumber {
			content = p.highlight(content, tokens)
		}
		fmt.Printf(
			"  %s %s | %s\n",
			p.muted(prefix),
			p.lineNumber(lineNumber+1, width),
			content,
		)
	}
}

func (p Printer) highlight(content string, tokens []string) string {
	if !p.color || len(tokens) == 0 {
		return content
	}

	sorted := uniqueTokens(tokens)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})

	lower := strings.ToLower(content)
	var builder strings.Builder
	cursor := 0

	for cursor < len(content) {
		matchLen := 0
		for _, token := range sorted {
			if len(token) == 0 {
				continue
			}
			if strings.HasPrefix(lower[cursor:], token) {
				matchLen = len(token)
				break
			}
		}

		if matchLen == 0 {
			builder.WriteByte(content[cursor])
			cursor++
			continue
		}

		builder.WriteString(colorBold + colorYellow)
		builder.WriteString(content[cursor : cursor+matchLen])
		builder.WriteString(colorReset)
		cursor += matchLen
	}

	return builder.String()
}

func (p Printer) styled(prefix string, text string) string {
	if !p.color || text == "" {
		return text
	}
	return prefix + text + colorReset
}

func (p Printer) lineNumber(value int, width int) string {
	return p.styled(colorDim, fmt.Sprintf("%*d", width, value))
}

func normalizeExts(extFilters []string) []string {
	seen := map[string]struct{}{}
	exts := make([]string, 0, len(extFilters))
	for _, ext := range extFilters {
		normalized := strings.TrimPrefix(strings.ToLower(ext), ".")
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		exts = append(exts, normalized)
	}
	sort.Strings(exts)
	return exts
}

func uniqueTokens(tokens []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		unique = append(unique, token)
	}
	return unique
}

func stdoutSupportsColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "" || os.Getenv("TERM") == "dumb" {
		return false
	}

	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeCharDevice) != 0
}

func readFileLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return []string{""}, nil
	}

	return strings.Split(text, "\n"), nil
}

func colorResetIf(enabled bool) string {
	if enabled {
		return colorReset
	}
	return ""
}

func formatBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(size)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}
