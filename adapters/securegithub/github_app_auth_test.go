package securegithub

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGitHubAppAuth_InstallationTokenExchangeAndCache(t *testing.T) {
	keyPath := writeTestPrivateKey(t)
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/app/installations/456/access_tokens" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != gitHubAPIVersion {
			t.Fatalf("api version = %s, want %s", got, gitHubAPIVersion)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("authorization header missing bearer JWT: %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "ghs_test_token",
			"expires_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
			"permissions": map[string]string{
				"contents": "read",
				"issues":   "read",
			},
		})
	}))
	t.Cleanup(server.Close)

	auth := NewGitHubAppAuth(LiveConfig{
		AppID:          123,
		InstallationID: 456,
		PrivateKeyPath: keyPath,
		APIBaseURL:     server.URL,
	})
	token, err := auth.InstallationToken(context.Background())
	if err != nil {
		t.Fatalf("InstallationToken: %v", err)
	}
	if token.Token == "" || token.Permissions["contents"] != "read" {
		t.Fatalf("unexpected token response: %+v", token)
	}
	if _, err := auth.InstallationToken(context.Background()); err != nil {
		t.Fatalf("cached InstallationToken: %v", err)
	}
	if calls != 1 {
		t.Fatalf("token endpoint calls = %d, want cached single call", calls)
	}
}

func TestLoadLiveConfigFromLookup(t *testing.T) {
	cfg, err := LoadLiveConfigFromLookup(func(key string) string {
		values := map[string]string{
			EnvGitHubConformance:    "true",
			EnvGitHubAppID:          "123",
			EnvGitHubInstallationID: "456",
			EnvGitHubPrivateKeyPath: "/tmp/private-key.pem",
			EnvGitHubOwner:          "fulcrum",
			EnvGitHubRepo:           "boundary",
			EnvGitHubIssueNumber:    "7",
		}
		return values[key]
	})
	if err != nil {
		t.Fatalf("LoadLiveConfigFromLookup: %v", err)
	}
	if !cfg.Enabled || cfg.AppID != 123 || cfg.InstallationID != 456 || cfg.IssueNumber != 7 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.APIBaseURL != DefaultGitHubAPIBaseURL {
		t.Fatalf("api base = %s", cfg.APIBaseURL)
	}
}

func TestLoadLiveConfigFromLookup_MissingRequiredWhenEnabled(t *testing.T) {
	_, err := LoadLiveConfigFromLookup(func(key string) string {
		if key == EnvGitHubConformance {
			return "true"
		}
		return ""
	})
	if err == nil {
		t.Fatal("expected missing env error")
	}
	if !strings.Contains(err.Error(), EnvGitHubAppID) || !strings.Contains(err.Error(), EnvGitHubIssueNumber) {
		t.Fatalf("missing env error omitted required names: %v", err)
	}
	if strings.Contains(err.Error(), "private-key.pem") {
		t.Fatalf("error leaked private key path: %v", err)
	}
}

func writeTestPrivateKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	path := filepath.Join(t.TempDir(), "github-app.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return path
}
