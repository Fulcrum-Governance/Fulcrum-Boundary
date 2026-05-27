package commandboundary

import (
	"context"
	"testing"
)

func TestDefaultPreviewPolicyActions(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want string
	}{
		{name: "observe", argv: []string{"pwd"}, want: "allow"},
		{name: "write", argv: []string{"touch", "file"}, want: "warn"},
		{name: "network", argv: []string{"curl", "https://example.invalid"}, want: "require_approval"},
		{name: "repo", argv: []string{"git", "push", "origin", "main"}, want: "require_approval"},
		{name: "destructive", argv: []string{"rm", "-rf", "dist"}, want: "deny"},
		{name: "infra", argv: []string{"docker", "run", "image"}, want: "deny"},
		{name: "secret", argv: []string{"cat", ".env"}, want: "deny"},
		{name: "package", argv: []string{"npm", "install"}, want: "require_approval"},
	}

	pipeline := NewDefaultPreviewPipeline()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification, err := Classify(tt.argv)
			if err != nil {
				t.Fatal(err)
			}
			req := BuildGovernanceRequest(RunRequest{Argv: tt.argv, CWD: t.TempDir()}, classification, HashArgv(tt.argv))
			decision, err := pipeline.Evaluate(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Action != tt.want {
				t.Fatalf("action = %q, want %q", decision.Action, tt.want)
			}
		})
	}
}
