package editboundary

import (
	"fmt"
	"strings"
	"testing"
)

func TestInspectPatchClassifiesE0ThroughE7(t *testing.T) {
	tests := []struct {
		name   string
		patch  string
		class  Class
		action RecommendedAction
	}{
		{
			name:   "empty patch is no-op",
			patch:  "",
			class:  ClassNoop,
			action: ActionAllow,
		},
		{
			name: "docs edit is safe content",
			patch: `diff --git a/docs/example.md b/docs/example.md
--- a/docs/example.md
+++ b/docs/example.md
@@ -1 +1,2 @@
 # Example
+More detail.
`,
			class:  ClassSafeContent,
			action: ActionAllow,
		},
		{
			name: "source edit requires approval",
			patch: `diff --git a/src/app.ts b/src/app.ts
--- a/src/app.ts
+++ b/src/app.ts
@@ -1 +1,2 @@
 export const ok = true
+export const changed = true
`,
			class:  ClassSourceConfig,
			action: ActionRequireApproval,
		},
		{
			name: "terraform edit requires approval",
			patch: `diff --git a/infra/main.tf b/infra/main.tf
--- a/infra/main.tf
+++ b/infra/main.tf
@@ -1 +1,2 @@
 resource "x" "y" {}
+resource "x" "z" {}
`,
			class:  ClassDeploymentInfra,
			action: ActionRequireApproval,
		},
		{
			name: "secret path denies and redacts",
			patch: `diff --git a/.env b/.env
new file mode 100644
--- /dev/null
+++ b/.env
@@ -0,0 +1 @@
+API_KEY=secret-value
`,
			class:  ClassSecretBearing,
			action: ActionDeny,
		},
		{
			name: "deleted file denies",
			patch: `diff --git a/src/app.ts b/src/app.ts
deleted file mode 100644
--- a/src/app.ts
+++ /dev/null
@@ -1 +0,0 @@
-export const ok = true
`,
			class:  ClassDestructive,
			action: ActionDeny,
		},
		{
			name: "package scripts require approval",
			patch: `diff --git a/package.json b/package.json
--- a/package.json
+++ b/package.json
@@ -1,3 +1,6 @@
 {
-  "name": "example"
+  "name": "example",
+  "scripts": {
+    "postinstall": "node scripts/install.js"
+  }
 }
`,
			class:  ClassExecutionBehavior,
			action: ActionRequireApproval,
		},
		{
			name: "parent traversal denies",
			patch: `diff --git a/../outside.txt b/../outside.txt
--- a/../outside.txt
+++ b/../outside.txt
@@ -1 +1,2 @@
 outside
+changed
`,
			class:  ClassOutsideProjectScope,
			action: ActionDeny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InspectPatch([]byte(tt.patch))
			if err != nil {
				t.Fatalf("InspectPatch returned error: %v", err)
			}
			if got.SchemaVersion != SchemaVersionInspection {
				t.Fatalf("schema version = %q", got.SchemaVersion)
			}
			if got.HighestClass != tt.class || got.RecommendedAction != tt.action {
				t.Fatalf("inspection class/action = %s/%s, want %s/%s: %#v", got.HighestClass, got.RecommendedAction, tt.class, tt.action, got)
			}
		})
	}
}

func TestParsePatchHandlesAddModifyDeleteRename(t *testing.T) {
	patch := `diff --git a/new.txt b/new.txt
new file mode 100644
--- /dev/null
+++ b/new.txt
@@ -0,0 +1 @@
+new
diff --git a/old.txt b/old.txt
deleted file mode 100644
--- a/old.txt
+++ /dev/null
@@ -1 +0,0 @@
-old
diff --git a/name-old.txt b/name-new.txt
similarity index 90%
rename from name-old.txt
rename to name-new.txt
--- a/name-old.txt
+++ b/name-new.txt
@@ -1 +1 @@
-old
+new
`
	changes, err := ParsePatch([]byte(patch))
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 3 {
		t.Fatalf("changes = %d, want 3: %#v", len(changes), changes)
	}
	if changes[0].Operation != OperationAdd || changes[1].Operation != OperationDelete || changes[2].Operation != OperationRename {
		t.Fatalf("operations = %s/%s/%s", changes[0].Operation, changes[1].Operation, changes[2].Operation)
	}
}

func TestInspectPatchRedactsSecretPathsAndContent(t *testing.T) {
	patch := `diff --git a/config/settings.yaml b/config/settings.yaml
--- a/config/settings.yaml
+++ b/config/settings.yaml
@@ -1 +1,2 @@
 name: app
+password=super-secret-value
`
	got, err := InspectPatch([]byte(patch))
	if err != nil {
		t.Fatal(err)
	}
	if got.HighestClass != ClassSecretBearing {
		t.Fatalf("class = %s, want %s", got.HighestClass, ClassSecretBearing)
	}
	rendered := got.Findings[0].Reason + " " + got.Findings[0].Path
	if strings.Contains(rendered, "super-secret-value") {
		t.Fatalf("secret value leaked in finding: %s", rendered)
	}

	secretPathPatch := `diff --git a/.env b/.env
--- /dev/null
+++ b/.env
@@ -0,0 +1 @@
+TOKEN=redacted
`
	got, err = InspectPatch([]byte(secretPathPatch))
	if err != nil {
		t.Fatal(err)
	}
	if got.Findings[0].Path != redactedSecretPath {
		t.Fatalf("secret path = %q, want redacted", got.Findings[0].Path)
	}
}

