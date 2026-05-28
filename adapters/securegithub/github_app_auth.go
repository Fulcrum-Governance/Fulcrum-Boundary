package securegithub

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const gitHubAPIVersion = "2026-03-10"

type InstallationToken struct {
	Token       string            `json:"token"`
	ExpiresAt   time.Time         `json:"expires_at"`
	Permissions map[string]string `json:"permissions,omitempty"`
}

type InstallationTokenProvider interface {
	InstallationToken(ctx context.Context) (InstallationToken, error)
}

type GitHubAppAuth struct {
	AppID          int64
	InstallationID int64
	PrivateKeyPath string
	HTTPClient     *http.Client
	BaseURL        string
	Now            func() time.Time

	mu     sync.Mutex
	cached InstallationToken
}

func NewGitHubAppAuth(cfg LiveConfig) *GitHubAppAuth {
	return &GitHubAppAuth{
		AppID:          cfg.AppID,
		InstallationID: cfg.InstallationID,
		PrivateKeyPath: cfg.PrivateKeyPath,
		BaseURL:        firstNonEmpty(cfg.APIBaseURL, DefaultGitHubAPIBaseURL),
	}
}

func (a *GitHubAppAuth) InstallationToken(ctx context.Context) (InstallationToken, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.now()
	if a.cached.Token != "" && a.cached.ExpiresAt.After(now.Add(5*time.Minute)) {
		return a.cached, nil
	}
	jwt, err := a.GenerateJWT(now)
	if err != nil {
		return InstallationToken{}, err
	}
	token, err := a.exchangeInstallationToken(ctx, jwt)
	if err != nil {
		return InstallationToken{}, err
	}
	a.cached = token
	return token, nil
}

func (a *GitHubAppAuth) GenerateJWT(now time.Time) (string, error) {
	if a.AppID <= 0 {
		return "", fmt.Errorf("GitHub App ID is required")
	}
	key, err := readRSAPrivateKey(a.PrivateKeyPath)
	if err != nil {
		return "", err
	}
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	payload := map[string]any{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": fmt.Sprintf("%d", a.AppID),
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := encodedHeader + "." + encodedPayload
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign GitHub App JWT: %w", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (a *GitHubAppAuth) exchangeInstallationToken(ctx context.Context, jwt string) (InstallationToken, error) {
	if a.InstallationID <= 0 {
		return InstallationToken{}, fmt.Errorf("GitHub App installation ID is required")
	}
	endpoint := fmt.Sprintf("%s/app/installations/%d/access_tokens", strings.TrimRight(firstNonEmpty(a.BaseURL, DefaultGitHubAPIBaseURL), "/"), a.InstallationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		return InstallationToken{}, fmt.Errorf("create GitHub App token request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-GitHub-Api-Version", gitHubAPIVersion)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient(a.HTTPClient).Do(req)
	if err != nil {
		return InstallationToken{}, fmt.Errorf("exchange GitHub App installation token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return InstallationToken{}, fmt.Errorf("exchange GitHub App installation token failed: status=%d body=%s", resp.StatusCode, redactCredentialText(string(body)))
	}
	var token InstallationToken
	if err := json.Unmarshal(body, &token); err != nil {
		return InstallationToken{}, fmt.Errorf("parse GitHub App installation token response: %w", err)
	}
	if token.Token == "" {
		return InstallationToken{}, fmt.Errorf("GitHub App installation token response missing token")
	}
	if token.ExpiresAt.IsZero() {
		return InstallationToken{}, fmt.Errorf("GitHub App installation token response missing expires_at")
	}
	return token, nil
}

func readRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("GitHub App private key path is required")
	}
	data, err := os.ReadFile(path) // #nosec G304 -- GitHub App private key path is an explicit operator-owned conformance input.
	if err != nil {
		return nil, fmt.Errorf("read GitHub App private key from configured path: %s", sanitizedFileError(err))
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("parse GitHub App private key: PEM block not found")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse GitHub App private key: unsupported RSA private key format")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("parse GitHub App private key: key is not RSA")
	}
	return key, nil
}

func sanitizedFileError(err error) string {
	if os.IsNotExist(err) {
		return "file not found"
	}
	if os.IsPermission(err) {
		return "permission denied"
	}
	return "read failed"
}

func httpClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return http.DefaultClient
}

func (a *GitHubAppAuth) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now().UTC()
}
