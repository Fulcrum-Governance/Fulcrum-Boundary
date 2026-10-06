# A2A Protocol Conformance

This document records the live protocol conformance evidence for the Boundary A2A adapter against the official `a2aproject` reference implementation.

## Protocol Versions
- **A2A Protocol Snapshot:** v0.3.0 and v1.0
- **Official SDK:** `@a2a-js/sdk` v1.3.0

## Test Cases Validated
The conformance suite (`tests/adapter_conformance/a2a_live_conformance_test.go`) validates the following behaviors using the live reference server:

1. **Agent Card Discovery:** The adapter passes through discovery payload requests when structured correctly, though discovery is typically handled out of band.
2. **Evaluated Methods:**
   - **`tasks/send`**: JSON-RPC parsed correctly, evaluated via pipeline.
   - **`message/send`**: JSON-RPC parsed correctly, evaluated via pipeline.
   - **`message/stream`**: Parses as action-bearing payload. Correctly allowed or blocked by pipeline.
   - For all evaluated methods, an **Allowed** decision forwards the task to the upstream server seamlessly, and a **Denied** decision blocks the task, returning a protocol-correct error response without the upstream server ever receiving it.
3. **Blocked / Unsupported Methods:**
   - **`tasks/get`**, **`tasks/cancel`**, **`tasks/resubscribe`**, **`push/config`**: These are not currently supported as action-bearing and are failed closed before reaching upstream to prevent silent bypasses.
4. **Unknown Methods / Mandatory Fields:** Unrecognized mandatory fields or completely unknown methods result in an unsupported error and do not reach the server.
5. **Streaming:** SSE streaming setup and behavior works correctly for `message/stream`.
6. **Large/Edge Payloads:** Payloads are correctly processed, forwarded, and bounded.
7. **Evaluator Failure:** If the pipeline or environment fails, the request fails closed and does not reach the upstream server.

## Execution
This conformance is checked continuously in CI using `.github/workflows/a2a-conformance.yml`. The test binary sets up an A2A HTTP Express-based server (from `@a2a-js/sdk`), proxies traffic through the Boundary A2A adapter, and evaluates the results against both allowed and denied rulesets.
