package editboundary

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func InspectPatch(patch []byte) (Inspection, error) {
	changes, err := ParsePatch(patch)
	if err != nil {
		return Inspection{}, err
	}
	if len(changes) == 0 {
		risk, action := postureFor(ClassNoop)
		return Inspection{
			SchemaVersion:     SchemaVersionInspection,
			FilesTouched:      0,
			PatchSHA256:       HashPatch(patch),
			HighestClass:      ClassNoop,
			Risk:              risk,
			RecommendedAction: action,
			Findings:          nil,
		}, nil
	}

	findings := make([]Finding, 0, len(changes))
	for _, change := range changes {
		findings = append(findings, classifyChange(change))
	}
	highest := highestClass(findings)
	risk, action := postureFor(highest)
	return Inspection{
		SchemaVersion:     SchemaVersionInspection,
		FilesTouched:      len(findings),
		PatchSHA256:       HashPatch(patch),
		HighestClass:      highest,
		Risk:              risk,
		RecommendedAction: action,
		Findings:          findings,
	}, nil
}

func HashPatch(patch []byte) string {
	sum := sha256.Sum256(patch)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func classifyChange(change FileChange) Finding {
	target := change.TargetPath()
	pathCheck := CheckProjectPath(target)
	if !pathCheck.Safe {
		risk, action := postureFor(ClassOutsideProjectScope)
		return Finding{
			Path:      pathCheck.Path,
			Operation: change.Operation,
			Class:     ClassOutsideProjectScope,
			Risk:      risk,
			Action:    action,
			Reason:    pathCheck.Reason,
			Redacted:  pathCheck.Path != target,
		}
	}

	displayPath := pathCheck.Path
	redacted := false
	if IsSecretPath(target) || IsSecretPath(change.OldPath) || IsSecretPath(change.NewPath) {
		displayPath = redactedSecretPath
		redacted = true
		risk, action := postureFor(ClassSecretBearing)
		return Finding{
			Path:      displayPath,
			Operation: change.Operation,
			Class:     ClassSecretBearing,
			Risk:      risk,
			Action:    action,
			Reason:    "secret-bearing path denied",
			Redacted:  redacted,
		}
	}
	if ContainsSecretContent(change.AddedLines) {
		risk, action := postureFor(ClassSecretBearing)
		return Finding{
			Path:      displayPath,
			Operation: change.Operation,
			Class:     ClassSecretBearing,
			Risk:      risk,
			Action:    action,
			Reason:    "secret-looking content added",
		}
	}
	if change.Operation == OperationDelete || destructiveVolume(change) {
		risk, action := postureFor(ClassDestructive)
		return Finding{
			Path:      displayPath,
			Operation: change.Operation,
			Class:     ClassDestructive,
			Risk:      risk,
			Action:    action,
			Reason:    "destructive edit denied",
		}
	}
	if change.Binary {
		risk, action := postureFor(ClassSourceConfig)
		return Finding{
			Path:        displayPath,
			Operation:   change.Operation,
			Class:       ClassSourceConfig,
			Risk:        risk,
			Action:      action,
			Reason:      "binary patch requires approval",
			Unsupported: true,
		}
	}
	if isExecutionBehaviorChange(change, displayPath) {
		risk, action := postureFor(ClassExecutionBehavior)
		return Finding{
			Path:      displayPath,
			Operation: change.Operation,
			Class:     ClassExecutionBehavior,
			Risk:      risk,
			Action:    action,
			Reason:    "execution behavior mutation",
		}
	}
	if isDeploymentInfraPath(displayPath) {
		risk, action := postureFor(ClassDeploymentInfra)
		return Finding{
			Path:      displayPath,
			Operation: change.Operation,
			Class:     ClassDeploymentInfra,
			Risk:      risk,
			Action:    action,
			Reason:    "deployment or infrastructure mutation",
		}
	}
	if isSourceOrConfigPath(displayPath) {
		risk, action := postureFor(ClassSourceConfig)
		return Finding{
			Path:      displayPath,
			Operation: change.Operation,
			Class:     ClassSourceConfig,
			Risk:      risk,
			Action:    action,
			Reason:    "source or config mutation",
		}
	}
	class := classFromPathFallback(displayPath)
	risk, action := postureFor(class)
	return Finding{
		Path:      displayPath,
		Operation: change.Operation,
		Class:     class,
		Risk:      risk,
		Action:    action,
		Reason:    "safe content edit",
	}
}

func destructiveVolume(change FileChange) bool {
	return len(change.DeletedLines) > 500 || len(change.AddedLines)+len(change.DeletedLines) > 1000
}

func isExecutionBehaviorChange(change FileChange, target string) bool {
	lower := strings.ToLower(target)
	if lower == "package.json" && linesContain(change.AddedLines, `"scripts"`) {
		return true
	}
	if strings.HasPrefix(lower, ".github/workflows/") {
		return true
	}
	if lower == "dockerfile" || strings.HasPrefix(lower, "dockerfile.") {
		return true
	}
	if strings.HasPrefix(lower, "docker-compose") || strings.HasSuffix(lower, "/docker-compose.yml") || strings.HasSuffix(lower, "/docker-compose.yaml") {
		return true
	}
	if lower == "makefile" || strings.HasSuffix(lower, "/makefile") {
		return true
	}
	if strings.HasPrefix(lower, "scripts/") && (strings.HasSuffix(lower, ".sh") || strings.HasSuffix(lower, ".bash")) {
		return true
	}
	if strings.Contains(lower, "/hooks/") || strings.HasSuffix(lower, "/hook") || change.ModeChanged {
		return true
	}
	if lower == "pyproject.toml" && (linesContain(change.AddedLines, "build-system") || linesContain(change.AddedLines, "scripts")) {
		return true
	}
	return false
}

func isDeploymentInfraPath(target string) bool {
	lower := strings.ToLower(target)
	if strings.HasSuffix(lower, ".tf") || strings.Contains(lower, "terraform") {
		return true
	}
	if strings.Contains(lower, "kubernetes") || strings.Contains(lower, "k8s/") || strings.Contains(lower, "helm/") {
		return true
	}
	if strings.HasSuffix(lower, "chart.yaml") || strings.HasSuffix(lower, "deploy.yaml") || strings.HasSuffix(lower, "deploy.yml") {
		return true
	}
	return false
}

func isSourceOrConfigPath(target string) bool {
	lower := strings.ToLower(target)
	if strings.HasPrefix(lower, "docs/") || strings.HasSuffix(lower, ".md") || strings.Contains(lower, "testdata/") {
		return false
	}
	for _, suffix := range []string{
		".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".rb",
		".json", ".yaml", ".yml", ".toml", ".mod", ".sum", ".lock",
	} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func linesContain(lines []string, needle string) bool {
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
