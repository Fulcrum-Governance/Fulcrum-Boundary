# Edit Boundary Preview Claims

## Planned Claim

```yaml
BND-CLAIM-EDIT-001:
  claim: "Boundary defines a preview Edit Boundary design for routed file mutation governance."
  status: planned
```

This is a roadmap claim only. Do not describe Edit Boundary as delivered until
the inspect, apply, redteam, and reconciliation work lands with tests and
evidence.

## Allowed Copy

- Boundary can classify and gate proposed file mutations before they are
  applied.
- Edit Boundary applies only to file mutations routed through a Boundary edit
  envelope.
- Edit Boundary is a planned preview surface for routed file mutation
  governance.

## Forbidden Copy

- Boundary controls all file writes.
- Boundary protects direct editor writes.
- Boundary prevents every unsafe edit.
- Boundary provides filesystem sandboxing.
- Boundary provides universal coding-agent file safety.
- Boundary governs direct file edits outside routed edit envelopes.

## Delivery Gate

A delivered Edit Boundary claim requires:

- inspect tests proving no mutation;
- path traversal and secret-bearing edit denial tests;
- apply wrapper tests proving denied, approval-required, and dry-run edits do
  not apply;
- exact patch hash binding for approval;
- redacted decision records;
- fixture redteam packs with no live mutation;
- release truth reconciliation that keeps direct editor writes and direct
  filesystem writes outside the claim.
