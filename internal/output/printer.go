package output

import (
	"fmt"
	"github.com/shinyonogi/sagasu/internal/index"
	"github.com/shinyonogi/sagasu/internal/tokenizer"
	"os"
	"sort"
	"strings"
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

type IndexSummary struct {
	IndexPath string
	Scanned   int
	Changed   int
	Skipped   int
	Deleted   int
	Chunks    int
	Terms     int
}

func NewPrinter() Printer {
	return Printer{color: stdoutSupportsColor()}
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

func (p Printer) PrintSearchResults(query string, extFilters []string, results []index.SearchResult) {
	fmt.Println(p.title("SEARCH"))
	fmt.Printf("%s%s\n", p.label("  query   "), p.value(query))
	if len(extFilters) > 0 {
		fmt.Printf("%s%s\n", p.label("  ext     "), p.value(strings.Join(normalizeExts(extFilters), ", ")))
	}
	fmt.Printf("%s%s\n", p.label("  results "), p.metric(len(results), "hits"))

	if len(results) == 0 {
		fmt.Printf("%s\n", p.muted("  no results"))
		return
	}

	tokens := tokenizer.Tokenize(query)
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
		fmt.Printf("    %s\n", p.highlight(result.Chunk.Content, tokens))
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

func colorResetIf(enabled bool) string {
	if enabled {
		return colorReset
	}
	return ""
}
