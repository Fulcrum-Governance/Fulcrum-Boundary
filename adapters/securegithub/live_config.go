package securegithub

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	EnvGitHubConformance    = "BOUNDARY_GITHUB_CONFORMANCE"
	EnvGitHubAppID          = "BOUNDARY_GITHUB_APP_ID"
	EnvGitHubInstallationID = "BOUNDARY_GITHUB_INSTALLATION_ID"
	EnvGitHubPrivateKeyPath = "BOUNDARY_GITHUB_PRIVATE_KEY_PATH"
	EnvGitHubOwner          = "BOUNDARY_GITHUB_OWNER"
	EnvGitHubRepo           = "BOUNDARY_GITHUB_REPO"
	EnvGitHubIssueNumber    = "BOUNDARY_GITHUB_ISSUE_NUMBER"
	EnvGitHubAPIBaseURL     = "BOUNDARY_GITHUB_API_BASE_URL"
	EnvGitHubTranscriptDir  = "BOUNDARY_GITHUB_TRANSCRIPT_DIR"
	EnvGitHubTranscript     = "BOUNDARY_GITHUB_TRANSCRIPT"
)

const (
	DefaultGitHubAPIBaseURL    = "https://api.github.com"
	DefaultGitHubTranscriptDir = ".boundary/conformance/secure-github"
)

type LiveConfig struct {
	Enabled        bool
	AppID          int64
	InstallationID int64
	PrivateKeyPath string
	Owner          string
	Repo           string
	IssueNumber    int
	APIBaseURL     string
	TranscriptDir  string
}

func LoadLiveConfigFromEnv() (LiveConfig, error) {
	return LoadLiveConfigFromLookup(os.Getenv)
}

func LoadLiveConfigFromLookup(getenv func(string) string) (LiveConfig, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	if strings.ToLower(strings.TrimSpace(getenv(EnvGitHubConformance))) != "true" {
		return LiveConfig{Enabled: false}, nil
	}
	var missing []string
	required := []string{
		EnvGitHubAppID,
		EnvGitHubInstallationID,
		EnvGitHubPrivateKeyPath,
		EnvGitHubOwner,
		EnvGitHubRepo,
		EnvGitHubIssueNumber,
	}
	for _, key := range required {
		if strings.TrimSpace(getenv(key)) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return LiveConfig{}, fmt.Errorf("secure GitHub live conformance enabled but required env vars are missing: %s", strings.Join(missing, ", "))
	}
	appID, err := parsePositiveInt64Env(EnvGitHubAppID, getenv(EnvGitHubAppID))
	if err != nil {
		return LiveConfig{}, err
	}
	installationID, err := parsePositiveInt64Env(EnvGitHubInstallationID, getenv(EnvGitHubInstallationID))
	if err != nil {
		return LiveConfig{}, err
	}
	issueNumber, err := parsePositiveIntEnv(EnvGitHubIssueNumber, getenv(EnvGitHubIssueNumber))
	if err != nil {
		return LiveConfig{}, err
	}
	apiBaseURL := strings.TrimSpace(getenv(EnvGitHubAPIBaseURL))
	if apiBaseURL == "" {
		apiBaseURL = DefaultGitHubAPIBaseURL
	}
	transcriptDir := strings.TrimSpace(getenv(EnvGitHubTranscriptDir))
	if transcriptDir == "" {
		transcriptDir = DefaultGitHubTranscriptDir
	}
	return LiveConfig{
		Enabled:        true,
		AppID:          appID,
		InstallationID: installationID,
		PrivateKeyPath: strings.TrimSpace(getenv(EnvGitHubPrivateKeyPath)),
		Owner:          strings.TrimSpace(getenv(EnvGitHubOwner)),
		Repo:           strings.TrimSpace(getenv(EnvGitHubRepo)),
		IssueNumber:    issueNumber,
		APIBaseURL:     strings.TrimRight(apiBaseURL, "/"),
		TranscriptDir:  transcriptDir,
	}, nil
}

func parsePositiveInt64Env(name, value string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func parsePositiveIntEnv(name, value string) (int, error) {
	parsed, err := parsePositiveInt64Env(name, value)
	if err != nil {
		return 0, err
	}
	maxInt := int64(1<<(strconv.IntSize-1) - 1)
	if parsed > maxInt {
		return 0, fmt.Errorf("%s must fit in platform int range", name)
	}
	return int(parsed), nil
}

func (c LiveConfig) adapterConfig() Config {
	return Config{
		TenantID:          "live-conformance-tenant",
		AgentID:           "secure-github-live-conformance-agent",
		SessionID:         "secure-github-live-conformance",
		Owner:             c.Owner,
		Repo:              c.Repo,
		OneRepoPerSession: true,
		FixtureMode:       false,
		LiveMode:          true,
		GatewayVersion:    DefaultGateway,
		BuildDigest:       "live-conformance",
	}
}
