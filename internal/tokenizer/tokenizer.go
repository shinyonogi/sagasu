package tokenizer

import (
	"regexp"
	"strings"
)

var splitPattern = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func Tokenize(text string) []string {
	text = strings.ToLower(text)

	parts := splitPattern.Split(text, -1)
	tokens := make([]string, 0, len(parts))

	for _, p := range parts {
		if p == "" {
			continue
		}
		tokens = append(tokens, p)
	}

	return tokens
}
