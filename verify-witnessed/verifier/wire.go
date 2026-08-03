package verifier

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/sumdb/tlog"
)

const (
	manifestFilename = "manifest.json"
	decisionsFile    = "decisions.jsonl"
	declinesFile     = "declines.jsonl"
	proofFile        = "inclusion-proof-v1.json"
	treeHeadFile     = "tree-head-v1.json"
	cosignaturesFile = "witness-cosignatures-v1.json"
)

var registeredBundleFiles = []string{
	decisionsFile,
	declinesFile,
	proofFile,
	treeHeadFile,
	cosignaturesFile,
}

type manifest struct {
	SchemaVersion string             `json:"schema_version"`
	TenantID      string             `json:"tenant_id"`
	Window        manifestWindow     `json:"window"`
	SourceFilter  string             `json:"source_filter,omitempty"`
	GeneratedAt   string             `json:"generated_at"`
	Exporter      manifestExporter   `json:"exporter"`
	Decisions     manifestCounts     `json:"decisions"`
	DeclinesCount int                `json:"declines_count"`
	DeclinesNote  string             `json:"declines_note"`
	Signing       manifestSigning    `json:"signing"`
	Index         []manifestIndex    `json:"index"`
	WitnessedLog  witnessedLogCounts `json:"witnessed_log"`
	Files         []manifestFile     `json:"files"`
}

type manifestWindow struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type manifestExporter struct {
	ExporterVersion             string `json:"exporter_version"`
	GoVersion                   string `json:"go_version"`
	VCSRevision                 string `json:"vcs_revision,omitempty"`
	VCSModified                 bool   `json:"vcs_modified,omitempty"`
	BoundaryWireContractVersion string `json:"boundary_wire_contract_version"`
}

type manifestCounts struct {
	Total                int            `json:"total"`
	ByVerdict            map[string]int `json:"by_verdict"`
	ByDecisionMode       map[string]int `json:"by_decision_mode"`
	ByVerificationStatus map[string]int `json:"by_verification_status"`
}

type manifestSigning struct {
	AnySigned   bool     `json:"any_signed"`
	SignedCount int      `json:"signed_count"`
	KeyIDs      []string `json:"signature_key_ids,omitempty"`
}

type manifestIndex struct {
	Line               int    `json:"line"`
	Source             string `json:"source"`
	DecisionHash       string `json:"decision_hash,omitempty"`
	Action             string `json:"action,omitempty"`
	DecisionMode       string `json:"decision_mode,omitempty"`
	VerificationStatus string `json:"verification_status"`
	Signed             bool   `json:"signed"`
	SignatureKeyID     string `json:"signature_key_id,omitempty"`
}

type witnessedLogCounts struct {
	ConfiguredWitnesses int `json:"configured_witnesses"`
	PresentCosignatures int `json:"present_cosignatures"`
}

type manifestFile struct {
	Name       string             `json:"name"`
	Lines      int                `json:"lines"`
	LineSHA256 []manifestLineHash `json:"line_sha256"`
}

type manifestLineHash struct {
	Line   int    `json:"line"`
	SHA256 string `json:"sha256"`
}

type inclusionProof struct {
	SchemaVersion     string            `json:"schema_version"`
	LogID             string            `json:"log_id"`
	TreeHeadID        string            `json:"tree_head_id"`
	SourceField       string            `json:"source_field"`
	SourceHash        string            `json:"source_hash"`
	MerkleLeafHash    string            `json:"merkle_leaf_hash"`
	LeafIndex         int64             `json:"leaf_index"`
	TreeSize          int64             `json:"tree_size"`
	AuditPath         []string          `json:"audit_path"`
	DecisionMode      string            `json:"decision_mode"`
	CoverageScope     []string          `json:"coverage_scope"`
	InvariantCitation invariantCitation `json:"invariant_citation"`
	ProofReference    proofReference    `json:"proof_reference"`
}

type invariantCitation struct {
	Theorem       string `json:"theorem"`
	Module        string `json:"module"`
	ProofsTag     string `json:"proofs_tag"`
	TheoremStatus string `json:"theorem_status"`
	RuntimeStatus string `json:"runtime_status"`
}

