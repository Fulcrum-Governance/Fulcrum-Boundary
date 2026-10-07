# A2A Protocol-Shaped Mock Upstream

This document details tests run for the Boundary A2A adapter against a protocol-shaped mock upstream. These tests **do not establish protocol conformance**, because the official `@a2a-js/sdk` was not exercised.

## Protocol Versions
- **A2A Protocol Snapshot:** v0.3.0 and v1.0
- **Test Server:** A hand-written Go `httptest` mock upstream designed to reflect JSON-RPC A2A structures for basic validation.

## Test Cases Validated
The test suite (`tests/adapter_conformance/a2a_mock_upstream_test.go`) validates the following behaviors against the mock upstream:

1. **Agent Card Discovery:** (`TestA2AMockUpstream/Agent_Card_Discovery`) The adapter passes through discovery payload requests when structured correctly.
2. **Evaluated Methods:**
   - **`tasks/send`**: (`TestA2AMockUpstream/Tasks_/_Send_-_Allowed`, `TestA2AMockUpstream/Tasks_/_Send_-_Denied`) JSON-RPC parsed correctly, evaluated via pipeline.
   - **`message/send`**: (`TestA2AMockUpstream/Message_/_Send_-_Allowed`) JSON-RPC parsed correctly, evaluated via pipeline.
   - **`message/stream`**: (`TestA2AMockUpstream/Streaming_(message/stream_with_SSE)_-_Allowed`, `TestA2AMockUpstream/Streaming_(message/stream_with_SSE)_-_Denied`) Parses as action-bearing payload. Correctly allowed or blocked by pipeline. *Note: only the request is evaluated; streamed response events are not governed.*
   - For all evaluated methods, an **Allowed** decision forwards the task to the upstream server seamlessly, and a **Denied** decision blocks the task, returning a protocol-correct error response without the upstream server ever receiving it.
3. **Blocked / Unsupported Methods:**
   - **`tasks/get`**, **`tasks/cancel`**, **`tasks/resubscribe`**, **`push/config`**: (`TestA2AMockUpstream/Tasks_Get_/_Cancel`, `TestA2AMockUpstream/Push-notification_config`) These are not currently supported as action-bearing and are failed closed before reaching upstream to prevent silent bypasses.
4. **Unknown Methods / Mandatory Fields:** (`TestA2AMockUpstream/Unknown_Methods_/_Mandatory_Fields`) Unrecognized mandatory fields or completely unknown methods result in an unsupported error and do not reach the server.
5. **Streaming:** (`TestA2AMockUpstream/Streaming_(message/stream_with_SSE)_-_Allowed`) SSE streaming setup behavior works.
6. **Large/Edge Payloads:** (`TestA2AMockUpstream/Large/Edge_Payloads_-_Bounded`) Payloads are correctly processed, forwarded, and bounded.
7. **Evaluator Failure:** (`TestA2AMockUpstream/Evaluator_Failure`) If the pipeline or environment fails, the request fails closed and does not reach the upstream server.

## Execution
The test binary sets up a mock A2A Go server, proxies traffic through the Boundary A2A adapter, and evaluates the results against both allowed and denied rulesets.
