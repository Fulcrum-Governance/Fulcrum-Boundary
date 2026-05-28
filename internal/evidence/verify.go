package evidence

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Verify(opts VerifyOptions) (*VerifyResult, error) {
	bundleDir := strings.TrimSpace(opts.BundleDir)
	if bundleDir == "" {
		return nil, fmt.Errorf("bundle directory is required")
	}
	absBundle, err := filepath.Abs(bundleDir)
	if err != nil {
		return nil, fmt.Errorf("resolve bundle: %w", err)
	}
	result := &VerifyResult{
		SchemaVersion: VerifySchemaVersion,
		Status:        "pass",
		Bundle:        absBundle,
	}
	addCheck := func(name, status, detail string) {
		if status != "pass" {
			result.Status = "fail"
		}
		result.Checks = append(result.Checks, VerifyCheck{Name: name, Status: status, Detail: detail})
	}

	manifestData, err := readBundleFile(absBundle, "manifest.json")
	if err != nil {
		addCheck("manifest_exists", "fail", err.Error())
		return result, nil
	}
	addCheck("manifest_exists", "pass", "manifest.json is present")

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		addCheck("manifest_parse", "fail", err.Error())
		return result, nil
	}
	result.ManifestSchema = manifest.SchemaVersion
	addCheck("manifest_parse", "pass", "manifest parses as JSON")
	if manifest.SchemaVersion != ManifestSchemaVersion {
		addCheck("manifest_schema", "fail", fmt.Sprintf("expected %s, got %s", ManifestSchemaVersion, manifest.SchemaVersion))
	} else {
		addCheck("manifest_schema", "pass", ManifestSchemaVersion)
	}

	result.ArtifactCount = len(manifest.Artifacts)
	for _, artifact := range manifest.Artifacts {
		if !safeRelPath(artifact.Path) {
			addCheck("artifact_path:"+artifact.Path, "fail", "artifact path is not a safe relative path")
			continue
		}
		absPath, err := artifactFullPath(absBundle, artifact.Path)
		if err != nil {
			addCheck("artifact_path:"+artifact.Path, "fail", err.Error())
			continue
		}
		stat, err := os.Stat(absPath)
		if err != nil {
			addCheck("artifact_exists:"+artifact.Path, "fail", err.Error())
			continue
		}
		if stat.Size() != artifact.SizeBytes {
			addCheck("artifact_size:"+artifact.Path, "fail", fmt.Sprintf("expected %d bytes, got %d", artifact.SizeBytes, stat.Size()))
		}
		computed, err := hashArtifact(absBundle, artifact.Path, artifact.Kind, artifact.SchemaVersion)
		if err != nil {
			addCheck("artifact_hash:"+artifact.Path, "fail", err.Error())
			continue
		}
		if computed.SHA256 != artifact.SHA256 {
			addCheck("artifact_hash:"+artifact.Path, "fail", fmt.Sprintf("expected %s, got %s", artifact.SHA256, computed.SHA256))
			continue
		}
		if artifact.SchemaVersion != "" {
			if err := verifyJSONSchema(absBundle, artifact.Path, artifact.SchemaVersion); err != nil {
				addCheck("artifact_schema:"+artifact.Path, "fail", err.Error())
				continue
			}
		}
		if artifact.Kind == "decision_record" {
			count, err := parseRecordArtifact(absBundle, artifact.Path)
			if err != nil {
				addCheck("record_parse:"+artifact.Path, "fail", err.Error())
				continue
			}
			result.ParsedRecords += count
		}
		result.VerifiedArtifacts++
		addCheck("artifact:"+artifact.Path, "pass", artifact.SHA256)
	}

	for _, kind := range manifest.FixtureSafeOutputs {
		if !manifestHasKind(manifest, kind) {
			addCheck("fixture_output:"+kind, "fail", "claimed fixture-safe output is missing from artifacts")
			continue
		}
		addCheck("fixture_output:"+kind, "pass", "claimed fixture-safe output is present")
	}
	switch {
	case manifest.Summary == "":
		addCheck("summary", "fail", "manifest summary path is empty")
	case !safeRelPath(manifest.Summary):
		addCheck("summary", "fail", "manifest summary path is unsafe")
	default:
		summaryData, err := readBundleFile(absBundle, manifest.Summary)
		if err != nil {
			addCheck("summary", "fail", err.Error())
		} else {
			summary := string(summaryData)
			missing := missingSummaryReferences(summary, manifest.Artifacts)
			if len(missing) > 0 {
				addCheck("summary_references", "fail", "summary missing artifact references: "+strings.Join(missing, ", "))
			} else {
				addCheck("summary_references", "pass", "summary references all manifest artifacts")
			}
		}
	}
	return result, nil
}

func readBundleFile(root, relPath string) ([]byte, error) {
	path, err := artifactFullPath(root, relPath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path) // #nosec G304 -- artifactFullPath constrains relPath under the evidence bundle root before reading.
}

func openBundleFile(root, relPath string) (*os.File, error) {
	path, err := artifactFullPath(root, relPath)
	if err != nil {
		return nil, err
	}
	return os.Open(path) // #nosec G304 -- artifactFullPath constrains relPath under the evidence bundle root before opening.
}

func verifyJSONSchema(root, relPath, want string) error {
	data, err := readBundleFile(root, relPath)
	if err != nil {
		return err
	}
	var payload struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parse JSON schema: %w", err)
	}
	if payload.SchemaVersion != want {
		return fmt.Errorf("expected schema %s, got %s", want, payload.SchemaVersion)
	}
	return nil
}

func parseRecordArtifact(root, relPath string) (int, error) {
	file, err := openBundleFile(root, relPath)
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()
	if strings.HasSuffix(relPath, ".jsonl") || strings.HasSuffix(relPath, ".ndjson") {
		scanner := bufio.NewScanner(file)
		count := 0
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(line), &payload); err != nil {
				return 0, fmt.Errorf("parse JSONL record: %w", err)
			}
			count++
		}
		if err := scanner.Err(); err != nil {
			return 0, err
		}
		return count, nil
	}
	var payload map[string]any
	if err := json.NewDecoder(file).Decode(&payload); err != nil {
		return 0, fmt.Errorf("parse JSON record: %w", err)
	}
	return 1, nil
}

func manifestHasKind(manifest Manifest, kind string) bool {
	for _, artifact := range manifest.Artifacts {
		if artifact.Kind == kind {
			return true
		}
	}
	return false
}

func missingSummaryReferences(summary string, artifacts []Artifact) []string {
	var missing []string
	for _, artifact := range artifacts {
		if !strings.Contains(summary, artifact.Path) {
			missing = append(missing, artifact.Path)
		}
	}
	return missing
}