type proofReference struct {
	DOI                string `json:"doi"`
	TheoremInventoryID string `json:"theorem_inventory_id"`
}

type treeHead struct {
	SchemaVersion  string `json:"schema_version"`
	TreeHeadID     string `json:"tree_head_id"`
	LogID          string `json:"log_id"`
	TenantID       string `json:"tenant_id"`
	TreeSize       int64  `json:"tree_size"`
	RootHash       string `json:"root_hash"`
	IssuedAt       string `json:"issued_at"`
	PrevTreeSize   int64  `json:"prev_tree_size"`
	PrevRootHash   string `json:"prev_root_hash"`
	SignatureKeyID string `json:"signature_key_id"`
	Signature      string `json:"signature"`
}

type cosignatureAggregate struct {
	SchemaVersion       string               `json:"schema_version"`
	TreeHeadID          string               `json:"tree_head_id"`
	ConfiguredWitnesses []configuredWitness  `json:"configured_witnesses"`
	PresentCosignatures int                  `json:"present_cosignatures"`
	Cosignatures        []witnessCosignature `json:"cosignatures"`
}

type configuredWitness struct {
	WitnessID    string `json:"witness_id"`
	WitnessKeyID string `json:"witness_key_id"`
}

type witnessCosignature struct {
	SchemaVersion string `json:"schema_version"`
	WitnessID     string `json:"witness_id"`
	LogID         string `json:"log_id"`
	TreeSize      int64  `json:"tree_size"`
	RootHash      string `json:"root_hash"`
	FulcrumKeyID  string `json:"fulcrum_key_id"`
	CosignedAt    string `json:"cosigned_at"`
	Cosignature   string `json:"cosignature"`
}

type decisionRecordV1 struct {
	SchemaVersion       string  `json:"schema_version"`
	EventType           string  `json:"event_type,omitempty"`
	RecordID            string  `json:"record_id"`
	Timestamp           string  `json:"timestamp"`
	BoundaryVersion     string  `json:"boundary_version,omitempty"`
	BoundaryBuildDigest string  `json:"boundary_build_digest,omitempty"`
	Adapter             string  `json:"adapter,omitempty"`
	AgentID             string  `json:"agent_id,omitempty"`
	TenantID            string  `json:"tenant_id,omitempty"`
	TraceID             string  `json:"trace_id,omitempty"`
	Tool                string  `json:"tool,omitempty"`
	Action              string  `json:"action"`
	Reason              string  `json:"reason,omitempty"`
	DecisionMode        string  `json:"decision_mode,omitempty"`
	MatchedRule         string  `json:"matched_rule,omitempty"`
	PolicyFile          string  `json:"policy_file,omitempty"`
	PolicyBundleHash    string  `json:"policy_bundle_hash,omitempty"`
	RequestHash         string  `json:"request_hash,omitempty"`
	RawShapeHash        string  `json:"raw_shape_hash,omitempty"`
	DecisionHash        string  `json:"decision_hash"`
	TrustScore          float64 `json:"trust_score"`
	TrustState          string  `json:"trust_state,omitempty"`
	Signature           string  `json:"signature,omitempty"`
	SignatureKeyID      string  `json:"signature_key_id,omitempty"`
}

func decodeManifest(data []byte) (manifest, error) {
	var value manifest
	err := decodeStrictRequired(data, &value,
		"schema_version", "tenant_id", "window", "generated_at", "exporter",
		"decisions", "declines_count", "declines_note", "signing", "index",
		"witnessed_log", "files",
	)
	if err != nil {
		return manifest{}, err
	}
	if err := value.validate(); err != nil {
		return manifest{}, err
	}
	return value, nil
}

