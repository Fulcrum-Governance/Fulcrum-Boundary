package governance

import (
	"context"
	"testing"
)

// A parse rejection is a decision record. When the checked publisher fails to
// persist it, the pipeline must report the loss the same way it does for an
// evaluated decision: AuditFailures rises and Degraded turns on.
func TestPublishParseRejection_DeliveryFailureMarksDegraded(t *testing.T) {
	auditor := &failingCheckedPublisher{failRemaining: 1}
	pipeline := NewPipeline(PipelineConfig{}, nil, nil, auditor)

	pipeline.PublishParseRejection(context.Background(), ParseRejectionEvent{
		Adapter:         TransportMCP,
		RawPayload:      []byte(`{"not":"a tool call"}`),
		RejectionReason: "malformed envelope",
	})

	if got := pipeline.AuditFailures(); got != 1 {
		t.Fatalf("AuditFailures() = %d, want 1 after a failed parse-rejection record", got)
	}
	if !pipeline.Degraded() {
		t.Fatal("Degraded() = false after a failed parse-rejection record")
	}

	pipeline.PublishParseRejection(context.Background(), ParseRejectionEvent{
		Adapter:         TransportMCP,
		RawPayload:      []byte(`{"still":"not a tool call"}`),
		RejectionReason: "malformed envelope",
	})
	if pipeline.Degraded() {
		t.Fatal("Degraded() stayed true after a later parse-rejection record was delivered")
	}
}
