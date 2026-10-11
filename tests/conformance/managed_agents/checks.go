// Package managedagentsconformance holds the Managed Agents conformance
// transcript schema and the criterion checks that evaluate it.
//
// The checks live in a non-test file so BOTH consumers run the same
// validation logic: the env-gated live harness (conformance_test.go, driven
// by BOUNDARY_MA_CONFORMANCE/BOUNDARY_MA_TRANSCRIPT) and the session driver's
// own offline tests, which exercise the criteria in-process against a stub
// transcript without environment variables.
package managedagentsconformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Transcript is the sanitized session evidence document the conformance
// criteria evaluate. Mode distinguishes a real upstream run ("live") from an
// offline driver run ("stub"); a stub transcript must never be accepted as
// live conformance evidence.
type Transcript struct {
	Sanitized      bool               `json:"sanitized"`
	Mode           string             `json:"mode,omitempty"`
	SessionCreated bool               `json:"session_created_through_boundary"`
	SessionID      string             `json:"session_id"`
	ThreadID       string             `json:"thread_id"`
	AgentID        string             `json:"agent_id"`
	Events         []transcriptEvent  `json:"events"`
	Confirmations  []confirmation     `json:"confirmations"`
	Decisions      []decisionRecord   `json:"decisions"`
	Budget         budgetEvidence     `json:"budget"`
	Trust          trustEvidence      `json:"trust"`
	FailClosed     failClosedEvidence `json:"fail_closed"`
	Provenance     *provenanceLinkage `json:"provenance,omitempty"`
	NotObserved    []string           `json:"criteria_not_observed,omitempty"`
	TranscriptHash string             `json:"transcript_sha256,omitempty"`
}

// provenanceLinkage is the transcript-embedded link to the run's
// provenance.json and sanitized raw event log. SessionID is the same
// pseudonym as the top-level session_id; no raw identifier appears.
type provenanceLinkage struct {
	Mode         string `json:"mode"`
	SessionID    string `json:"session_id"`
	RawLogSHA256 string `json:"raw_log_sha256"`
}

type transcriptEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
	Tool      string `json:"tool,omitempty"`
}

type confirmation struct {
	ToolUseID string `json:"tool_use_id"`
	Result    string `json:"result"`
	Tool      string `json:"tool,omitempty"`
	// Delivered mirrors the driver's confirmation field; it must round-trip
	// so CheckLiveProvenance can reproduce the recorded transcript digest.
	Delivered *bool `json:"delivered,omitempty"`
	// Enforced mirrors the driver's not-held deny marker; same round-trip need.
	Enforced *bool `json:"enforced,omitempty"`
}

type decisionRecord struct {
	AgentID   string  `json:"agent_id"`
	SessionID string  `json:"session_id"`
	ThreadID  string  `json:"thread_id"`
	Tool      string  `json:"tool"`
	Action    string  `json:"action"`
	Rule      string  `json:"rule"`
	Trust     float64 `json:"trust"`
	RequestID string  `json:"request_id,omitempty"`
	Envelope  string  `json:"envelope_id,omitempty"`
}

type budgetEvidence struct {
	Ceiling       float64 `json:"ceiling"`
	Used          float64 `json:"used"`
	UsageObserved bool    `json:"usage_observed"`
	SpendUnknown  bool    `json:"spend_unknown,omitempty"`
}

type trustEvidence struct {
	Tracked bool    `json:"tracked"`
	Score   float64 `json:"score"`
}

type failClosedEvidence struct {
	Observed bool   `json:"observed"`
	Action   string `json:"action"`
	Reason   string `json:"reason,omitempty"`
}

// ParseTranscript unmarshals one sanitized transcript document.
func ParseTranscript(data []byte) (Transcript, error) {
	var tr Transcript
	if err := json.Unmarshal(data, &tr); err != nil {
		return Transcript{}, fmt.Errorf("parse transcript: %w", err)
	}
	return tr, nil
}

// LoadTranscriptFile reads and parses the transcript at path, returning both
// the decoded document and the raw bytes (the sanitized check scans the raw
// bytes for secret-shaped data).
func LoadTranscriptFile(path string) (Transcript, []byte, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Transcript{}, nil, fmt.Errorf("read transcript: %w", err)
	}
	tr, err := ParseTranscript(data)
	if err != nil {
		return Transcript{}, nil, err
	}
	return tr, data, nil
}

// Check is one named conformance criterion. Run returns nil when the
// transcript satisfies the criterion and a descriptive error otherwise.
type Check struct {
	Name string
	Run  func(tr Transcript, raw []byte) error
}

