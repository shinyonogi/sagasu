package indexpath

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const appDirName = "sagasu"

func ResolveForRoots(explicit string, roots []string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if len(roots) == 0 {
		return "", fmt.Errorf("no roots provided")
	}

	normalized, err := normalizeRoots(roots)
	if err != nil {
		return "", err
	}

	return buildManagedPath(normalized)
}

func ResolveForRoot(explicit string, root string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get current directory: %w", err)
		}
		root = cwd
	}

	normalized, err := normalizeRoots([]string{root})
	if err != nil {
		return "", err
	}

	return buildManagedPath(normalized)
}

func normalizeRoots(roots []string) ([]string, error) {
	normalized := make([]string, 0, len(roots))
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("resolve absolute path for %q: %w", root, err)
		}
		normalized = append(normalized, filepath.Clean(abs))
	}
	return normalized, nil
}

func buildManagedPath(roots []string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir: %w", err)
	}

	indexDir := filepath.Join(cacheDir, appDirName, "indexes")
	if err := os.MkdirAll(indexDir, 0o755); err != nil {
		return "", fmt.Errorf("create managed index dir: %w", err)
	}

	keySource := strings.Join(roots, "\n")
	sum := sha256.Sum256([]byte(keySource))
	hash := hex.EncodeToString(sum[:])[:12]

	label := filepath.Base(roots[0])
	if len(roots) > 1 {
		label = "multi"
	}
	label = sanitizeLabel(label)

	return filepath.Join(indexDir, fmt.Sprintf("%s-%s.sqlite", label, hash)), nil
}

func sanitizeLabel(label string) string {
	if label == "" || label == "." || label == string(filepath.Separator) {
		return "root"
	}

	var builder strings.Builder
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}

	s := strings.Trim(builder.String(), "-")
	if s == "" {
		return "root"
	}
	return s
}
