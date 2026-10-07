package hookboundary_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/hookboundary"
)

type ToolRoutingState string

const (
	StateRouted          ToolRoutingState = "routed"
	StateNotRouted       ToolRoutingState = "not_routed"
	StatePartiallyRouted ToolRoutingState = "partially_routed"
)

type toolInventory struct {
	ToolName string
	State    ToolRoutingState
}

func TestHookMatcherCoverageInventory(t *testing.T) {
	inventory := []toolInventory{
		{"Bash", StateRouted},
		{"Edit", StateRouted},
		{"Write", StateRouted},
		{"MultiEdit", StateRouted},
		{"NotebookEdit", StateRouted},
		{"bash", StateNotRouted},
		{"Shell", StateNotRouted},
		{"shell", StateNotRouted},
		{"Read", StateNotRouted},
		{"WebFetch", StateNotRouted},
		{"mcp__postgres__query", StateNotRouted},
		{"Task", StateNotRouted},
		{"Grep", StateNotRouted},
		{"Glob", StateNotRouted},
		{"mcp__Bash__run", StatePartiallyRouted},
		{"NotBash", StatePartiallyRouted},
	}

	manifestPath := filepath.Join("..", "..", "hooks", "hooks.json")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read hooks.json: %v", err)
	}

	var manifest struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatalf("parse hooks.json: %v", err)
	}

	if len(manifest.Hooks.PreToolUse) == 0 {
		t.Fatalf("no PreToolUse hooks defined in manifest")
	}

	matcherStr := manifest.Hooks.PreToolUse[0].Matcher
	matcher, err := regexp.Compile(matcherStr)
	if err != nil {
		t.Fatalf("compile matcher: %v", err)
	}

	// Verify all inventory items
	for _, item := range inventory {
		t.Run(item.ToolName, func(t *testing.T) {
			pluginMatches := matcher.MatchString(item.ToolName)
			binaryRoutes := hookboundary.RouteFor(item.ToolName) != hookboundary.RouteNone

			var got ToolRoutingState
			if pluginMatches && binaryRoutes {
				got = StateRouted
			} else if pluginMatches && !binaryRoutes {
				got = StatePartiallyRouted
			} else if !pluginMatches && binaryRoutes {
				got = StateNotRouted // We mark it as not_routed since plugin doesn't send it, even though binary handles it
			} else {
				got = StateNotRouted
			}

			if got != item.State {
				t.Fatalf("tool %q routing state = %q, want %q", item.ToolName, got, item.State)
			}
		})
	}

	// Pin the matcher set
	expectedMatcher := "Bash|Edit|Write|MultiEdit|NotebookEdit"
	if matcherStr != expectedMatcher {
		t.Fatalf("matcher string changed from %q to %q; update the inventory test and ENFORCEMENT_REPORT.md", expectedMatcher, matcherStr)
	}
}
