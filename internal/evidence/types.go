package evidence

import "time"

const (
	ManifestSchemaVersion = "boundary.evidence_bundle.v1"
	VerifySchemaVersion   = "boundary.evidence_verify.v1"
)

type Artifact struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	SHA256        string `json:"sha256"`
	SizeBytes     int64  `json:"size_bytes"`
	SchemaVersion string `json:"schema_version,omitempty"`
}

type Manifest struct {
	SchemaVersion       string     `json:"schema_version"`
	CreatedAt           string     `json:"created_at"`
	Source              string     `json:"source"`
	Output              string     `json:"output"`
	Summary             string     `json:"summary"`
	IncludeDemo         bool       `json:"include_demo"`
	RequiresCredentials bool       `json:"requires_credentials"`
	RequiresNetwork     bool       `json:"requires_network"`
	MutatesLiveSystems  bool       `json:"mutates_live_systems"`
	FixtureSafeOutputs  []string   `json:"fixture_safe_outputs"`
	Artifacts           []Artifact `json:"artifacts"`
	Warnings            []string   `json:"warnings,omitempty"`
}

type BundleOptions struct {
	SourceDir   string
	OutDir      string
	IncludeDemo bool
	Now         time.Time
}

type BundleResult struct {
	Manifest     Manifest `json:"manifest"`
	ManifestPath string   `json:"manifest_path"`
}

type VerifyOptions struct {
	BundleDir string
}

type VerifyResult struct {
	SchemaVersion     string        `json:"schema_version"`
	Status            string        `json:"status"`
	Bundle            string        `json:"bundle"`
	ManifestSchema    string        `json:"manifest_schema,omitempty"`
	ArtifactCount     int           `json:"artifact_count"`
	VerifiedArtifacts int           `json:"verified_artifacts"`
	ParsedRecords     int           `json:"parsed_records"`
	Checks            []VerifyCheck `json:"checks"`
}

type VerifyCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}
