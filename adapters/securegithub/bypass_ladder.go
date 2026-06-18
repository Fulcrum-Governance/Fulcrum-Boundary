package securegithub

// BypassLadderLevel is the Secure GitHub bypass-proof ladder rung a deployment
// has earned. It encodes how much of the "agents can only reach GitHub through
// Boundary" property is evidenced. Higher is stronger.
//
// The ladder is the machine-readable form of the levels described in
// docs/ROUTE_CONFORMANCE_CHECKLIST.md and docs/ADAPTER_READINESS_MATRIX.md:
//
//	L0 — fixture/demo denies before upstream; no credentials; no live mutation.
//	L1 — operator-owned live conformance with a controlled GitHub App;
//	     no-mutation proof for the denied write-after-taint path.
//	L2 — managed deployment topology: no direct GitHub API token, upstream GitHub
//	     MCP, SSH/git-write, unmanaged gh, or ambient-egress route outside Boundary.
//	L3 — third-party / enterprise deployment attestation + network-policy
//	     evidence (adjudicated by external review, never by this code).
//
// L2 is the internal-only "production-candidate" gate. Reaching L2 does NOT make
// Secure GitHub production: the adapter stays preview in readiness.yaml and
// claims until Boundary release truth changes. "Production-candidate" is an
// internal planning word and must never appear in public copy.
type BypassLadderLevel int

const (
	// BypassL0 is the fixture/demo floor: deny-before-upstream proven offline.
	BypassL0 BypassLadderLevel = iota
	// BypassL1 adds operator-owned live conformance with no-mutation proof.
	BypassL1
	// BypassL2 adds managed deployment topology that denies every direct path.
	BypassL2
	// BypassL3 adds third-party/enterprise attestation; never returned by code.
	BypassL3
)

// ProductionCandidateLevel is the internal gate that authorizes calling Secure
// GitHub a production-candidate. It is L2 and is never below L2. It is NOT a
// public production label.
const ProductionCandidateLevel = BypassL2

// String renders the level as L0..L3, or L? for an out-of-range value.
func (l BypassLadderLevel) String() string {
	switch l {
	case BypassL0:
		return "L0"
	case BypassL1:
		return "L1"
	case BypassL2:
		return "L2"
	case BypassL3:
		return "L3"
	default:
		return "L?"
	}
}

// LadderFacts are the machine-checkable inputs a deployment supplies to classify
// its bypass-proof level. Every field is a fact the operator or the routed
// evidence can substantiate; the classifier never assumes a fact.
//
// L1 facts are produced by the routed live-conformance harness. L2 facts are
// operator-attested deployment-topology denials (delegated, not adapter-proven);
// the classifier records what was attested and computes the level, but the
// attestation's truth is the operator's responsibility, not Boundary's.
type LadderFacts struct {
	// L1 — routed live evidence (produced by RunLiveDeniedWriteConformance).
	LiveDeniedWriteRecorded   bool // a live denied-write transcript was recorded
	LiveNoMutationProven      bool // that transcript proved github_mutation_called=false
	TranscriptSanitized       bool // the transcript declared sanitized=true
	DecisionRecordHashPresent bool // the transcript carried a decision record hash

	// L2 — operator-attested deployment-topology denials (delegated).
	AgentHasNoDirectToken    bool // the agent holds no direct GitHub token/SSH key
	AppCredentialRuntimeOnly bool // the GitHub App credential is held only by the governed runtime
	UpstreamMCPUnavailable   bool // the upstream GitHub MCP server is unreachable by the agent
	NoUnmanagedGitOrGH       bool // no unmanaged gh/git/SSH write path is available to the agent
	EgressPolicyEnforced     bool // egress/network policy prevents bypassing Boundary for GitHub writes

	// L3 — third-party attestation, recorded but NOT adjudicated by this code.
	ThirdPartyAttestation bool
}

// ClassifyBypassLevel returns the highest ladder level the supplied facts
// support, plus an ordered list of reasons each higher level was not reached. It
// is pure and deterministic. It caps at L2: L3 is third-party attestation
// reviewed outside this code, so even a fully attested packet returns at most L2.
// Reasons are always non-empty below L2 so callers can show the next gate.
func ClassifyBypassLevel(f LadderFacts) (level BypassLadderLevel, reasons []string) {
	l1 := f.LiveDeniedWriteRecorded && f.LiveNoMutationProven &&
		f.TranscriptSanitized && f.DecisionRecordHashPresent
	if !l1 {
		if !f.LiveDeniedWriteRecorded {
			reasons = append(reasons, "L1 requires a recorded operator-owned live denied-write conformance transcript")
		}
		if !f.LiveNoMutationProven {
			reasons = append(reasons, "L1 requires the live transcript to prove github_mutation_called=false")
		}
		if !f.TranscriptSanitized {
			reasons = append(reasons, "L1 requires a sanitized transcript")
		}
		if !f.DecisionRecordHashPresent {
			reasons = append(reasons, "L1 requires a decision record hash in the transcript")
		}
		return BypassL0, reasons
	}

	type denial struct {
		ok     bool
		reason string
	}
	denials := []denial{
		{f.AgentHasNoDirectToken, "L2 requires attesting the agent holds no direct GitHub token or SSH key"},
		{f.AppCredentialRuntimeOnly, "L2 requires attesting the GitHub App credential is held only by the governed runtime"},
		{f.UpstreamMCPUnavailable, "L2 requires attesting the upstream GitHub MCP server is unreachable by the agent"},
		{f.NoUnmanagedGitOrGH, "L2 requires attesting no unmanaged gh/git/SSH write path is available to the agent"},
		{f.EgressPolicyEnforced, "L2 requires attesting egress/network policy prevents bypassing Boundary for GitHub writes"},
	}
	l2 := true
	for _, d := range denials {
		if !d.ok {
			l2 = false
			reasons = append(reasons, d.reason)
		}
	}
	if !l2 {
		return BypassL1, reasons
	}

	// L2 reached. Cap here: L3 is external-attestation review, not a code verdict.
	reasons = append(reasons, "L3 (third-party/enterprise attestation) is reviewed outside Boundary and is never asserted by code")
	return BypassL2, reasons
}
