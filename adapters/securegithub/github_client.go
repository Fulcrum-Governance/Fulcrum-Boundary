package securegithub

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type GitHubClient interface {
	GetIssue(ctx context.Context, req LiveIssueRequest) (LiveIssue, error)
	CreateOrUpdateFile(ctx context.Context, req LiveFileMutationRequest) error
}

type LiveIssueRequest struct {
	Owner  string
	Repo   string
	Number int
}

type LiveIssue struct {
	Owner             string
	Repo              string
	Number            int
	URL               string
	AuthorAssociation string
	TitleSHA256       string
	BodySHA256        string
	FetchedAt         time.Time
}

type LiveFileMutationRequest struct {
	Owner   string
	Repo    string
	Path    string
	Message string
	Content string
	Branch  string
}

type RESTGitHubClient struct {
	TokenProvider InstallationTokenProvider
	HTTPClient    *http.Client
	BaseURL       string
	Now           func() time.Time
}

func NewRESTGitHubClient(provider InstallationTokenProvider, baseURL string) *RESTGitHubClient {
	return &RESTGitHubClient{
		TokenProvider: provider,
		BaseURL:       firstNonEmpty(strings.TrimRight(baseURL, "/"), DefaultGitHubAPIBaseURL),
	}
}

func (c *RESTGitHubClient) GetIssue(ctx context.Context, req LiveIssueRequest) (LiveIssue, error) {
	if err := validateIssueRequest(req); err != nil {
		return LiveIssue{}, err
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%d", strings.TrimRight(firstNonEmpty(c.BaseURL, DefaultGitHubAPIBaseURL), "/"), url.PathEscape(req.Owner), url.PathEscape(req.Repo), req.Number)
	httpReq, err := c.newRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return LiveIssue{}, err
	}
	resp, err := httpClient(c.HTTPClient).Do(httpReq)
	if err != nil {
		return LiveIssue{}, fmt.Errorf("read GitHub issue: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LiveIssue{}, fmt.Errorf("read GitHub issue failed: status=%d body=%s", resp.StatusCode, redactCredentialText(string(body)))
	}
	var parsed struct {
		HTMLURL           string `json:"html_url"`
		Title             string `json:"title"`
		Body              string `json:"body"`
		AuthorAssociation string `json:"author_association"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return LiveIssue{}, fmt.Errorf("parse GitHub issue response: %w", err)
	}
	return LiveIssue{
		Owner:             req.Owner,
		Repo:              req.Repo,
		Number:            req.Number,
		URL:               parsed.HTMLURL,
		AuthorAssociation: firstNonEmpty(parsed.AuthorAssociation, "UNKNOWN"),
		TitleSHA256:       sha256Hex(parsed.Title),
		BodySHA256:        sha256Hex(parsed.Body),
		FetchedAt:         c.now(),
	}, nil
}

func (c *RESTGitHubClient) CreateOrUpdateFile(ctx context.Context, req LiveFileMutationRequest) error {
	if strings.TrimSpace(req.Owner) == "" || strings.TrimSpace(req.Repo) == "" || strings.TrimSpace(req.Path) == "" {
		return fmt.Errorf("owner, repo, and path are required for GitHub file mutation")
	}
	payload := map[string]string{
		"message": firstNonEmpty(req.Message, "Boundary conformance mutation"),
		"content": base64.StdEncoding.EncodeToString([]byte(req.Content)),
	}
	if req.Branch != "" {
		payload["branch"] = req.Branch
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/contents/%s", strings.TrimRight(firstNonEmpty(c.BaseURL, DefaultGitHubAPIBaseURL), "/"), url.PathEscape(req.Owner), url.PathEscape(req.Repo), escapePathSegments(req.Path))
	httpReq, err := c.newRequest(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp, err := httpClient(c.HTTPClient).Do(httpReq)
	if err != nil {
		return fmt.Errorf("create or update GitHub file: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("create or update GitHub file failed: status=%d body=%s", resp.StatusCode, redactCredentialText(string(respBody)))
	}
	return nil
}

func (c *RESTGitHubClient) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	if c.TokenProvider == nil {
		return nil, fmt.Errorf("GitHub installation token provider is required")
	}
	token, err := c.TokenProvider.InstallationToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("X-GitHub-Api-Version", gitHubAPIVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *RESTGitHubClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now().UTC()
}

func validateIssueRequest(req LiveIssueRequest) error {
	if strings.TrimSpace(req.Owner) == "" || strings.TrimSpace(req.Repo) == "" {
		return fmt.Errorf("owner and repo are required for GitHub issue read")
	}
	if req.Number <= 0 {
		return fmt.Errorf("GitHub issue number must be positive")
	}
	return nil
}

func issueNumberFromArgs(args map[string]any, fallback int) int {
	for _, key := range []string{"issue_number", "number", "issue"} {
		if args == nil {
			continue
		}
		value, ok := args[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed
		case int64:
			return int(typed)
		case float64:
			return int(typed)
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(typed))
			if err == nil {
				return parsed
			}
		}
	}
	return fallback
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func escapePathSegments(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
