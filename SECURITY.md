# Fulcrum Boundary Security Policy

## Reporting a vulnerability

Email: **agent@fulcrumlayer.io**

Please include:

- A description of the issue and the version or commit affected.
- A minimal reproducer when one is available.
- Your assessment of impact and any known exploitation conditions.
- Whether the issue has been disclosed elsewhere.

Do not file public GitHub issues for security vulnerabilities.

We review security reports as project maintainers have availability and
prioritize issues that affect released Boundary behavior. This repository does
not advertise a bounty, guaranteed response window, or hosted incident
response service.

## What is in scope

Boundary is the action boundary for routed agent tool calls. The following are
in scope for this repository:

- Bypasses where a routed tool call reaches a downstream tool without passing
  through Boundary evaluation.
- Incorrect fail-closed or fail-open behavior, including evaluator errors that
  lead to an unsafe allow decision on a fail-closed path.
- Race conditions in the evaluation pipeline or adapter paths.
- Logic bugs in static policy, policy evaluation, or interceptor stages that
  invert intended semantics.
- Parser vulnerabilities across released adapters and preview adapter packages.
- Fixture or evidence utilities that report a deny decision while still making
  the upstream mutation call.

## Known limitations

Boundary evaluates routed action metadata, arguments, policy state, and adapter
context. It does not perform general semantic analysis of every natural-language
prompt, every tool output, or every bypass path outside the routed deployment
topology.

Known limits include:

- Direct tool access is outside Boundary unless deployment topology removes or
  blocks that bypass.
- Secure GitHub is a preview surface; fixture and conformance receipts do not
  make it a production GitHub security product.
- Command Boundary is a preview, routed-path-only surface; it does not control
  every shell command on a machine.
- Edit Boundary is a preview, routed-edit-envelope-only surface; it does not
  control every file write on a machine.
- Generated policies are starter policies for operator review, not complete
  production policy.
- Dashboard output is local-only artifact visibility, not hosted monitoring.
- Some encoded, compiled, or transformed payloads can evade static parser
  coverage unless a routed adapter or interceptor recognizes them.

## Dependencies

Boundary keeps the root module dependency set small and runs vulnerability
checks through repository CI. Dependency scan results are one signal; they do
not by themselves prove production safety for a deployment.

## Disclosure credit

Unless you ask otherwise, valid reports may be credited in release notes for
the fix.
