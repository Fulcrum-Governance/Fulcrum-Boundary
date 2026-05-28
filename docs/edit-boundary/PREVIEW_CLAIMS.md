# Edit Boundary Preview Claims

## Delivered Preview Claim

```yaml
BND-CLAIM-EDIT-001:
  claim: "Boundary provides preview Edit Boundary governance for proposed file mutations routed through Boundary edit envelopes."
  status: delivered
```

## Delivered Fixture Claim

```yaml
BND-CLAIM-EDIT-002:
  claim: "Boundary runs fixture Edit Boundary redteam packs that deny or require approval for selected file-mutation risk paths without live project mutation."
  status: delivered
```

`BND-CLAIM-EDIT-001` is delivered preview behavior only. It does not mark Edit
Boundary production, and it does not cover direct editor writes, direct
filesystem writes, shell redirection, direct `git apply`, or unwrapped IDE APIs.
`BND-CLAIM-EDIT-002` is fixture proof for selected edit-risk paths.

## Allowed Copy

- Boundary can classify and gate proposed file mutations before they are
  applied.
- Edit Boundary applies only to file mutations routed through a Boundary edit
  envelope.
- Boundary provides preview Edit Boundary governance for proposed file mutations
  routed through Boundary edit envelopes.

## Forbidden Copy

- Boundary controls all file writes.
- Boundary protects direct editor writes.
- Boundary prevents every unsafe edit.
- Boundary provides filesystem sandboxing.
- Boundary provides universal coding-agent file safety.
- Boundary governs direct file edits outside routed edit envelopes.

## Delivery Gate

The delivered preview Edit Boundary claim requires:

- inspect tests proving no mutation;
- path traversal and secret-bearing edit denial tests;
- apply wrapper tests proving denied, approval-required, and dry-run edits do
  not apply;
- exact patch hash binding for approval;
- redacted decision records;
- fixture redteam packs with no live mutation;
- release truth reconciliation that keeps direct editor writes and direct
  filesystem writes outside the claim.
