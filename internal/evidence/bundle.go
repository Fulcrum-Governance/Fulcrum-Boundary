package evidence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/demo"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/doctor"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/selftest"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/versioninfo"
)

func Bundle(ctx context.Context, opts BundleOptions) (*BundleResult, error) {
	sourceDir := strings.TrimSpace(opts.SourceDir)
	if sourceDir == "" {
		sourceDir = ".boundary"
	}
	outDir := strings.TrimSpace(opts.OutDir)
	if outDir == "" {
		outDir = "boundary-evidence"
	}
	absSource, err := filepath.Abs(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("resolve source: %w", err)
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return nil, fmt.Errorf("resolve output: %w", err)
	}
	if err := os.MkdirAll(absOut, 0o700); err != nil {
		return nil, fmt.Errorf("create evidence output: %w", err)
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	manifest := Manifest{
		SchemaVersion:       ManifestSchemaVersion,
		CreatedAt:           now.Format(time.RFC3339),
		Source:              absSource,
		Output:              absOut,
		Summary:             "summary.md",
		IncludeDemo:         opts.IncludeDemo,
		RequiresCredentials: false,
		RequiresNetwork:     false,
		MutatesLiveSystems:  false,
		FixtureSafeOutputs: []string{
			"version",
			"selftest",
			"doctor",
		},
	}

	addArtifact := func(relPath, kind, schema string) error {
		artifact, err := hashArtifact(absOut, relPath, kind, schema)
		if err != nil {
			return err
		}
		manifest.Artifacts = append(manifest.Artifacts, artifact)
		return nil
	}
	writeJSONArtifact := func(relPath, kind, schema string, payload any) error {
		if err := writeJSON(filepath.Join(absOut, relPath), payload); err != nil {
			return err
		}
		return addArtifact(relPath, kind, schema)
	}
	writeTextArtifact := func(relPath, kind, body string) error {
		if err := writeFile(filepath.Join(absOut, relPath), []byte(body)); err != nil {
			return err
		}
		return addArtifact(relPath, kind, "")
	}

	version := versioninfo.Current()
	if err := writeJSONArtifact("version.json", "version", versioninfo.SchemaVersion, version); err != nil {
		return nil, fmt.Errorf("write version artifact: %w", err)
	}
	if err := writeTextArtifact("version.txt", "version_text", renderVersionText(version)); err != nil {
		return nil, fmt.Errorf("write version text: %w", err)
	}

	selftestResult, err := selftest.Run(ctx, selftest.Options{})
	if err != nil {
		return nil, fmt.Errorf("run fixture selftest: %w", err)
	}
	if err := writeJSONArtifact("selftest.json", "selftest", selftest.SchemaVersion, selftestResult); err != nil {
		return nil, fmt.Errorf("write selftest artifact: %w", err)
	}
	var selftestText bytes.Buffer
	if err := selftest.WriteText(&selftestText, selftestResult, selftest.RenderOptions{}); err != nil {
		return nil, fmt.Errorf("render selftest text: %w", err)
	}
	if err := writeTextArtifact("selftest.txt", "selftest_text", selftestText.String()); err != nil {
		return nil, fmt.Errorf("write selftest text: %w", err)
	}

	doctorResult, err := doctor.Run(doctor.Options{})
	if err != nil {
		return nil, fmt.Errorf("run doctor: %w", err)
	}
	if err := writeJSONArtifact("doctor.json", "doctor", doctor.SchemaVersion, doctorResult); err != nil {
		return nil, fmt.Errorf("write doctor artifact: %w", err)
	}

	if opts.IncludeDemo {
		actionDemo, err := demo.RunActionBoundary(ctx, demo.ActionBoundaryOptions{Now: now})
		if err != nil {
			return nil, fmt.Errorf("run action-boundary demo: %w", err)
		}
		if err := writeJSONArtifact("demo/action-boundary.json", "action_boundary_demo", demo.ActionBoundarySchemaVersion, actionDemo); err != nil {
			return nil, fmt.Errorf("write action-boundary demo: %w", err)
		}
		var demoText bytes.Buffer
		if err := demo.WriteActionBoundaryText(&demoText, actionDemo); err != nil {
			return nil, fmt.Errorf("render action-boundary demo: %w", err)
		}
		if err := writeTextArtifact("demo/action-boundary.txt", "action_boundary_demo_text", demoText.String()); err != nil {
			return nil, fmt.Errorf("write action-boundary demo text: %w", err)
		}
		manifest.FixtureSafeOutputs = append(manifest.FixtureSafeOutputs, "action_boundary_demo")
	}

	copied, warnings, err := copySourceArtifacts(absSource, absOut)
	if err != nil {
		return nil, err
	}
	manifest.Warnings = append(manifest.Warnings, warnings...)
	manifest.Artifacts = append(manifest.Artifacts, copied...)
	sort.Slice(manifest.Artifacts, func(i, j int) bool {
		return manifest.Artifacts[i].Path < manifest.Artifacts[j].Path
	})

	summary := renderSummary(manifest)
	if err := writeFile(filepath.Join(absOut, manifest.Summary), []byte(summary)); err != nil {
		return nil, fmt.Errorf("write summary: %w", err)
	}
	summaryArtifact, err := hashArtifact(absOut, manifest.Summary, "summary", "")
	if err != nil {
		return nil, err
	}
	manifest.Artifacts = append(manifest.Artifacts, summaryArtifact)
	sort.Slice(manifest.Artifacts, func(i, j int) bool {
		return manifest.Artifacts[i].Path < manifest.Artifacts[j].Path
	})

	manifestPath := filepath.Join(absOut, "manifest.json")
	if err := writeJSON(manifestPath, manifest); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}
	return &BundleResult{
		Manifest:     manifest,
		ManifestPath: manifestPath,
	}, nil
}