func (m manifest) validate() error {
	if m.SchemaVersion != "fulcrum.receipt_bundle_manifest.v1" {
		return fmt.Errorf("manifest schema_version = %q", m.SchemaVersion)
	}
	if err := requireNonempty("manifest tenant_id", m.TenantID); err != nil {
		return err
	}
	from, err := parseCanonicalUTC("manifest window.from", m.Window.From)
	if err != nil {
		return err
	}
	to, err := parseCanonicalUTC("manifest window.to", m.Window.To)
	if err != nil {
		return err
	}
	if !from.Before(to) {
		return fmt.Errorf("manifest window must satisfy from < to")
	}
	if _, err := parseCanonicalUTC("manifest generated_at", m.GeneratedAt); err != nil {
		return err
	}
	if m.Exporter.ExporterVersion == "" || m.Exporter.GoVersion == "" || m.Exporter.BoundaryWireContractVersion == "" {
		return fmt.Errorf("manifest exporter required strings must be non-empty")
	}
	if err := m.Decisions.validate(); err != nil {
		return err
	}
	if m.DeclinesCount < 0 || m.DeclinesNote == "" {
		return fmt.Errorf("manifest decline count/note is invalid")
	}
	if err := m.Signing.validate(m.Decisions.Total); err != nil {
		return err
	}
	if m.Index == nil || len(m.Index) != m.Decisions.Total {
		return fmt.Errorf("manifest index length = %d, want %d", len(m.Index), m.Decisions.Total)
	}
	for i, entry := range m.Index {
		if entry.Line != i+1 || entry.Source == "" || entry.VerificationStatus == "" {
			return fmt.Errorf("manifest index entry %d is invalid", i+1)
		}
		if entry.DecisionHash != "" {
			if _, err := parseHash(entry.DecisionHash); err != nil {
				return fmt.Errorf("manifest index entry %d decision_hash: %w", i+1, err)
			}
		}
		if entry.SignatureKeyID != "" {
			if err := validateKeyID(entry.SignatureKeyID); err != nil {
				return fmt.Errorf("manifest index entry %d signature_key_id: %w", i+1, err)
			}
		}
	}
	if m.WitnessedLog.ConfiguredWitnesses < 0 || m.WitnessedLog.PresentCosignatures < 0 ||
		m.WitnessedLog.PresentCosignatures > m.WitnessedLog.ConfiguredWitnesses {
		return fmt.Errorf("manifest witnessed_log counts are invalid")
	}
	if len(m.Files) != len(registeredBundleFiles) {
		return fmt.Errorf("manifest files length = %d, want %d", len(m.Files), len(registeredBundleFiles))
	}
	for i, file := range m.Files {
		if file.Name != registeredBundleFiles[i] {
			return fmt.Errorf("manifest files[%d].name = %q, want %q", i, file.Name, registeredBundleFiles[i])
		}
		if file.Lines < 0 || file.LineSHA256 == nil || len(file.LineSHA256) != file.Lines {
			return fmt.Errorf("manifest registration for %s has invalid line counts", file.Name)
		}
		for j, line := range file.LineSHA256 {
			if line.Line != j+1 || !isLowerHexDigest(line.SHA256) {
				return fmt.Errorf("manifest registration for %s line %d is invalid", file.Name, j+1)
			}
		}
	}
	if m.Files[1].Lines != 0 || m.DeclinesCount != 0 {
		return fmt.Errorf("declines.jsonl must be registered as zero lines")
	}
	for _, file := range m.Files[2:] {
		if file.Lines != 1 {
			return fmt.Errorf("manifest registration for %s must contain one line", file.Name)
		}
	}
	return nil
}

func (c manifestCounts) validate() error {
	if c.Total < 0 || c.ByVerdict == nil || c.ByDecisionMode == nil || c.ByVerificationStatus == nil {
		return fmt.Errorf("manifest decision counts are invalid")
	}
	for name, values := range map[string]map[string]int{
		"by_verdict": c.ByVerdict, "by_decision_mode": c.ByDecisionMode, "by_verification_status": c.ByVerificationStatus,
	} {
		total := 0
		for _, count := range values {
			if count < 0 {
				return fmt.Errorf("manifest decisions.%s contains a negative count", name)
			}
			total += count
		}
		if total != c.Total {
			return fmt.Errorf("manifest decisions.%s sums to %d, want %d", name, total, c.Total)
		}
	}
	return nil
}

