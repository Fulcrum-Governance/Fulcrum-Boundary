package integration

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func TestTrustShowDisplaysStandaloneState(t *testing.T) {
	var stdout bytes.Buffer
	code := boundarycli.Run([]string{"trust", "show", "agent-1"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("trust show failed")
	}
	for _, want := range []string{"agent_id: agent-1", "state: TRUSTED", "score:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("trust output missing %q: %s", want, stdout.String())
		}
	}
}

func TestTrustDegradationDemoIsolatesAgent(t *testing.T) {
	var stdout bytes.Buffer
	code := boundarycli.Run([]string{"demo", "trust-degradation"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("trust degradation demo failed")
	}
	if !strings.Contains(stdout.String(), "state=ISOLATED") && !strings.Contains(stdout.String(), "state: ISOLATED") {
		t.Fatalf("demo did not isolate agent: %s", stdout.String())
	}
}
