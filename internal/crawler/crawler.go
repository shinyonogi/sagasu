package crawler

import (
	"github.com/shinyonogi/sagasu/internal/file"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

type FileEntry struct {
	Path     string
	Modified time.Time
}

var ignoredDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"dist":         {},
	"build":        {},
	"vendor":       {},
}

func CollectFiles(roots []string) ([]FileEntry, error) {
	var paths []FileEntry

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
			if file.IsAllowed(ext) {
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
