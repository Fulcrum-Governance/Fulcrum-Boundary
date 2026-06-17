package securegithub

import (
	"encoding/json"
	"fmt"
	"time"
)

// BypassProofSchemaVersion identifies the deployment bypass-proof packet shape.
const BypassProofSchemaVersion = "boundary.secure_github.bypass_proof_packet.v1"

// AttestedDenial is one operator-attested deployment-topology denial: whether the
// operator attests the path is closed, and a short, non-secret reference to the
// evidence (a manifest name, a policy id, a runbook link). The Evidence string
// must never contain credentials; the validator rejects secret-like evidence. An
// AttestedDenial is the operator's claim, not Boundary's proof.
type AttestedDenial struct {
	Attested bool   `json:"attested"`
	Evidence string `json:"evidence,omitempty"`
}

// DeploymentAttestation is the operator-owned L2 topology attestation: the five
// token-custody / direct-path denials a managed deployment must close so GitHub
// writes can only reach GitHub through Boundary. It is deployment-delegated
// evidence; Boundary records and classifies it but does not verify the
// deployment.
type DeploymentAttestation struct {
	AgentHasNoDirectToken    AttestedDenial `json:"agent_has_no_direct_token"`
	AppCredentialRuntimeOnly AttestedDenial `json:"app_credential_runtime_only"`
	UpstreamMCPUnavailable   AttestedDenial `json:"upstream_mcp_unavailable"`
	NoUnmanagedGitOrGH       AttestedDenial `json:"no_unmanaged_git_or_gh"`
	EgressPolicyEnforced     AttestedDenial `json:"egress_policy_enforced"`
}

// BypassProofPacket binds operator-owned live evidence (L1) to an operator
// deployment-topology attestation (L2) and records the earned ladder level. The
// packet is the product artifact for "here are the direct paths an agent could
// have used and how this deployment denies them." It asserts only what the
// evidence proves and what the operator attested; it never claims the deployment
// is bypass-proof, and Secure GitHub stays preview at every level.
type BypassProofPacket struct {
	SchemaVersion string                `json:"schema_version"`
	ProfileID     string                `json:"profile_id"`
	ProfileStatus string                `json:"profile_status"`
	GeneratedAt   time.Time             `json:"generated_at"`
	LiveEvidence  LiveEvidenceIndex     `json:"live_evidence"`
	Attestation   DeploymentAttestation `json:"deployment_attestation"`
	Level         BypassLadderLevel     `json:"level"`
	LevelString   string                `json:"level_string"`
	Reasons       []string              `json:"reasons"`
}

// containsCredentialPattern checks attestation evidence strings for actual
// credential material (token values, bearer tokens, raw keys) using only regex
// patterns. Unlike containsSecretLikeData it does not trigger on descriptive
// phrases such as "private key sealed to runtime" that refer to the location of a
// credential rather than the credential itself.
func containsCredentialPattern(text string) bool {
	for _, pattern := range credentialPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// BuildBypassProofPacket validates the inputs, composes the L1 routed facts with
// the L2 attested facts, classifies the level, and returns the packet. It fails
// closed: an unsanitized live-evidence index or secret-like attestation evidence
// is rejected; any unattested L2 denial caps the level below L2; absent live
// evidence caps it at L0.
func BuildBypassProofPacket(idx LiveEvidenceIndex, att DeploymentAttestation) (BypassProofPacket, error) {
	if !idx.Sanitized {
		return BypassProofPacket{}, fmt.Errorf("bypass-proof packet requires a sanitized live-evidence index")
	}
	for name, d := range map[string]AttestedDenial{
		"agent_has_no_direct_token":   att.AgentHasNoDirectToken,
		"app_credential_runtime_only": att.AppCredentialRuntimeOnly,
		"upstream_mcp_unavailable":    att.UpstreamMCPUnavailable,
		"no_unmanaged_git_or_gh":      att.NoUnmanagedGitOrGH,
		"egress_policy_enforced":      att.EgressPolicyEnforced,
	} {
		if containsCredentialPattern(d.Evidence) {
			return BypassProofPacket{}, fmt.Errorf("attestation %q evidence contains secret-like data; reference a manifest or policy id, not a credential", name)
		}
	}

	facts := idx.LadderFacts() // L1 facts from routed evidence
	facts.AgentHasNoDirectToken = att.AgentHasNoDirectToken.Attested
	facts.AppCredentialRuntimeOnly = att.AppCredentialRuntimeOnly.Attested
	facts.UpstreamMCPUnavailable = att.UpstreamMCPUnavailable.Attested
	facts.NoUnmanagedGitOrGH = att.NoUnmanagedGitOrGH.Attested
	facts.EgressPolicyEnforced = att.EgressPolicyEnforced.Attested

	level, reasons := ClassifyBypassLevel(facts)
	packet := BypassProofPacket{
		SchemaVersion: BypassProofSchemaVersion,
		ProfileID:     ProfileID,
		ProfileStatus: StatusPreview,
		GeneratedAt:   time.Now().UTC(),
		LiveEvidence:  idx,
		Attestation:   att,
		Level:         level,
		LevelString:   level.String(),
		Reasons:       reasons,
	}
	body, err := json.Marshal(packet)
	if err != nil {
		return BypassProofPacket{}, err
	}
	if containsCredentialPattern(string(body)) {
		return BypassProofPacket{}, fmt.Errorf("refusing to build Secure GitHub bypass-proof packet with secret-like data")
	}
	return packet, nil
}

// IsProductionCandidate reports whether the packet has reached the internal-only
// production-candidate gate (L2) while the adapter remains preview. It is never a
// public production signal: Secure GitHub stays preview until Boundary release
// truth changes.
func (p BypassProofPacket) IsProductionCandidate() bool {
	return p.Level >= ProductionCandidateLevel && p.ProfileStatus == StatusPreview
}
