package crawler

import (
	"github.com/shinyonogi/sagasu/internal/file"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type FileEntry struct {
	Path     string
	Modified time.Time
}

type Options struct {
	IncludePatterns []string
	ExcludePatterns []string
	IgnoreDirs      []string
}

var ignoredDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"dist":         {},
	"build":        {},
	"vendor":       {},
}

func CollectFiles(roots []string, options Options) ([]FileEntry, error) {
	var paths []FileEntry
	ignoredDirs := buildIgnoredDirs(options.IgnoreDirs)

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				if _, ignored := ignoredDirs[d.Name()]; ignored {
					return filepath.SkipDir
				}
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if file.IsAllowed(ext) && shouldInclude(root, path, options) {
				info, err := d.Info()
				if err != nil {
					return err
				}

				paths = append(paths, FileEntry{
					Path:     path,
					Modified: info.ModTime(),
				})
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return paths, nil
}

func buildIgnoredDirs(extra []string) map[string]struct{} {
	merged := make(map[string]struct{}, len(ignoredDirs)+len(extra))
	for name := range ignoredDirs {
		merged[name] = struct{}{}
	}
	for _, name := range extra {
		if name == "" {
			continue
		}
		merged[name] = struct{}{}
	}
	return merged
}

func shouldInclude(root string, fullPath string, options Options) bool {
	relative, err := filepath.Rel(root, fullPath)
	if err != nil {
		return true
	}

	normalized := filepath.ToSlash(relative)
	if matchesAny(normalized, options.ExcludePatterns) {
		return false
	}
	if len(options.IncludePatterns) == 0 {
		return true
	}
	return matchesAny(normalized, options.IncludePatterns)
}

func matchesAny(target string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		if matchPattern(target, pattern) {
			return true
		}
	}
	return false
}

func matchPattern(target string, pattern string) bool {
	target = path.Clean(target)
	pattern = path.Clean(filepath.ToSlash(pattern))

	regex := globToRegexp(pattern)
	ok, err := regexp.MatchString(regex, target)
	if err != nil {
		return false
	}
	return ok
}

func globToRegexp(pattern string) string {
	var builder strings.Builder
	builder.WriteString("^")

	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					builder.WriteString("(?:.*/)?")
					i += 2
				} else {
					builder.WriteString(".*")
					i++
				}
			} else {
				builder.WriteString("[^/]*")
			}
		case '?':
			builder.WriteString(".")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			builder.WriteByte('\\')
			builder.WriteByte(pattern[i])
		default:
			builder.WriteByte(pattern[i])
		}
	}

	builder.WriteString("$")
	return builder.String()
}
