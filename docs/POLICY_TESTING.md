# Policy-as-Code Tests

`boundary test` runs operator-authored policy test cases against local Boundary
policy bundles. It is a local-only assertion runner: it does not require
credentials, does not call upstream tools, does not use the network, and does
not mutate live systems.

Availability note: `boundary test` is included in the `v0.9.0` release. The
`@v0.9.0` install includes it; the historical `@v0.8.0` install does not.

```bash
boundary test --path tests/fixtures/policy-test/cases
boundary test --path tests/fixtures/policy-test/cases --format json
```

The default path is `.boundary/tests`, so an operator-owned repository can keep
policy tests beside its policy bundle:

```bash
boundary test
boundary test --format json
```

## What A Case Contains

Each YAML case names a policy bundle, a request fixture, and the expected
Boundary verdict:

```yaml
name: deny-write-after-taint
policies: ../policies/deny-write-after-taint
request:
  transport: mcp
  tool_name: github.create_or_update_file
  action: tools/call
  agent_id: policy-test-agent
  tenant_id: policy-test-tenant
  arguments:
    untrusted_input: "true"
    target_repo_visibility: private
    file_path: src/handler.go
expect:
  action: deny
  reason_contains: lethal_trifecta_detected
```

Supported expected actions are `allow`, `deny`, `warn`, `require_approval`,
`escalate`, and `parse_rejection`. The `parse_rejection` action is for negative
policy-bundle fixtures: the case passes only when the named policy bundle fails
to load. Any other load error, malformed case file, decision mismatch, or
`reason_contains` mismatch fails the run and exits non-zero.

## Output

Text output is meant for humans:

```text
boundary test: tests/fixtures/policy-test/cases
credentials: none
network: none
live mutation: none
  [pass] deny-write-after-taint       expect=deny             actual=deny matched_rule=deny-private-write-after-taint
status: pass
cases: 6
passed: 6
failed: 0
```

JSON output emits a stable `boundary.test.v1` envelope with the same local-safety
flags:

```bash
boundary test --path tests/fixtures/policy-test/cases --format json
```

The JSON result includes `requires_credentials: false`, `requires_network:
false`, `mutates_live_systems: false`, a summary, one result per case, and a
fixed `does_not_prove` footer.

## CI Recipe

Build Boundary, then run the policy corpus as an ordinary CI step:

```yaml
name: Boundary policy tests
on:
  pull_request:
  push:
    branches: [main]

jobs:
  boundary-policy-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go build -o ./bin/boundary ./cmd/boundary
      - run: ./bin/boundary test --path .boundary/tests --format json | tee boundary-test.json
```

The CI check fails when `boundary test` exits non-zero. Treat the JSON output as
test evidence for the policy bundle under review, not as production enforcement
evidence.

## What This Does Not Prove

`boundary test` reports policy verdicts for routed request fixtures only. A
passing run does not prove production route enforcement, does not prove a
deployment removed every direct or unrouted path to the same tool, and does not
prove the verdict was globally correct. It proves only that the local policy
bundle produced the expected decision for the supplied fixture.

Use route conformance and deployment bypass evidence for deployment posture.
Use `boundary verify-record`, `boundary explain`, and `boundary replay` for
decision-record inspection and reproduction.
