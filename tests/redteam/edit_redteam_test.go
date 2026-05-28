package redteam_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func TestEditRedteamPackageScriptCLI(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"redteam", "--pack", "edit-package-script-mutation"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("edit redteam pack exit = %d, stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"pack: edit-package-script-mutation",
		"live mutation: none",
		"real secrets: none",
		"scenario: edit-package-postinstall",
		"attack: edit-package-script-mutation",
		"patch: package.json scripts changed",
		"class: E6",
		"risk: HIGH",
		"applied: false",
		"expected: REQUIRE_APPROVAL",
		"actual: REQUIRE_APPROVAL",
		"result: pass",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("edit redteam output missing %q: %s", want, output)
		}
	}
}

func TestEditRedteamPacksJSON(t *testing.T) {
	for _, pack := range []string{
		"edit-secret-exfil",
		"edit-package-script-mutation",
		"edit-ci-deploy-mutation",
		"edit-destructive-delete",
		"edit-cross-scope-mutation",
	} {
		t.Run(pack, func(t *testing.T) {
			var stdout bytes.Buffer
			code := boundarycli.Run([]string{"redteam", "--pack", pack, "--format", "json"}, &stdout, &bytes.Buffer{})
			if code != 0 {
				t.Fatalf("edit redteam json exit = %d", code)
			}
			var payload struct {
				PackID             string `json:"pack_id"`
				Passed             bool   `json:"passed"`
				MutatesLiveSystems bool   `json:"mutates_live_systems"`
				RealSecretsUsed    bool   `json:"real_secrets_used"`
				Results            []struct {
					ScenarioID     string `json:"scenario_id"`
					Patch          string `json:"patch"`
					EditClass      string `json:"edit_class"`
					ExpectedAction string `json:"expected_action"`
					ActualAction   string `json:"actual_action"`
					Applied        bool   `json:"applied"`
				} `json:"results"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("parse edit redteam json: %v\n%s", err, stdout.String())
			}
			if payload.PackID != pack || !payload.Passed {
				t.Fatalf("unexpected payload identity: %#v", payload)
			}
			if payload.MutatesLiveSystems || payload.RealSecretsUsed {
				t.Fatalf("fixture payload must not mutate live systems or use real secrets: %#v", payload)
			}
			if len(payload.Results) == 0 {
				t.Fatal("expected at least one scenario result")
			}
			for _, result := range payload.Results {
				if result.ExpectedAction != result.ActualAction {
					t.Fatalf("%s expected %q actual %q", result.ScenarioID, result.ExpectedAction, result.ActualAction)
				}
				if result.Applied {
					t.Fatalf("%s applied a fixture patch", result.ScenarioID)
				}
				if result.Patch == "" || result.EditClass == "" {
					t.Fatalf("%s missing edit metadata: %#v", result.ScenarioID, result)
				}
			}
		})
	}
}

func TestEditRedteamListShowsImplementedPacks(t *testing.T) {
	var stdout bytes.Buffer
	code := boundarycli.Run([]string{"redteam", "--list"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("redteam list exit = %d", code)
	}
	output := stdout.String()
	for _, want := range []string{
		"edit-secret-exfil\timplemented",
		"edit-package-script-mutation\timplemented",
		"edit-ci-deploy-mutation\timplemented",
		"edit-destructive-delete\timplemented",
		"edit-cross-scope-mutation\timplemented",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("redteam list missing %q: %s", want, output)
		}
	}
}
