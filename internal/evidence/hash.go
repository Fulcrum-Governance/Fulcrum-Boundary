package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func hashArtifact(root, relPath, kind, schema string) (Artifact, error) {
	if !safeRelPath(relPath) {
		return Artifact{}, fmt.Errorf("unsafe artifact path: %s", relPath)
	}
	path := filepath.Join(root, filepath.FromSlash(relPath))
	file, err := os.Open(path)
	if err != nil {
		return Artifact{}, err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return Artifact{}, err
	}
	return Artifact{
		Path:          filepath.ToSlash(relPath),
		Kind:          kind,
		SHA256:        "sha256:" + hex.EncodeToString(hash.Sum(nil)),
		SizeBytes:     size,
		SchemaVersion: schema,
	}, nil
}

func safeRelPath(relPath string) bool {
	clean := filepath.ToSlash(filepath.Clean(relPath))
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return false
	}
	return clean == relPath || filepath.ToSlash(filepath.Clean(clean)) == clean
}