func (s manifestSigning) validate(total int) error {
	if s.SignedCount < 0 || s.SignedCount > total || s.AnySigned != (s.SignedCount > 0) {
		return fmt.Errorf("manifest signing state is inconsistent")
	}
	if s.KeyIDs != nil {
		if err := validateSortedUnique("manifest signing signature_key_ids", s.KeyIDs, false); err != nil {
			return err
		}
		for _, id := range s.KeyIDs {
			if err := validateKeyID(id); err != nil {
				return fmt.Errorf("manifest signing signature_key_ids: %w", err)
			}
		}
	}
	return nil
}

func decodeInclusionProof(data []byte) (inclusionProof, error) {
	var proof inclusionProof
	err := decodeCompactJSONLine(data, &proof,
		"schema_version", "log_id", "tree_head_id", "source_field", "source_hash",
		"merkle_leaf_hash", "leaf_index", "tree_size", "audit_path", "decision_mode",
		"coverage_scope", "invariant_citation", "proof_reference",
	)
	if err != nil {
		return inclusionProof{}, err
	}
	if err := proof.validate(); err != nil {
		return inclusionProof{}, err
	}
	return proof, nil
}

func (p inclusionProof) validate() error {
	if p.SchemaVersion != "inclusion-proof-v1" || p.LogID == "" || p.TreeHeadID == "" {
		return fmt.Errorf("inclusion proof version or required IDs are invalid")
	}
	// Per-decision receipt bundles must bind the signed leaf to the exported
	// DecisionRecord. A payload_sha leaf is valid for generic capture integrity,
	// but cannot establish that relationship from this wire format.
	if p.SourceField != "decision_hash" {
		return fmt.Errorf("inclusion proof source_field = %q", p.SourceField)
	}
	if _, err := parseHash(p.SourceHash); err != nil {
		return fmt.Errorf("inclusion proof source_hash: %w", err)
	}
	if _, err := parseHash(p.MerkleLeafHash); err != nil {
		return fmt.Errorf("inclusion proof merkle_leaf_hash: %w", err)
	}
	if p.TreeSize <= 0 || p.LeafIndex < 0 || p.LeafIndex >= p.TreeSize {
		return fmt.Errorf("inclusion proof tree_size/leaf_index relationship is invalid")
	}
	if p.AuditPath == nil {
		return fmt.Errorf("inclusion proof audit_path must be an array")
	}
	for i, hash := range p.AuditPath {
		if _, err := parseHash(hash); err != nil {
			return fmt.Errorf("inclusion proof audit_path[%d]: %w", i, err)
		}
	}
	if !validDecisionMode(p.DecisionMode) {
		return fmt.Errorf("inclusion proof decision_mode = %q", p.DecisionMode)
	}
	if err := validateSortedUnique("inclusion proof coverage_scope", p.CoverageScope, true); err != nil {
		return err
	}
	citation := p.InvariantCitation
	if citation.Theorem == "" || citation.Module == "" || citation.ProofsTag != "v0.2.0" || citation.TheoremStatus != "Proved" ||
		(citation.RuntimeStatus != "Implemented" && citation.RuntimeStatus != "Conjectured") {
		return fmt.Errorf("inclusion proof invariant_citation is invalid")
	}
	switch citation.Theorem {
	case "high_risk_execution_requires_stronger_trust", "high_risk_execution_kernel_guarantee":
		if citation.RuntimeStatus != "Conjectured" {
			return fmt.Errorf("inclusion proof high-risk invariant runtime_status must be Conjectured")
		}
	}
	if p.ProofReference.DOI != "10.5281/zenodo.19900714" || p.ProofReference.TheoremInventoryID == "" {
		return fmt.Errorf("inclusion proof proof_reference is invalid")
	}
	return nil
}

func decodeTreeHead(data []byte) (treeHead, error) {
	var head treeHead
	err := decodeCompactJSONLine(data, &head,
		"schema_version", "tree_head_id", "log_id", "tenant_id", "tree_size", "root_hash",
		"issued_at", "prev_tree_size", "prev_root_hash", "signature_key_id", "signature",
	)
	if err != nil {
		return treeHead{}, err
	}
	if err := head.validate(); err != nil {
		return treeHead{}, err
	}
	return head, nil
}

