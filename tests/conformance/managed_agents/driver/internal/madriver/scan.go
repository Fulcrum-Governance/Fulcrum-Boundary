package madriver

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ScanDirForSecrets walks dir and reports every file containing a
// secret-shaped substring. It returns the count of offending files; the file
// list uses paths only — matched content is never returned or logged.
func ScanDirForSecrets(dir string) (hitCount int, hitPaths []string, err error) {
	var hits []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return fmt.Errorf("scan %s: %w", path, err)
		}
		if LooksSecret(string(data)) {
			hits = append(hits, path)
		}
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	return len(hits), hits, nil
}

// scanReport renders a non-secret summary for the run log.
func scanReport(hits []string) string {
	var b strings.Builder
	for _, h := range hits {
		fmt.Fprintf(&b, "  secret-like content in %s\n", h)
	}
	return b.String()
}
