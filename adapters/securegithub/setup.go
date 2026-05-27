package securegithub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type SetupResult struct {
	Directory string
	Profile   string
	Policy    string
}

type setupProfile struct {
	SchemaVersion      string           `json:"schema_version"`
	CreatedAt          string           `json:"created_at"`
	ProfileID          string           `json:"profile_id"`
	Status             string           `json:"status"`
	FixtureMode        bool             `json:"fixture_mode"`
	LiveGitHubEvidence bool             `json:"live_github_evidence"`
	Owner              string           `json:"owner"`
	Repo               string           `json:"repo"`
	OneRepoPerSession  bool             `json:"one_repo_per_session"`
	Tools              []toolDescriptor `json:"tools"`
	Notes              []string         `json:"notes"`
}

type toolDescriptor struct {
	Name            string `json:"name"`
	CapabilityClass string `json:"capability_class"`
	SourceClass     string `json:"source_class"`
	TargetSink      string `json:"target_sink"`
	MutationClass   string `json:"mutation_class"`
}

func WriteSetup(outDir string, cfg Config) (*SetupResult, error) {
	cfg = cfg.withDefaults()
	if outDir == "" {
		outDir = ".boundary/secure-github"
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absOut, 0o700); err != nil {
		return nil, fmt.Errorf("create Secure GitHub setup directory: %w", err)
	}
	profile := setupProfile{
		SchemaVersion:      "boundary.secure_github.profile.v1",
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
		ProfileID:          ProfileID,
		Status:             StatusPreview,
		FixtureMode:        true,
		LiveGitHubEvidence: false,
		Owner:              cfg.Owner,
		Repo:               cfg.Repo,
		OneRepoPerSession:  true,
		Tools:              setupTools(),
		Notes: []string{
			"Preview fixture profile; live GitHub App conformance evidence is not present.",
			"Protected W1/W2 private-repo mutations after taint deny before upstream execution.",
			"Direct GitHub API or upstream MCP access remains a bypass unless deployment topology removes it.",
		},
	}
	profilePath := filepath.Join(absOut, "secure-github-profile.json")
	body, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(profilePath, append(body, '\n'), 0o600); err != nil {
		return nil, fmt.Errorf("write Secure GitHub profile: %w", err)
	}
	policyPath := filepath.Join(absOut, "secure-github-policy.json")
	policyBody, err := json.MarshalIndent(DefaultPolicyRules(), "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(policyPath, append(policyBody, '\n'), 0o600); err != nil {
		return nil, fmt.Errorf("write Secure GitHub policies: %w", err)
	}
	return &SetupResult{Directory: absOut, Profile: profilePath, Policy: policyPath}, nil
}

func setupTools() []toolDescriptor {
	return []toolDescriptor{
		{Name: "get_issue", CapabilityClass: "R0", SourceClass: "external_collaborator", TargetSink: "none", MutationClass: "none"},
		{Name: "get_pull_request", CapabilityClass: "R0", SourceClass: "external_collaborator", TargetSink: "none", MutationClass: "none"},
		{Name: "get_file_contents", CapabilityClass: "R0", SourceClass: "allowlisted_resource", TargetSink: "none", MutationClass: "none"},
		{Name: "create_issue", CapabilityClass: "W0", SourceClass: "agent_generated", TargetSink: "private_repo", MutationClass: "issue_or_pr_create"},
		{Name: "create_pull_request", CapabilityClass: "W0", SourceClass: "agent_generated", TargetSink: "private_repo", MutationClass: "issue_or_pr_create"},
		{Name: "create_or_update_file", CapabilityClass: "W1", SourceClass: "agent_generated", TargetSink: "private_repo", MutationClass: "private_repo_content_write"},
		{Name: "push_files", CapabilityClass: "W1", SourceClass: "agent_generated", TargetSink: "private_repo", MutationClass: "private_repo_content_write"},
		{Name: "merge_pull_request", CapabilityClass: "W2", SourceClass: "agent_generated", TargetSink: "private_repo", MutationClass: "merge_or_release"},
	}
}