// Checks returns the conformance criteria in the order
// docs/adapters/MANAGED_AGENTS_CONFORMANCE.md lists them, with the live-mode
// gate last. Drivers validating offline (stub) transcripts expect every
// check to pass except live_mode_transcript_only, which must reject a stub.
func Checks() []Check {
	return []Check{
		{Name: "session_created_through_boundary", Run: func(tr Transcript, _ []byte) error { return CheckSessionCreated(tr) }},
		{Name: "tool_confirmation_allow", Run: func(tr Transcript, _ []byte) error { return CheckAllowConfirmation(tr) }},
		{Name: "tool_confirmation_deny", Run: func(tr Transcript, _ []byte) error { return CheckDenyConfirmation(tr) }},
		{Name: "mcp_tool_use_event", Run: func(tr Transcript, _ []byte) error { return CheckMCPToolUse(tr) }},
		{Name: "thread_creation_and_tracking", Run: func(tr Transcript, _ []byte) error { return CheckThreadTracking(tr) }},
		{Name: "budget_tracking_against_ceiling", Run: func(tr Transcript, _ []byte) error { return CheckBudget(tr) }},
		{Name: "trust_tracking_in_decisions", Run: func(tr Transcript, _ []byte) error { return CheckTrustScores(tr) }},
		{Name: "decision_metadata", Run: func(tr Transcript, _ []byte) error { return CheckDecisionMetadata(tr) }},
		{Name: "fail_closed_on_pipeline_error", Run: func(tr Transcript, _ []byte) error { return CheckFailClosed(tr) }},
		{Name: "sanitized_transcript_evidence", Run: CheckSanitized},
		{Name: "live_mode_transcript_only", Run: func(tr Transcript, _ []byte) error { return CheckLiveMode(tr) }},
	}
}

// CheckSessionCreated requires evidence the session was created through the
// Boundary proxy.
func CheckSessionCreated(tr Transcript) error {
	if !tr.SessionCreated || tr.SessionID == "" {
		return fmt.Errorf("session creation through Boundary proxy not recorded: %+v", tr)
	}
	return nil
}

// CheckAllowConfirmation requires at least one allow confirmation.
func CheckAllowConfirmation(tr Transcript) error {
	if !hasConfirmation(tr, "allow") {
		return fmt.Errorf("no allow confirmation recorded: %+v", tr.Confirmations)
	}
	return nil
}

// CheckDenyConfirmation requires at least one deny confirmation.
func CheckDenyConfirmation(tr Transcript) error {
	if !hasConfirmation(tr, "deny") {
		return fmt.Errorf("no deny confirmation recorded: %+v", tr.Confirmations)
	}
	return nil
}

// CheckMCPToolUse requires an agent.mcp_tool_use event.
func CheckMCPToolUse(tr Transcript) error {
	if !hasEvent(tr, "agent.mcp_tool_use") {
		return fmt.Errorf("no agent.mcp_tool_use event recorded: %+v", tr.Events)
	}
	return nil
}

// CheckThreadTracking requires thread creation/tracking evidence.
func CheckThreadTracking(tr Transcript) error {
	if tr.ThreadID == "" && !hasEvent(tr, "session.thread_created") {
		return fmt.Errorf("thread creation/tracking evidence missing: thread_id=%q events=%+v", tr.ThreadID, tr.Events)
	}
	return nil
}

// CheckBudget requires a positive ceiling with usage inside it AND observed
// usage evidence: a recorded Used of 0 means nothing unless upstream usage
// actually arrived, and a run that flagged spend as unknown can never satisfy
// the criterion.
func CheckBudget(tr Transcript) error {
	if tr.Budget.Ceiling <= 0 {
		return fmt.Errorf("budget ceiling missing: %+v", tr.Budget)
	}
	if tr.Budget.SpendUnknown {
		return fmt.Errorf("spend was recorded as unknown; not budget evidence: %+v", tr.Budget)
	}
	if !tr.Budget.UsageObserved {
		return fmt.Errorf("no observed usage evidence; used=%v is not evidence: %+v", tr.Budget.Used, tr.Budget)
	}
	if tr.Budget.Used < 0 || tr.Budget.Used > tr.Budget.Ceiling {
		return fmt.Errorf("budget usage outside ceiling: %+v", tr.Budget)
	}
	return nil
}

// CheckTrustScores requires trust tracking and a non-zero score on every
// decision record.
func CheckTrustScores(tr Transcript) error {
	if !tr.Trust.Tracked {
		return fmt.Errorf("trust tracking not recorded")
	}
	for _, decision := range tr.Decisions {
		if decision.Trust == 0 {
			return fmt.Errorf("decision missing trust score: %+v", decision)
		}
	}
	return nil
}

// CheckDecisionMetadata requires the full metadata set on every decision.
func CheckDecisionMetadata(tr Transcript) error {
	if len(tr.Decisions) == 0 {
		return fmt.Errorf("no decision records in transcript")
	}
	for _, decision := range tr.Decisions {
		if missing := missingDecisionFields(decision); len(missing) > 0 {
			return fmt.Errorf("decision missing metadata %v: %+v", missing, decision)
		}
	}
	return nil
}

