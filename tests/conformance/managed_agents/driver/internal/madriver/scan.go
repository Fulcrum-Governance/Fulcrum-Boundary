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
//
// The walk is scoped to an os.Root: every file is opened through the root
// handle, so a path swapped in mid-walk (or a symlink escaping dir) cannot
// redirect a read outside the scan directory.
func ScanDirForSecrets(dir string) (hitCount int, hitPaths []string, err error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return 0, nil, fmt.Errorf("open scan root: %w", err)
	}
	defer func() {
		if cerr := root.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()
	var hits []string
	walkErr := fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := root.ReadFile(path)
		if err != nil {
			return fmt.Errorf("scan %s: %w", path, err)
		}
		if LooksSecret(string(data)) {
			hits = append(hits, filepath.Join(dir, path))
		}
		return nil
	})
	if walkErr != nil {
		return 0, nil, walkErr
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