func (h treeHead) validate() error {
	if h.SchemaVersion != "tree-head-v1" || h.TreeHeadID == "" || h.LogID == "" || h.TenantID == "" {
		return fmt.Errorf("tree head version or required IDs are invalid")
	}
	if h.TreeSize <= 0 || h.PrevTreeSize < 0 || h.PrevTreeSize > h.TreeSize {
		return fmt.Errorf("tree head size relationship is invalid")
	}
	if _, err := parseHash(h.RootHash); err != nil {
		return fmt.Errorf("tree head root_hash: %w", err)
	}
	if _, err := parseCanonicalUTC("tree head issued_at", h.IssuedAt); err != nil {
		return err
	}
	if h.PrevTreeSize == 0 {
		if h.PrevRootHash != "" {
			return fmt.Errorf("tree head prev_root_hash must be empty at size zero")
		}
	} else if _, err := parseHash(h.PrevRootHash); err != nil {
		return fmt.Errorf("tree head prev_root_hash: %w", err)
	}
	if err := validateKeyID(h.SignatureKeyID); err != nil {
		return fmt.Errorf("tree head signature_key_id: %w", err)
	}
	if _, err := parseEd25519Value(h.Signature, 64, "tree head signature"); err != nil {
		return err
	}
	return nil
}

func decodeCosignatureAggregate(data []byte) (cosignatureAggregate, error) {
	var aggregate cosignatureAggregate
	err := decodeCompactJSONLine(data, &aggregate,
		"schema_version", "tree_head_id", "configured_witnesses", "present_cosignatures", "cosignatures",
	)
	if err != nil {
		return cosignatureAggregate{}, err
	}
	if err := aggregate.validate(); err != nil {
		return cosignatureAggregate{}, err
	}
	return aggregate, nil
}

func (a cosignatureAggregate) validate() error {
	if a.SchemaVersion != "witness-cosignatures-v1" || a.TreeHeadID == "" {
		return fmt.Errorf("cosignature aggregate version or tree_head_id is invalid")
	}
	if a.ConfiguredWitnesses == nil || a.Cosignatures == nil {
		return fmt.Errorf("cosignature aggregate arrays must not be null")
	}
	if a.PresentCosignatures != len(a.Cosignatures) || len(a.Cosignatures) > len(a.ConfiguredWitnesses) {
		return fmt.Errorf("cosignature aggregate present count is invalid")
	}
	configuredIndex := make(map[string]int, len(a.ConfiguredWitnesses))
	keyIDs := make(map[string]struct{}, len(a.ConfiguredWitnesses))
	previousID := ""
	for i, configured := range a.ConfiguredWitnesses {
		if configured.WitnessID == "" || (i > 0 && configured.WitnessID <= previousID) {
			return fmt.Errorf("configured witnesses must have unique sorted witness IDs")
		}
		if err := validateKeyID(configured.WitnessKeyID); err != nil {
			return fmt.Errorf("configured witness %q key ID: %w", configured.WitnessID, err)
		}
		if _, exists := keyIDs[configured.WitnessKeyID]; exists {
			return fmt.Errorf("configured witness key ID %q is duplicated", configured.WitnessKeyID)
		}
		configuredIndex[configured.WitnessID] = i
		keyIDs[configured.WitnessKeyID] = struct{}{}
		previousID = configured.WitnessID
	}
	lastIndex := -1
	seenCosignatures := make(map[string]struct{}, len(a.Cosignatures))
	for i, item := range a.Cosignatures {
		if err := item.validate(); err != nil {
			return fmt.Errorf("cosignature %d: %w", i, err)
		}
		index, ok := configuredIndex[item.WitnessID]
		if !ok || index <= lastIndex {
			return fmt.Errorf("cosignatures must be a unique configured-witness subsequence")
		}
		if _, exists := seenCosignatures[item.WitnessID]; exists {
			return fmt.Errorf("cosignature witness ID %q is duplicated", item.WitnessID)
		}
		seenCosignatures[item.WitnessID] = struct{}{}
		lastIndex = index
	}
	return nil
}