// CheckFailClosed requires observed fail-closed evidence whose recorded
// action is deny (ADR-047: a required check that cannot produce a valid
// result must block execution).
func CheckFailClosed(tr Transcript) error {
	if !tr.FailClosed.Observed {
		return fmt.Errorf("fail-closed pipeline-error evidence missing")
	}
	if tr.FailClosed.Action != "deny" {
		return fmt.Errorf("pipeline error did not deny: %+v", tr.FailClosed)
	}
	return nil
}

// CheckLiveMode rejects anything but a live-mode transcript. A stub-mode
// transcript is offline fixture evidence produced by the session driver; it
// exercises this harness but must never be accepted as live conformance
// evidence.
func CheckLiveMode(tr Transcript) error {
	if tr.Mode != "live" {
		return fmt.Errorf("live conformance requires a mode=live transcript, got mode=%q", tr.Mode)
	}
	return nil
}

// provenanceFile mirrors the linkage fields of the driver's provenance.json.
type provenanceFile struct {
	Mode                string `json:"mode"`
	TranscriptSessionID string `json:"transcript_session_id"`
	TranscriptSHA256    string `json:"transcript_sha256"`
	RawLogFile          string `json:"raw_log_file"`
	RawLogSHA256        string `json:"raw_log_sha256"`
}

// CheckLiveProvenance verifies the transcript ↔ provenance ↔ raw-log linkage
// for a live transcript. transcriptPath points at the sanitized transcript;
// provenance.json and the raw event log must sit in the same directory. The
// check rejects when:
//   - the transcript carries no provenance block or a non-live one;
//   - provenance.json is missing, unreadable, or says mode=stub (or anything
//     but live);
//   - the pseudonymized session ids do not all agree;
//   - the transcript file on disk does not hash to the digest recorded for it
//     (the digest is recomputed from the bytes read from disk by clearing
//     transcript_sha256, re-marshalling, and hashing — the same recipe the
//     driver used to record it), or either document carries no
//     transcript_sha256: for live mode an absent hash is rejected outright
//     (fail closed) rather than skipped;
//   - the SHA-256 recorded for the sanitized raw log (in the transcript and
//     in provenance.json) does not match the raw log file's real digest.
//
// This prevents accidents — picking up a stub or mismatched evidence set — it
// is not tamper evidence: a determined human can still forge the files.
func CheckLiveProvenance(tr Transcript, transcriptPath string) error {
	if tr.Mode != "live" {
		return fmt.Errorf("live conformance requires a mode=live transcript, got mode=%q", tr.Mode)
	}
	if tr.Provenance == nil {
		return fmt.Errorf("live transcript carries no provenance linkage block")
	}
	if tr.Provenance.Mode != "live" {
		return fmt.Errorf("transcript provenance mode=%q, want live", tr.Provenance.Mode)
	}
	if tr.Provenance.SessionID == "" || tr.Provenance.SessionID != tr.SessionID {
		return fmt.Errorf("transcript provenance session id %q does not match transcript session id %q", tr.Provenance.SessionID, tr.SessionID)
	}
	// Both evidence reads are scoped to a root opened on the transcript's
	// directory so only sibling files are reachable.
	dir := filepath.Dir(transcriptPath)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("open evidence directory %q: %w", dir, err)
	}
	defer func() { _ = root.Close() }()
	provBytes, err := root.ReadFile("provenance.json")
	if err != nil {
		return fmt.Errorf("live transcript requires a sibling provenance.json: %w", err)
	}
	var prov provenanceFile
	if err := json.Unmarshal(provBytes, &prov); err != nil {
		return fmt.Errorf("parse provenance.json: %w", err)
	}
	if prov.Mode != "live" {
		return fmt.Errorf("provenance.json mode=%q contradicts live transcript", prov.Mode)
	}
	if prov.TranscriptSessionID == "" || prov.TranscriptSessionID != tr.SessionID {
		return fmt.Errorf("provenance session linkage %q does not match transcript session id %q", prov.TranscriptSessionID, tr.SessionID)
	}
	// The recorded transcript digest is verified against the actual bytes on
	// disk — stored hashes alone say nothing about whether the file was
	// modified after it was written. The digest recipe is the driver's:
	// parse the file, clear transcript_sha256, MarshalIndent, SHA-256.
	transcriptBytes, err := root.ReadFile(filepath.Base(transcriptPath))
	if err != nil {
		return fmt.Errorf("read transcript %q for digest verification: %w", transcriptPath, err)
	}
	onDisk, err := ParseTranscript(transcriptBytes)
	if err != nil {
		return fmt.Errorf("re-parse transcript %q: %w", transcriptPath, err)
	}
	recorded := onDisk.TranscriptHash
	if recorded == "" {
		return fmt.Errorf("transcript file %q carries no recorded transcript_sha256; the digest cannot be verified (fail closed)", transcriptPath)
	}
	onDisk.TranscriptHash = ""
	unsigned, err := json.MarshalIndent(onDisk, "", "  ")
	if err != nil {
		return fmt.Errorf("re-marshal transcript %q: %w", transcriptPath, err)
	}
	trSum := sha256.Sum256(unsigned)
	trActual := hex.EncodeToString(trSum[:])
	if tr.TranscriptHash == "" {
		return fmt.Errorf("live transcript carries no transcript_sha256; the digest cannot be verified (fail closed)")
	}
	if tr.TranscriptHash != trActual {
		return fmt.Errorf("transcript_sha256 %q does not match the digest %q recomputed from the transcript bytes", tr.TranscriptHash, trActual)
	}
	if recorded != trActual {
		return fmt.Errorf("transcript file's recorded digest %q does not match its content digest %q", recorded, trActual)
	}
	if prov.TranscriptSHA256 == "" {
		return fmt.Errorf("provenance.json carries no transcript_sha256; the digest cannot be verified (fail closed)")
	}
	if prov.TranscriptSHA256 != trActual {
		return fmt.Errorf("provenance transcript_sha256 %q does not match the transcript digest %q", prov.TranscriptSHA256, trActual)
	}
	rawName := prov.RawLogFile
	if rawName == "" {
		rawName = "events.raw.jsonl"
	}
	rawBytes, err := root.ReadFile(filepath.Base(rawName))
	if err != nil {
		return fmt.Errorf("live transcript requires the sibling raw event log %q: %w", rawName, err)
	}
	sum := sha256.Sum256(rawBytes)
	actual := hex.EncodeToString(sum[:])
	if tr.Provenance.RawLogSHA256 != actual {
		return fmt.Errorf("transcript raw_log_sha256 %q does not match %s digest %q", tr.Provenance.RawLogSHA256, rawName, actual)
	}
	if prov.RawLogSHA256 != "" && prov.RawLogSHA256 != actual {
		return fmt.Errorf("provenance raw_log_sha256 %q does not match %s digest %q", prov.RawLogSHA256, rawName, actual)
	}
	return nil
}

