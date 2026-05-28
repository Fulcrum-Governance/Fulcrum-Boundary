# Edit Boundary Redteam

Edit Boundary redteam packs are fixture-only checks for proposed file mutation
risk paths. They classify and evaluate patch bytes, but they do not apply
patches and do not mutate live projects.

```bash
boundary redteam --pack edit-secret-exfil
boundary redteam --pack edit-package-script-mutation
boundary redteam --pack edit-ci-deploy-mutation
boundary redteam --pack edit-destructive-delete
boundary redteam --pack edit-cross-scope-mutation
```

Every pack runs in fixture mode. Live mode is unsupported for redteam packs.

## What They Prove

- selected secret-bearing edit paths are denied;
- selected destructive or cross-scope edit paths are denied;
- selected package, script, CI, Docker, and infrastructure mutations require
  approval;
- fixture redteam output reports `applied=false`.

## What They Do Not Prove

- direct editor-write protection;
- arbitrary filesystem interception;
- IDE control;
- filesystem sandboxing;
- universal prevention of unsafe file edits.

## Example

```text
redteam mode: fixture
pack: edit-package-script-mutation
live mutation: none
real secrets: none
scenario: edit-package-postinstall
attack: edit-package-script-mutation
patch: package.json scripts changed
class: E6
risk: HIGH
applied: false
expected: REQUIRE_APPROVAL
actual: REQUIRE_APPROVAL
result: pass
reason: execution behavior edit requires approval
```

## Fixture Corpus

Patch fixtures live under `fixtures/editboundary/redteam/` so the public
examples remain inspectable without requiring credentials or live mutation.