func copySourceArtifacts(absSource, absOut string) ([]Artifact, []string, error) {
	stat, err := os.Stat(absSource)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, []string{"source directory not present; no existing .boundary artifacts copied"}, nil
		}
		return nil, nil, fmt.Errorf("inspect source: %w", err)
	}
	if !stat.IsDir() {
		return nil, nil, fmt.Errorf("source is not a directory: %s", absSource)
	}

	var artifacts []Artifact
	var warnings []string
	err = filepath.WalkDir(absSource, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if samePath(path, absOut) || isWithin(path, absOut) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			warnings = append(warnings, "skipped non-regular source artifact: "+path)
			return nil
		}
		rel, err := filepath.Rel(absSource, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel == "." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
			return fmt.Errorf("unsafe source artifact path: %s", rel)
		}
		destRel := filepath.ToSlash(filepath.Join("artifacts", rel))
		dest := filepath.Join(absOut, filepath.FromSlash(destRel))
		if err := copyFile(path, dest); err != nil {
			return err
		}
		artifact, err := hashArtifact(absOut, destRel, sourceArtifactKind(rel), "")
		if err != nil {
			return err
		}
		artifacts = append(artifacts, artifact)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("copy source artifacts: %w", err)
	}
	return artifacts, warnings, nil
}

func sourceArtifactKind(rel string) string {
	lower := strings.ToLower(rel)
	if strings.Contains(lower, "decision") || strings.Contains(lower, "record") {
		return "decision_record"
	}
	return "source_artifact"
}

func renderVersionText(info versioninfo.Info) string {
	return fmt.Sprintf("Fulcrum Boundary %s\ncommit: %s\nbuild_date: %s\ngo: %s\nmodule: %s\n",
		info.Version,
		info.Commit,
		info.BuildDate,
		info.GoVersion,
		info.Module,
	)
}

func renderSummary(manifest Manifest) string {
	var builder strings.Builder
	fmt.Fprintln(&builder, "# Boundary Evidence Bundle")
	fmt.Fprintln(&builder)
	fmt.Fprintf(&builder, "- Schema: `%s`\n", manifest.SchemaVersion)
	fmt.Fprintf(&builder, "- Created: `%s`\n", manifest.CreatedAt)
	fmt.Fprintf(&builder, "- Source: `%s`\n", manifest.Source)
	fmt.Fprintf(&builder, "- Credentials: `none`\n")
	fmt.Fprintf(&builder, "- Network: `none`\n")
	fmt.Fprintf(&builder, "- Live mutation: `none`\n")
	fmt.Fprintf(&builder, "- Manifest: `manifest.json`\n")
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "## Artifacts")
	fmt.Fprintln(&builder)
	for _, artifact := range manifest.Artifacts {
		fmt.Fprintf(&builder, "- `%s` (`%s`, %s)\n", artifact.Path, artifact.Kind, artifact.SHA256)
	}
	fmt.Fprintf(&builder, "- `%s` (`summary`)\n", manifest.Summary)
	if len(manifest.Warnings) > 0 {
		fmt.Fprintln(&builder)
		fmt.Fprintln(&builder, "## Warnings")
		fmt.Fprintln(&builder)
		for _, warning := range manifest.Warnings {
			fmt.Fprintf(&builder, "- %s\n", warning)
		}
	}
	return builder.String()
}

func writeJSON(path string, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFile(path, data)
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func samePath(a, b string) bool {
	aClean, errA := filepath.Abs(a)
	bClean, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	return filepath.Clean(aClean) == filepath.Clean(bClean)
}

func isWithin(path, parent string) bool {
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(parentAbs, pathAbs)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}