func TestCheckProjectPathRejectsUnsafeForms(t *testing.T) {
	for _, raw := range []string{
		"../outside.txt",
		"/tmp/outside.txt",
		`C:\tmp\outside.txt`,
		`\\server\share\outside.txt`,
		".git/hooks/pre-commit",
		"safe/../../outside.txt",
	} {
		t.Run(raw, func(t *testing.T) {
			got := CheckProjectPath(raw)
			if got.Safe {
				t.Fatalf("path %q was marked safe", raw)
			}
		})
	}
}

// TestCheckProjectPathRejectsControlSurfaces covers the self-protection rules:
// an agent must not be able to edit the gate its own tool calls pass through,
// nor the evidence of what that gate decided.
func TestCheckProjectPathRejectsControlSurfaces(t *testing.T) {
	for _, raw := range []string{
		".claude/settings.json",
		".claude/settings.local.json",
		".claude/hooks/pretooluse.sh",
		".claude/hooks/nested/deep.sh",
		".CLAUDE/Settings.json",
		"integrations/claude-code/pretooluse-boundary.sh",
		"vendor/integrations/claude-code/pretooluse-boundary.sh",
		"integrations/claude-code/sessionend-boundary.sh",
		"hooks/hooks.json",
		"nested/plugin/hooks/hooks.json",
		".boundary/command/decision-records.jsonl",
		".boundary/evidence/bundle.json",
		".boundary/bin/git",
		"nested/repo/.boundary/hook/decision-records.jsonl",
		"nested/repo/.claude/settings.json",
	} {
		t.Run(raw, func(t *testing.T) {
			got := CheckProjectPath(raw)
			if got.Safe {
				t.Fatalf("control-surface path %q was marked safe", raw)
			}
			if !strings.Contains(got.Reason, "outside the edit envelope scope") {
				t.Fatalf("reason = %q, want the edit-envelope framing", got.Reason)
			}
		})
	}
}

// TestControlSurfacePathsMatchTheImplementedRules is a drift guard: every shape
// the documented list advertises must actually be rejected, and the list must
// not grow a shape the classifier does not implement.
func TestControlSurfacePathsMatchTheImplementedRules(t *testing.T) {
	// One concrete path per documented shape, in the same order.
	concrete := []string{
		".boundary/hook/decision-records.jsonl",
		".claude/settings.json",
		".claude/settings.local.json",
		".claude/hooks/pretooluse.sh",
		"integrations/claude-code/pretooluse-boundary.sh",
		"integrations/claude-code/sessionend-boundary.sh",
		"hooks/hooks.json",
	}
	shapes := ControlSurfacePaths()
	if len(shapes) != len(concrete) {
		t.Fatalf("documented shapes = %d, sample paths = %d; keep them in step: %v",
			len(shapes), len(concrete), shapes)
	}
	for i, path := range concrete {
		if got := CheckProjectPath(path); got.Safe {
			t.Fatalf("documented shape %q (sample %q) is not rejected", shapes[i], path)
		}
	}
}

// TestCheckProjectPathKeepsOrdinaryAgentContentEditable pins the narrow scope of
// the control-surface rules: the rest of `.claude/` is ordinary content, and a
// file that merely mentions a control surface's name is not one.
func TestCheckProjectPathKeepsOrdinaryAgentContentEditable(t *testing.T) {
	for _, raw := range []string{
		".claude/skills/thing/SKILL.md",
		".claude/commands/ship.md",
		".claude/settings.example.json",
		"docs/integrations/claude-code/README.md",
		"integrations/claude-code/settings.snippet.json",
		"boundary/config.yaml",
	} {
		t.Run(raw, func(t *testing.T) {
			if got := CheckProjectPath(raw); !got.Safe {
				t.Fatalf("path %q was rejected as a control surface: %q", raw, got.Reason)
			}
		})
	}
}

// TestControlSurfaceEditsClassifyToADenyAction is the end-to-end posture: a
// synthesized one-file patch against each control surface must reach a
// deny-action class, not merely a require-approval one.
func TestControlSurfaceEditsClassifyToADenyAction(t *testing.T) {
	for _, target := range []string{
		".claude/settings.json",
		".claude/settings.local.json",
		".claude/hooks/pretooluse.sh",
		"integrations/claude-code/pretooluse-boundary.sh",
		".boundary/command/decision-records.jsonl",
	} {
		t.Run(target, func(t *testing.T) {
			patch := fmt.Sprintf("diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n", target, target, target, target)
			got, err := InspectPatch([]byte(patch))
			if err != nil {
				t.Fatalf("InspectPatch(%q): %v", target, err)
			}
			if got.RecommendedAction != ActionDeny {
				t.Fatalf("action = %q, want deny", got.RecommendedAction)
			}
			if got.HighestClass != ClassOutsideProjectScope {
				t.Fatalf("class = %q, want %q", got.HighestClass, ClassOutsideProjectScope)
			}
		})
	}
}
