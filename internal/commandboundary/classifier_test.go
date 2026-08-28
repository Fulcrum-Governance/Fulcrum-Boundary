package commandboundary

import (
	"strings"
	"testing"
)

func TestClassifyCommandTaxonomy(t *testing.T) {
	tests := []struct {
		name   string
		argv   []string
		class  Class
		risk   Risk
		action RecommendedAction
		reason string
	}{
		{
			name:   "git status observes repository",
			argv:   []string{"git", "status"},
			class:  ClassObserveRead,
			risk:   RiskLow,
			action: ActionAllow,
			reason: "repository observation",
		},
		{
			name:   "git push mutates external repository",
			argv:   []string{"git", "push", "origin", "main"},
			class:  ClassRepositoryMutation,
			risk:   RiskHigh,
			action: ActionRequireApproval,
			reason: "external repository mutation",
		},
		{
			name:   "rm is destructive",
			argv:   []string{"rm", "-rf", "dist"},
			class:  ClassDestructiveMutation,
			risk:   RiskCritical,
			action: ActionDeny,
			reason: "destructive local mutation",
		},
		{
			name:   "cat env is secret access",
			argv:   []string{"cat", ".env"},
			class:  ClassCredentialAccess,
			risk:   RiskCritical,
			action: ActionDeny,
			reason: "credential or secret access",
		},
		{
			name:   "npm install runs lifecycle",
			argv:   []string{"npm", "install"},
			class:  ClassPackageLifecycle,
			risk:   RiskHigh,
			action: ActionRequireApproval,
			reason: "package lifecycle execution",
		},
		{
			name:   "docker run mutates runtime",
			argv:   []string{"docker", "run", "image"},
			class:  ClassInfrastructureMutation,
			risk:   RiskCritical,
			action: ActionDeny,
			reason: "runtime mutation",
		},
		{
			name:   "kubectl apply mutates infrastructure",
			argv:   []string{"kubectl", "apply", "-f", "deploy.yaml"},
			class:  ClassInfrastructureMutation,
			risk:   RiskCritical,
			action: ActionDeny,
			reason: "infrastructure mutation",
		},
		{
			name:   "terraform apply mutates infrastructure",
			argv:   []string{"terraform", "apply", "-auto-approve"},
			class:  ClassInfrastructureMutation,
			risk:   RiskCritical,
			action: ActionDeny,
			reason: "infrastructure mutation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Classify(tt.argv)
			if err != nil {
				t.Fatalf("Classify returned error: %v", err)
			}
			if got.SchemaVersion != SchemaVersionClassification {
				t.Fatalf("schema version = %q", got.SchemaVersion)
			}
			if got.Class != tt.class || got.Risk != tt.risk || got.RecommendedAction != tt.action || got.Reason != tt.reason {
				t.Fatalf("classification = %#v", got)
			}
		})
	}
}

func TestClassifyRedactsSecretArguments(t *testing.T) {
	got, err := Classify([]string{"curl", "--token", "abc123", "-H", "Authorization: bearer abc123", "-d", "@.env", "https://example.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != ClassCredentialAccess {
		t.Fatalf("class = %s, want %s", got.Class, ClassCredentialAccess)
	}
	for _, arg := range got.ArgsRedacted {
		if arg == "abc123" || arg == "@.env" || arg == "Authorization: bearer abc123" {
			t.Fatalf("secret argument was not redacted: %#v", got.ArgsRedacted)
		}
	}
}

// TestClassifyRedactsInlineURLCredentials covers the canonical secret shape on a
// command line: credentials embedded in a connection string. Nothing else in the
// redaction table matches `scheme://user:pass@host`, so the password used to
// survive into the classification, the operator-facing reason, and the persisted
// decision record.
func TestClassifyRedactsInlineURLCredentials(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want []string
	}{
		{
			name: "dsn keeps the account and drops the password",
			argv: []string{"psql", "postgres://admin:hunter2@db.internal/prod", "-c", "select 1"},
			want: []string{"postgres://admin:[redacted]@db.internal/prod", "-c", "select 1"},
		},
		{
			name: "bare userinfo is a token and is dropped whole",
			argv: []string{"git", "clone", "https://ghp_A1b2C3d4@github.com/o/r.git"},
			want: []string{"clone", "https://[redacted]@github.com/o/r.git"},
		},
		{
			name: "flag value carrying a dsn",
			argv: []string{"myapp", "--dsn=mysql://root:s3cr3t@localhost:3306/app"},
			want: []string{"--dsn=mysql://root:[redacted]@localhost:3306/app"},
		},
		{
			name: "scp-style remote has no userinfo to redact",
			argv: []string{"git", "clone", "git@github.com:o/r.git"},
			want: []string{"clone", "git@github.com:o/r.git"},
		},
		{
			name: "an at sign in the path is not userinfo",
			argv: []string{"curl", "https://example.invalid/a@b"},
			want: []string{"https://example.invalid/a@b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Classify(tt.argv)
			if err != nil {
				t.Fatalf("Classify(%v): %v", tt.argv, err)
			}
			if len(got.ArgsRedacted) != len(tt.want) {
				t.Fatalf("args = %#v, want %#v", got.ArgsRedacted, tt.want)
			}
			for i, want := range tt.want {
				if got.ArgsRedacted[i] != want {
					t.Fatalf("args[%d] = %q, want %q", i, got.ArgsRedacted[i], want)
				}
			}
			for _, arg := range got.ArgsRedacted {
				for _, secret := range []string{"hunter2", "ghp_A1b2C3d4", "s3cr3t"} {
					if strings.Contains(arg, secret) {
						t.Fatalf("secret %q survived redaction in %q", secret, arg)
					}
				}
			}
		})
	}
}

func TestClassifyRejectsMissingCommand(t *testing.T) {
	if _, err := Classify(nil); err == nil {
		t.Fatal("expected missing command to fail")
	}
}
