package editboundary

import "strings"

const SchemaVersionInspection = "boundary.edit_inspection.v1"

type Class string

const (
	ClassNoop                Class = "E0"
	ClassSafeContent         Class = "E1"
	ClassSourceConfig        Class = "E2"
	ClassDeploymentInfra     Class = "E3"
	ClassSecretBearing       Class = "E4"
	ClassDestructive         Class = "E5"
	ClassExecutionBehavior   Class = "E6"
	ClassOutsideProjectScope Class = "E7"
)

type Risk string

const (
	RiskLow      Risk = "LOW"
	RiskMedium   Risk = "MEDIUM"
	RiskHigh     Risk = "HIGH"
	RiskCritical Risk = "CRITICAL"
)

type RecommendedAction string

const (
	ActionAllow           RecommendedAction = "allow"
	ActionRequireApproval RecommendedAction = "require_approval"
	ActionDeny            RecommendedAction = "deny"
)

type Inspection struct {
	SchemaVersion     string            `json:"schema_version"`
	FilesTouched      int               `json:"files_touched"`
	PatchSHA256       string            `json:"patch_sha256"`
	HighestClass      Class             `json:"highest_class"`
	Risk              Risk              `json:"risk"`
	RecommendedAction RecommendedAction `json:"recommended_action"`
	Findings          []Finding         `json:"findings"`
}

func (i Inspection) RedactedPaths() []string {
	paths := make([]string, 0, len(i.Findings))
	for _, finding := range i.Findings {
		if finding.Redacted {
			paths = append(paths, finding.Path)
		}
	}
	return paths
}

func (i Inspection) RecordPaths() []string {
	paths := make([]string, 0, len(i.Findings))
	for _, finding := range i.Findings {
		paths = append(paths, finding.Path)
	}
	return paths
}

type Finding struct {
	Path        string            `json:"path"`
	Operation   Operation         `json:"operation"`
	Class       Class             `json:"class"`
	Risk        Risk              `json:"risk"`
	Action      RecommendedAction `json:"recommended_action"`
	Reason      string            `json:"reason"`
	Redacted    bool              `json:"redacted,omitempty"`
	Unsupported bool              `json:"unsupported,omitempty"`
}

func (i Inspection) HighestClassLabel() string {
	return string(i.HighestClass) + " " + classMeaning(i.HighestClass)
}

func (f Finding) ClassLabel() string {
	return string(f.Class) + " " + classMeaning(f.Class)
}

func classMeaning(class Class) string {
	switch class {
	case ClassNoop:
		return "metadata/no-op"
	case ClassSafeContent:
		return "safe content edit"
	case ClassSourceConfig:
		return "source/config mutation"
	case ClassDeploymentInfra:
		return "deployment/infrastructure mutation"
	case ClassSecretBearing:
		return "secret-bearing edit"
	case ClassDestructive:
		return "destructive edit"
	case ClassExecutionBehavior:
		return "execution behavior mutation"
	case ClassOutsideProjectScope:
		return "outside project scope"
	default:
		return "unknown"
	}
}

func postureFor(class Class) (Risk, RecommendedAction) {
	switch class {
	case ClassNoop, ClassSafeContent:
		return RiskLow, ActionAllow
	case ClassSourceConfig, ClassDeploymentInfra, ClassExecutionBehavior:
		return RiskHigh, ActionRequireApproval
	case ClassSecretBearing, ClassDestructive, ClassOutsideProjectScope:
		return RiskCritical, ActionDeny
	default:
		return RiskHigh, ActionRequireApproval
	}
}

func highestClass(findings []Finding) Class {
	highest := ClassNoop
	for _, finding := range findings {
		if classRank(finding.Class) > classRank(highest) {
			highest = finding.Class
		}
	}
	return highest
}

func classRank(class Class) int {
	switch class {
	case ClassNoop:
		return 0
	case ClassSafeContent:
		return 1
	case ClassSourceConfig:
		return 2
	case ClassDeploymentInfra:
		return 3
	case ClassExecutionBehavior:
		return 4
	case ClassSecretBearing:
		return 5
	case ClassDestructive:
		return 6
	case ClassOutsideProjectScope:
		return 7
	default:
		return 4
	}
}

func (a RecommendedAction) String() string {
	return string(a)
}

func (c Class) String() string {
	return string(c)
}

func classFromPathFallback(path string) Class {
	if strings.TrimSpace(path) == "" {
		return ClassNoop
	}
	return ClassSafeContent
}
