package firewall

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func readFileBytes(path string) ([]byte, string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return body, sha256Hex(body), nil
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func writeFileAtomic(path string, body []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".boundary-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func defaultWorkspacePath(outDir, subdir, name string) (string, error) {
	if outDir == "" {
		outDir = ".boundary/firewall"
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(absOut, subdir, name), nil
}

func cleanAbsPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}
