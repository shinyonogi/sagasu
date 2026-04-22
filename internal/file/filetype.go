package file

import (
	"path/filepath"
	"strings"
)

var allowedExtensions = map[string]struct{}{
	".txt":   {},
	".md":    {},
	".go":    {},
	".ts":    {},
	".tsx":   {},
	".js":    {},
	".jsx":   {},
	".json":  {},
	".yaml":  {},
	".yml":   {},
	".tf":    {},
	".proto": {},
}

func NormalizeExt(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

func IsAllowed(path string) bool {
	ext := NormalizeExt(path)
	_, ok := allowedExtensions[ext]
	return ok
}
