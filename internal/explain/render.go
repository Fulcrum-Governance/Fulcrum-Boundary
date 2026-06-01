package explain

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteJSON emits the boundary.explain.v1 envelope as two-space-indented JSON,
// matching the doctor/selftest/version JSON convention.
func WriteJSON(w io.Writer, result *Result) error {
	if result == nil {
		return fmt.Errorf("explain result is required")
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// WriteText renders the explain Result as a human-readable account. It is a pure
// description of the record; it states explicitly that explain does not verify
// hashes and that direct access remains a bypass a record cannot see.
func WriteText(w io.Writer, result *Result) error {
	if result == nil {
		return fmt.Errorf("explain result is required")
	}
	fmt.Fprintln(w, "Boundary explain")
	fmt.Fprintf(w, "status: %s\n", result.Status)
	fmt.Fprintf(w, "record schema_version: %s\n", result.RecordSchemaVersion)
	if result.RecordID != "" {
		fmt.Fprintf(w, "record_id: %s\n", result.RecordID)
	}
	fmt.Fprintln(w, "credentials: none")
	fmt.Fprintln(w, "network: none")
	fmt.Fprintln(w, "live mutation: none")

	fmt.Fprintln(w, "\nDecision:")
	fmt.Fprintf(w, "  action: %s\n", result.Decision.Action)
	writeOptional(w, "reason", result.Decision.Reason)
	writeOptional(w, "decision_mode", result.Decision.DecisionMode)
	writeOptional(w, "matched_rule", result.Decision.MatchedRule)
	writeOptional(w, "policy_file", result.Decision.PolicyFile)
	writeOptional(w, "tool", result.Decision.Tool)
	writeOptional(w, "adapter", result.Decision.Adapter)
	writeOptional(w, "event_type", result.Decision.EventType)

	if result.RouteContext != nil {
		fmt.Fprintln(w, "\nRoute context (schema_version 2, descriptive only):")
		writeOptional(w, "adapter_id", result.RouteContext.AdapterID)
		writeOptional(w, "route_id", result.RouteContext.RouteID)
		writeOptional(w, "topology_profile (asserted, not attested)", result.RouteContext.TopologyProfile)
		if claim := result.RouteContext.ExecutionClaim; claim != nil {
			fmt.Fprintf(w, "  execution_claim (self-report, not corroborated): upstream_called=%t executed=%t",
				claim.UpstreamCalled, claim.Executed)
			if claim.Source != "" {
				fmt.Fprintf(w, " source=%s", claim.Source)
			}
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintln(w, "\nHashes (described, not recomputed):")
	for _, hash := range result.Hashes {
		if !hash.Present {
			continue
		}
		fmt.Fprintf(w, "  %s: %s\n", hash.Field, hash.Value)
		fmt.Fprintf(w, "    covers: %s\n", hash.Covers)
	}
	fmt.Fprintf(w, "\nverify: %s\n", result.VerifyHint)

	fmt.Fprintln(w, "\nWhat this does not prove:")
	for _, line := range result.DoesNotProve {
		fmt.Fprintf(w, "- %s\n", line)
	}
	return nil
}

// writeOptional prints an indented "name: value" line only when value is set, so
// the human output stays clean for V1 records that omit optional fields.
func writeOptional(w io.Writer, name, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(w, "  %s: %s\n", name, value)
}