func (w witnessCosignature) validate() error {
	if w.SchemaVersion != "witness-cosignature-v1" || w.WitnessID == "" || w.LogID == "" || w.TreeSize <= 0 {
		return fmt.Errorf("version, required IDs, or tree_size is invalid")
	}
	if _, err := parseHash(w.RootHash); err != nil {
		return fmt.Errorf("root_hash: %w", err)
	}
	if err := validateKeyID(w.FulcrumKeyID); err != nil {
		return fmt.Errorf("fulcrum_key_id: %w", err)
	}
	if _, err := parseCanonicalUTC("cosigned_at", w.CosignedAt); err != nil {
		return err
	}
	if _, err := parseEd25519Value(w.Cosignature, 64, "witness cosignature"); err != nil {
		return err
	}
	return nil
}

func decodeDecisionRecord(data []byte) (decisionRecordV1, error) {
	var record decisionRecordV1
	err := decodeStrictRequired(data, &record,
		"schema_version", "record_id", "timestamp", "action", "decision_hash", "trust_score",
	)
	if err != nil {
		return decisionRecordV1{}, err
	}
	if err := record.validate(); err != nil {
		return decisionRecordV1{}, err
	}
	return record, nil
}

func (r decisionRecordV1) validate() error {
	if r.SchemaVersion != "1" {
		return fmt.Errorf("decision record schema_version = %q, only pinned version %q is supported", r.SchemaVersion, "1")
	}
	if r.RecordID == "" || r.Timestamp == "" || r.Action == "" {
		return fmt.Errorf("decision record required strings must be non-empty")
	}
	if _, err := parseCanonicalUTC("decision record timestamp", r.Timestamp); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"decision_hash":      r.DecisionHash,
		"policy_bundle_hash": r.PolicyBundleHash,
		"request_hash":       r.RequestHash,
		"raw_shape_hash":     r.RawShapeHash,
	} {
		if value == "" && name != "decision_hash" {
			continue
		}
		if _, err := parseHash(value); err != nil {
			return fmt.Errorf("decision record %s: %w", name, err)
		}
	}
	if r.DecisionMode != "" && !validDecisionMode(r.DecisionMode) {
		return fmt.Errorf("decision record decision_mode = %q", r.DecisionMode)
	}
	if (r.Signature == "") != (r.SignatureKeyID == "") {
		return fmt.Errorf("decision record signature and signature_key_id must appear together")
	}
	if r.Signature != "" {
		if err := validateKeyID(r.SignatureKeyID); err != nil {
			return fmt.Errorf("decision record signature_key_id: %w", err)
		}
		if _, err := parseEd25519Value(r.Signature, 64, "decision record signature"); err != nil {
			return err
		}
	}
	return nil
}

func parseHash(value string) (tlog.Hash, error) {
	const prefix = "sha256:"
	var hash tlog.Hash
	if !strings.HasPrefix(value, prefix) || !isLowerHexDigest(strings.TrimPrefix(value, prefix)) {
		return hash, fmt.Errorf("hash %q must use sha256:<64 lowercase hex>", value)
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return hash, fmt.Errorf("decode hash: %w", err)
	}
	copy(hash[:], decoded)
	return hash, nil
}

func isLowerHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func parseCanonicalUTC(label, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || !strings.HasSuffix(value, "Z") || parsed.UTC().Format(time.RFC3339Nano) != value {
		return time.Time{}, fmt.Errorf("%s %q is not canonical UTC RFC3339Nano", label, value)
	}
	return parsed, nil
}

func validateSortedUnique(label string, values []string, nonempty bool) error {
	if values == nil || (nonempty && len(values) == 0) {
		return fmt.Errorf("%s must be a%s array", label, map[bool]string{true: " non-empty", false: "n"}[nonempty])
	}
	if !sort.StringsAreSorted(values) {
		return fmt.Errorf("%s must be sorted", label)
	}
	for i, value := range values {
		if value == "" || (i > 0 && value == values[i-1]) {
			return fmt.Errorf("%s must contain unique non-empty strings", label)
		}
	}
	return nil
}

func validDecisionMode(value string) bool {
	switch value {
	case "proved", "deterministic", "classified", "human_approved":
		return true
	default:
		return false
	}
}

func requireNonempty(label, value string) error {
	if value == "" {
		return fmt.Errorf("%s must be non-empty", label)
	}
	return nil
}