// CheckSanitized requires sanitized=true, a well-formed optional
// transcript_sha256, and no secret-shaped data in the raw bytes.
func CheckSanitized(tr Transcript, raw []byte) error {
	if !tr.Sanitized {
		return fmt.Errorf("transcript does not declare sanitized=true")
	}
	if HasSecretLikeData(string(raw)) {
		return fmt.Errorf("transcript contains secret-like or raw personal data")
	}
	if tr.TranscriptHash != "" {
		if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(tr.TranscriptHash) {
			return fmt.Errorf("transcript_sha256 must be a lowercase sha256 hex digest, got %q", tr.TranscriptHash)
		}
	}
	return nil
}

func hasEvent(tr Transcript, eventType string) bool {
	for _, event := range tr.Events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func hasConfirmation(tr Transcript, result string) bool {
	for _, c := range tr.Confirmations {
		if c.Result == result {
			return true
		}
	}
	return false
}

func missingDecisionFields(decision decisionRecord) []string {
	var missing []string
	if decision.AgentID == "" {
		missing = append(missing, "agent_id")
	}
	if decision.SessionID == "" {
		missing = append(missing, "session_id")
	}
	if decision.ThreadID == "" {
		missing = append(missing, "thread_id")
	}
	if decision.Tool == "" {
		missing = append(missing, "tool")
	}
	if decision.Action == "" {
		missing = append(missing, "action")
	}
	if decision.Rule == "" {
		missing = append(missing, "rule")
	}
	if decision.Trust == 0 {
		missing = append(missing, "trust")
	}
	return missing
}

// HasSecretLikeData reports whether data contains secret-shaped or raw
// personal content. It is the conformance-side secret scan; the driver runs
// its own broader redactor in addition.
func HasSecretLikeData(data string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)anthropic[_-]?api[_-]?key`),
		regexp.MustCompile(`(?i)bearer\s+[a-z0-9._~+/=-]{12,}`),
		regexp.MustCompile(`sk-ant-[a-zA-Z0-9_-]+`),
		regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
	}
	for _, pattern := range patterns {
		if pattern.MatchString(data) {
			return true
		}
	}
	for _, marker := range []string{"unsanitized", "raw_secret", "api_key", "bearer_token", "session_secret"} {
		if strings.Contains(strings.ToLower(data), marker) {
			return true
		}
	}
	return false
}
