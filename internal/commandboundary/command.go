package commandboundary

import "strings"

const SchemaVersionClassification = "boundary.command_classification.v1"

type Class string

const (
	ClassObserveRead            Class = "C0"
	ClassLocalFileWrite         Class = "C1"
	ClassNetworkEgress          Class = "C2"
	ClassRepositoryMutation     Class = "C3"
	ClassDestructiveMutation    Class = "C4"
	ClassInfrastructureMutation Class = "C5"
	ClassCredentialAccess       Class = "C6"
	ClassPackageLifecycle       Class = "C7"
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
	ActionWarn            RecommendedAction = "warn"
	ActionRequireApproval RecommendedAction = "require_approval"
	ActionDeny            RecommendedAction = "deny"
)

type Classification struct {
	SchemaVersion     string            `json:"schema_version"`
	Command           string            `json:"command"`
	ArgsRedacted      []string          `json:"args_redacted"`
	Class             Class             `json:"class"`
	Risk              Risk              `json:"risk"`
	RecommendedAction RecommendedAction `json:"recommended_action"`
	Reason            string            `json:"reason"`
}

func (c Classification) ClassLabel() string {
	return string(c.Class) + " " + classMeaning(c.Class)
}

func (c Classification) RedactedCommandLine() string {
	parts := append([]string{c.Command}, c.ArgsRedacted...)
	return strings.Join(parts, " ")
}

func classMeaning(class Class) string {
	switch class {
	case ClassObserveRead:
		return "observe/read"
	case ClassLocalFileWrite:
		return "local file write"
	case ClassNetworkEgress:
		return "network egress"
	case ClassRepositoryMutation:
		return "repo mutation"
	case ClassDestructiveMutation:
		return "destructive local mutation"
	case ClassInfrastructureMutation:
		return "infrastructure/runtime mutation"
	case ClassCredentialAccess:
		return "credential/secret access"
	case ClassPackageLifecycle:
		return "package lifecycle execution"
	default:
		return "unknown"
	}
}

func postureFor(class Class) (Risk, RecommendedAction) {
	switch class {
	case ClassObserveRead:
		return RiskLow, ActionAllow
	case ClassLocalFileWrite:
		return RiskMedium, ActionWarn
	case ClassNetworkEgress, ClassRepositoryMutation, ClassPackageLifecycle:
		return RiskHigh, ActionRequireApproval
	case ClassDestructiveMutation, ClassInfrastructureMutation, ClassCredentialAccess:
		return RiskCritical, ActionDeny
	default:
		return RiskHigh, ActionRequireApproval
	}
}
