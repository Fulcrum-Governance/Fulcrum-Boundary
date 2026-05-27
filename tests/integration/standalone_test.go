package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fulcrum-governance/boundary/governance"
	"github.com/fulcrum-governance/boundary/governance/standalone"
	"github.com/stretchr/testify/require"
)

func TestStandaloneBundleBootsWithoutExternalDependencies(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(`
name: standalone
version: v1
rules:
  - name: block-drop
    tool: query
    action: deny
    reason: destructive SQL
    match:
      field: arguments.sql
      contains: DROP TABLE
`), 0o600))

	ctx := context.Background()
	bundle, err := standalone.NewBundle(ctx, dir)
	require.NoError(t, err)
	require.Len(t, bundle.PolicyRules, 1)

	state, err := bundle.Trust.CheckAgentState(ctx, "agent-1")
	require.NoError(t, err)
	require.Equal(t, governance.TrustStateTrusted, state)

	cost, err := bundle.Cost.PredictCost(ctx, governance.GovernanceRequest{AgentID: "agent-1"})
	require.NoError(t, err)
	allowed, err := bundle.Budget.CheckBudget(ctx, "tenant-1", "agent-1", cost)
	require.NoError(t, err)
	require.True(t, allowed)

	envelopeID, err := bundle.Envelope.CreateEnvelope(ctx, governance.GovernanceRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, envelopeID)

	ref, err := bundle.Proofs.GetCorrespondence("trust_termination")
	require.NoError(t, err)
	require.Equal(t, "design", ref.Correspondence)
}
