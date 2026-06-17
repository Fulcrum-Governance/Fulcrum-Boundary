package securegithub

import (
	"strings"
	"testing"
)

func TestBypassLadderLevelString(t *testing.T) {
	cases := map[BypassLadderLevel]string{
		BypassL0:              "L0",
		BypassL1:              "L1",
		BypassL2:              "L2",
		BypassL3:              "L3",
		BypassLadderLevel(99): "L?",
		BypassLadderLevel(-1): "L?",
	}
	for level, want := range cases {
		if got := level.String(); got != want {
			t.Fatalf("level %d String() = %q, want %q", int(level), got, want)
		}
	}
}

func TestProductionCandidateGateIsL2(t *testing.T) {
	if ProductionCandidateLevel != BypassL2 {
		t.Fatalf("ProductionCandidateLevel = %s, want L2 (production-candidate is internal-only and never below L2)", ProductionCandidateLevel)
	}
}

func TestClassifyBypassLevelFixtureOnlyIsL0(t *testing.T) {
	// No live conformance, no topology attestation -> L0 fixture/demo floor.
	level, reasons := ClassifyBypassLevel(LadderFacts{})
	if level != BypassL0 {
		t.Fatalf("empty facts classified as %s, want L0", level)
	}
	if len(reasons) == 0 {
		t.Fatal("expected reasons explaining why L1+ was not reached")
	}
	if !strings.Contains(strings.Join(reasons, " | "), "L1") {
		t.Fatalf("reasons must explain the missing L1 gate: %q", strings.Join(reasons, " | "))
	}
}

func TestClassifyBypassLevelL1(t *testing.T) {
	// Operator-owned live denied-write conformance recorded, no-mutation proven,
	// but no topology attestation -> L1.
	f := LadderFacts{
		LiveDeniedWriteRecorded:   true,
		LiveNoMutationProven:      true,
		TranscriptSanitized:       true,
		DecisionRecordHashPresent: true,
	}
	level, reasons := ClassifyBypassLevel(f)
	if level != BypassL1 {
		t.Fatalf("classified as %s, want L1; reasons=%v", level, reasons)
	}
	if strings.Join(reasons, " | ") == "" {
		t.Fatal("L1 must still report why L2 was not reached")
	}
}

func TestClassifyBypassLevelL2RequiresAllTopologyDenials(t *testing.T) {
	base := LadderFacts{
		LiveDeniedWriteRecorded:   true,
		LiveNoMutationProven:      true,
		TranscriptSanitized:       true,
		DecisionRecordHashPresent: true,
		AgentHasNoDirectToken:     true,
		AppCredentialRuntimeOnly:  true,
		UpstreamMCPUnavailable:    true,
		NoUnmanagedGitOrGH:        true,
		EgressPolicyEnforced:      true,
	}
	level, reasons := ClassifyBypassLevel(base)
	if level != BypassL2 {
		t.Fatalf("full topology facts classified as %s, want L2; reasons=%v", level, reasons)
	}
	// Drop one denial -> must fall back to L1 with a reason naming the gap.
	partial := base
	partial.EgressPolicyEnforced = false
	level, reasons = ClassifyBypassLevel(partial)
	if level != BypassL1 {
		t.Fatalf("missing egress denial classified as %s, want L1", level)
	}
	if !strings.Contains(strings.Join(reasons, " | "), "egress") {
		t.Fatalf("reason must name the missing egress denial: %v", reasons)
	}
}

func TestClassifyBypassLevelNeverReturnsL3FromFacts(t *testing.T) {
	// L3 is third-party/enterprise attestation, out of code scope. Even a fully
	// attested L2 packet must not auto-promote to L3.
	f := LadderFacts{
		LiveDeniedWriteRecorded:   true,
		LiveNoMutationProven:      true,
		TranscriptSanitized:       true,
		DecisionRecordHashPresent: true,
		AgentHasNoDirectToken:     true,
		AppCredentialRuntimeOnly:  true,
		UpstreamMCPUnavailable:    true,
		NoUnmanagedGitOrGH:        true,
		EgressPolicyEnforced:      true,
		ThirdPartyAttestation:     true, // recorded, but code does not adjudicate it
	}
	level, _ := ClassifyBypassLevel(f)
	if level > BypassL2 {
		t.Fatalf("classifier returned %s; code must cap at L2 and leave L3 to external attestation review", level)
	}
}
