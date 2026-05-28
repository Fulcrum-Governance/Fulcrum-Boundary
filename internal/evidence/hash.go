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
	path, err := artifactFullPath(root, relPath)
	if err != nil {
		return Artifact{}, err
	}
	// #nosec G304 -- artifactFullPath constrains relPath under the evidence bundle root before opening.
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

func artifactFullPath(root, relPath string) (string, error) {
	if !safeRelPath(relPath) {
		return "", fmt.Errorf("unsafe artifact path: %s", relPath)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve artifact root: %w", err)
	}
	path := filepath.Join(absRoot, filepath.FromSlash(relPath))
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve artifact path: %w", err)
	}
	if !samePath(absPath, absRoot) && !isWithin(absPath, absRoot) {
		return "", fmt.Errorf("artifact path escapes bundle root: %s", relPath)
	}
	return absPath, nil
}

func safeRelPath(relPath string) bool {
	clean := filepath.ToSlash(filepath.Clean(relPath))
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return false
	}
	return clean == relPath || filepath.ToSlash(filepath.Clean(clean)) == clean
}
