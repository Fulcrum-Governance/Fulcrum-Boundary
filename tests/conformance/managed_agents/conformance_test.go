package managedagentsconformance

import (
	"os"
	"testing"
)

const (
	enableEnv     = "BOUNDARY_MA_CONFORMANCE"
	transcriptEnv = "BOUNDARY_MA_TRANSCRIPT"
)

func TestSessionCreationThroughBoundaryProxy(t *testing.T) {
	tr := loadTranscript(t)
	if err := CheckSessionCreated(tr); err != nil {
		t.Fatal(err)
	}
}

func TestToolConfirmationAllow(t *testing.T) {
	tr := loadTranscript(t)
	if err := CheckAllowConfirmation(tr); err != nil {
		t.Fatal(err)
	}
}

func TestToolConfirmationDeny(t *testing.T) {
	tr := loadTranscript(t)
	if err := CheckDenyConfirmation(tr); err != nil {
		t.Fatal(err)
	}
}

func TestMCPToolUseEvent(t *testing.T) {
	tr := loadTranscript(t)
	if err := CheckMCPToolUse(tr); err != nil {
		t.Fatal(err)
	}
}

func TestThreadCreationAndTracking(t *testing.T) {
	tr := loadTranscript(t)
	if err := CheckThreadTracking(tr); err != nil {
		t.Fatal(err)
	}
}

func TestBudgetTrackingAgainstCeiling(t *testing.T) {
	tr := loadTranscript(t)
	if err := CheckBudget(tr); err != nil {
		t.Fatal(err)
	}
}

func TestTrustTrackingInDecisionRecords(t *testing.T) {
	tr := loadTranscript(t)
	if err := CheckTrustScores(tr); err != nil {
		t.Fatal(err)
	}
}

func TestDecisionMetadata(t *testing.T) {
	tr := loadTranscript(t)
	if err := CheckDecisionMetadata(tr); err != nil {
		t.Fatal(err)
	}
}

func TestFailClosedBehaviorOnPipelineError(t *testing.T) {
	tr := loadTranscript(t)
	if err := CheckFailClosed(tr); err != nil {
		t.Fatal(err)
	}
}

// TestLiveModeTranscriptOnly rejects anything but a live-mode transcript when
// the live conformance gate is enabled. A stub-mode transcript is offline
// fixture evidence produced by the session driver; it exercises this harness
// but must never be accepted as live conformance evidence.
func TestLiveModeTranscriptOnly(t *testing.T) {
	tr := loadTranscript(t)
	if err := CheckLiveMode(tr); err != nil {
		t.Fatal(err)
	}
}

func TestSanitizedTranscriptEvidence(t *testing.T) {
	tr, data := loadTranscriptBytes(t)
	if err := CheckSanitized(tr, data); err != nil {
		t.Fatal(err)
	}
}

func loadTranscript(t *testing.T) Transcript {
	t.Helper()
	tr, _ := loadTranscriptBytes(t)
	return tr
}

func loadTranscriptBytes(t *testing.T) (Transcript, []byte) {
	t.Helper()
	if os.Getenv(enableEnv) != "true" {
		t.Skip(enableEnv + " not set")
	}
	path := os.Getenv(transcriptEnv)
	if path == "" {
		t.Skip(transcriptEnv + " not set; run live conformance and point this env var at the sanitized transcript")
	}
	tr, data, err := LoadTranscriptFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return tr, data
}
