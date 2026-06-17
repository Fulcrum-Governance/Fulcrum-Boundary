package securegithub

import (
	"strings"
	"testing"
)

func fullAttestation() DeploymentAttestation {
	return DeploymentAttestation{
		AgentHasNoDirectToken:    AttestedDenial{Attested: true, Evidence: "no PAT/SSH key mounted in agent runtime; verified by deploy manifest"},
		AppCredentialRuntimeOnly: AttestedDenial{Attested: true, Evidence: "GitHub App private key sealed to governed runtime secret store"},
		UpstreamMCPUnavailable:   AttestedDenial{Attested: true, Evidence: "no upstream github-mcp endpoint reachable from agent network namespace"},
		NoUnmanagedGitOrGH:       AttestedDenial{Attested: true, Evidence: "gh/git absent from agent image; only Boundary route present"},
		EgressPolicyEnforced:     AttestedDenial{Attested: true, Evidence: "egress NetworkPolicy denies api.github.com except from Boundary pod"},
	}
}

func TestBuildBypassProofPacketL2(t *testing.T) {
	idx, err := BuildLiveEvidenceIndex([]LiveConformanceResult{deniedWriteResult()})
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	p, err := BuildBypassProofPacket(idx, fullAttestation())
	if err != nil {
		t.Fatalf("BuildBypassProofPacket: %v", err)
	}
	if p.SchemaVersion != BypassProofSchemaVersion {
		t.Fatalf("schema = %q", p.SchemaVersion)
	}
	if p.ProfileStatus != StatusPreview {
		t.Fatalf("profile status = %q; Secure GitHub stays preview even at L2", p.ProfileStatus)
	}
	if p.Level != BypassL2 {
		t.Fatalf("level = %s, want L2; reasons=%v", p.Level, p.Reasons)
	}
	if p.LevelString != "L2" {
		t.Fatalf("level string = %q, want L2", p.LevelString)
	}
	if !p.IsProductionCandidate() {
		t.Fatal("a preview L2 packet must be a production-candidate")
	}
}

func TestBypassPacketFailsClosedOnMissingDenial(t *testing.T) {
	idx, _ := BuildLiveEvidenceIndex([]LiveConformanceResult{deniedWriteResult()})
	att := fullAttestation()
	att.EgressPolicyEnforced = AttestedDenial{Attested: false}
	p, err := BuildBypassProofPacket(idx, att)
	if err != nil {
		t.Fatalf("packet build: %v", err)
	}
	if p.Level != BypassL1 {
		t.Fatalf("missing egress denial must cap at L1, got %s", p.Level)
	}
	if p.IsProductionCandidate() {
		t.Fatal("L1 packet must NOT be a production-candidate")
	}
	if !strings.Contains(strings.Join(p.Reasons, " | "), "egress") {
		t.Fatalf("reasons must name the missing egress denial: %v", p.Reasons)
	}
}

func TestBypassPacketFailsClosedWithoutL1Evidence(t *testing.T) {
	// Full topology attestation but no live evidence -> still L0 (no L1 floor).
	idx := LiveEvidenceIndex{SchemaVersion: LiveEvidenceSchemaVersion, ProfileID: ProfileID, ProfileStatus: StatusPreview, Sanitized: true}
	p, err := BuildBypassProofPacket(idx, fullAttestation())
	if err != nil {
		t.Fatalf("packet build: %v", err)
	}
	if p.Level != BypassL0 {
		t.Fatalf("no live evidence must yield L0 regardless of attestation, got %s", p.Level)
	}
	if p.IsProductionCandidate() {
		t.Fatal("L0 packet must not be a production-candidate")
	}
}

func TestBypassPacketRejectsAttestationEvidenceWithSecrets(t *testing.T) {
	idx, _ := BuildLiveEvidenceIndex([]LiveConformanceResult{deniedWriteResult()})
	att := fullAttestation()
	att.AppCredentialRuntimeOnly = AttestedDenial{Attested: true, Evidence: "Authorization: Bearer ghp_exampleTokenMaterialThatMustNotBeStored"}
	if _, err := BuildBypassProofPacket(idx, att); err == nil {
		t.Fatal("expected fail-closed error when attestation evidence carries secret-like data")
	}
}

func TestBypassPacketUnsanitizedIndexRejected(t *testing.T) {
	idx, _ := BuildLiveEvidenceIndex([]LiveConformanceResult{deniedWriteResult()})
	idx.Sanitized = false
	if _, err := BuildBypassProofPacket(idx, fullAttestation()); err == nil {
		t.Fatal("expected fail-closed error for unsanitized live-evidence index")
	}
}
