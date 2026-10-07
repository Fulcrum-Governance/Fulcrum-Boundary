#!/usr/bin/env python3
"""Standalone verifier for a Fulcrum Boundary decision record.

This script reproduces a decision record's ``decision_hash`` with no Boundary
code on the path: it reads a decision-record JSON file, canonicalizes it with a
stock RFC 8785 / JCS implementation (the ``rfc8785`` package), SHA-256s the
canonical bytes, and compares the result to the record's own stored
``decision_hash``.

What this proves and does not prove
-----------------------------------
The decision record is RFC 8785 / JCS-canonicalized, and its ``decision_hash``
is an unkeyed SHA-256 over that canonical form. Recomputing it here is an
INTEGRITY check: it detects whether the covered fields of the record were
altered after emission. It is NOT an AUTHENTICITY check -- an unkeyed hash does
not prove who produced the record, and editing the record yields a new,
internally consistent hash. The optional ``signature`` / ``signature_key_id``
fields (which this script intentionally excludes from the hash, mirroring
Boundary) are where authorship would be attested; this verifier does not check
them. A passing check is also not evidence that the governed action was executed
or prevented.

How the hash is reproduced (mirrors governance/receipt.go ComputeDecisionHash)
------------------------------------------------------------------------------
Boundary computes ``decision_hash`` over the record with four fields neutralized
first, so the hash is self-excluding and signature-excluding:

  * ``record_id``      -> set to "" (Boundary always emits this key)
  * ``decision_hash``  -> set to "" (Boundary always emits this key)
  * ``signature``      -> dropped  (Boundary emits this key only when set)
  * ``signature_key_id`` -> dropped (Boundary emits this key only when set)

then it canonicalizes the result with RFC 8785 / JCS and takes
``"sha256:" + hex(sha256(canonical))``.

Two Go-vs-JCS subtleties this reproduces correctly via the ``rfc8785`` library:

  * HTML-significant characters ``&``, ``<``, ``>`` stay LITERAL in the
    canonical form (Go's default JSON encoder would escape them; JCS does not).
    Records on disk may store these as ``\\u0026`` / ``\\u003c`` / ``\\u003e``;
    ``json.load`` decodes them back to literal characters before canonicalizing,
    so the canonical preimage matches Boundary's.
  * Numbers use the ECMAScript shortest-round-trip form (e.g. a trust_score of
    1/3 serializes as ``0.3333333333333333``). The ``rfc8785`` library applies
    the same Number-to-string rule.

Both decision-record schema versions ("1" and "2") hash through this exact same
path; schema 2 simply carries additional route-context keys that JCS sorts in
with the rest. No per-version branching is needed here.

Usage
-----
    pip install rfc8785
    python3 boundary_verify.py <record.json>

Exit status: 0 when the recomputed hash equals the stored ``decision_hash``;
1 on mismatch, a missing/empty ``decision_hash``, or a load error. See README.md
in this directory.
"""

from __future__ import annotations

import hashlib
import json
import sys
from typing import Any

try:
    import rfc8785
except ImportError:  # pragma: no cover - exercised only without the dependency.
    sys.stderr.write(
        "error: the 'rfc8785' package is required.\n"
        "       install it with:  pip install rfc8785\n"
    )
    raise SystemExit(1)


# Fields blanked to "" before hashing. Boundary always emits these keys, and its
# ComputeDecisionHash sets them to the empty string, so the canonical preimage
# contains them as "".
_BLANK_TO_EMPTY = ("record_id", "decision_hash")

# Fields dropped entirely before hashing. Boundary declares these omitempty, so
# after blanking they are absent from the marshaled preimage. Removing the keys
# here reproduces that exactly.
_DROP = ("signature", "signature_key_id")


def compute_decision_hash(record: dict[str, Any]) -> str:
    """Return the ``decision_hash`` Boundary would compute for ``record``.

    ``record`` is the parsed decision record (a plain dict). This does not
    mutate the caller's dict: it copies, applies Boundary's field neutralization
    (blank ``record_id`` / ``decision_hash`` to "", drop ``signature`` /
    ``signature_key_id``), canonicalizes with RFC 8785 / JCS, and returns
    ``"sha256:" + hex`` of the SHA-256 digest.
    """
    preimage = dict(record)
    for key in _BLANK_TO_EMPTY:
        preimage[key] = ""
    for key in _DROP:
        preimage.pop(key, None)

    canonical = rfc8785.dumps(preimage)  # bytes, RFC 8785 canonical form.
    digest = hashlib.sha256(canonical).hexdigest()
    return "sha256:" + digest


def verify_record(record: dict[str, Any]) -> tuple[bool, str]:
    """Verify a parsed decision record.

    Returns ``(ok, message)``. ``ok`` is True only when the record carries a
    non-empty ``decision_hash`` and the recomputed hash equals it. ``message``
    is a human-readable line suitable for printing.
    """
    stored = record.get("decision_hash")
    if not stored:
        return False, "decision_hash missing or empty: nothing to verify against"

    recomputed = compute_decision_hash(record)
    if recomputed == stored:
        return True, "record verification: ok"
    return False, f"decision_hash mismatch: got {recomputed} want {stored}"


# Machine-readable rejection classes, emitted on stderr as ``reason=<code>``
# when verification or parsing fails. This is the shared vocabulary the
# cross-language verifier parity harness (scripts/ci/run-verifier-parity.sh,
# docs/VERIFIER_PARITY.md) compares across the Go, Python, TypeScript, and Rust
# verifiers.
_REASON_DUPLICATE_KEY = "duplicate-key"
_REASON_TRAILING_DATA = "trailing-data"
_REASON_NOT_OBJECT = "not-object"
_REASON_UNKNOWN_FIELD = "unknown-field"
_REASON_PARSE = "parse-error"
_REASON_READ = "read-error"
_REASON_MISSING_HASH = "missing-hash"
_REASON_HASH_MISMATCH = "hash-mismatch"

# The decision record's member set is closed: every object position the schema
# defines accepts only the member names the Go type declares (mirrors
# DecisionRecordV1 at the top level, CheckFailure inside check, and
# ExecutionClaim inside execution_claim). A verifier that dropped unknown
# members before hashing would accept attacker-added content under a valid
# stored hash; the record is rejected at ingest instead, matching Go's strict
# decode.
_KNOWN_FIELDS = frozenset({
    "schema_version", "event_type", "record_id", "timestamp",
    "boundary_version", "boundary_build_digest", "adapter", "agent_id",
    "tenant_id", "trace_id", "request_id", "tool", "action", "reason",
    "decision_mode",
    "matched_rule", "policy_file", "policy_bundle_hash", "request_hash",
    "raw_shape_hash", "decision_hash", "trust_score", "trust_state",
    "signature", "signature_key_id", "check",
    "adapter_id", "route_id", "topology_profile", "execution_claim",
})

_KNOWN_CLAIM_FIELDS = frozenset({"upstream_called", "executed", "source"})

# CheckFailure's serialized member set (governance/request.go): Detail is
# json:"-" and never serialized, so it is not a member.
_KNOWN_CHECK_FIELDS = frozenset({"stage", "class", "category", "cause"})

# Nesting ceiling mirroring Go's encoding/json decoder (which fails beyond
# 10,000 levels). Inputs deeper than this are rejected with parse-error
# without consuming the interpreter's recursion budget.
_MAX_JSON_DEPTH = 10_000


def _reject_duplicates(ordered_pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    """JSON object_pairs_hook that rejects a repeated member name.

    ``json.loads`` silently keeps the last value for a repeated key, which
    would let one byte stream carry two different verdicts. A decision record
    must be rejected instead. The hook runs at every nesting depth, so nested
    objects are checked too. Uniqueness is case-folded: ``"action"`` and
    ``"Action"`` in one object are a duplicate, matching the Go, TypeScript,
    and Rust verifiers.
    """
    out: dict[str, Any] = {}
    seen_folded: set[str] = set()
    for key, value in ordered_pairs:
        folded = key.lower()
        if folded in seen_folded:
            raise ValueError(f"duplicate key: {key}")
        seen_folded.add(folded)
        out[key] = value
    return out


def _reject_nonfinite(value: str) -> Any:
    """JSON ``parse_constant`` hook rejecting non-finite numbers.

    Python's stock decoder accepts ``NaN`` / ``Infinity`` / ``-Infinity`` —
    nonstandard literals that RFC 8785 cannot represent, so canonicalization
    raises ``rfc8785.FloatDomainError`` outside any error handler. Rejecting
    them here keeps the failure on the parse path (reason=parse-error), as
    the Go, TypeScript, and Rust verifiers already do.
    """
    raise ValueError(f"non-finite number: {value}")


def _enforce_depth_limit(content: str) -> None:
    """Reject input nested deeper than ``_MAX_JSON_DEPTH`` before decoding.

    Iterative scan with no recursion, so input millions of levels deep is
    rejected without exhausting the stack. Container openers inside string
    literals are skipped (a ``"{"`` in a string is not structure). Inputs
    below this cap may still exceed the interpreter's recursion ceiling;
    ``_load_record`` maps that ``RecursionError`` to parse-error too.
    """
    depth = 0
    in_string = False
    escaped = False
    for ch in content:
        if in_string:
            if escaped:
                escaped = False
            elif ch == "\\":
                escaped = True
            elif ch == '"':
                in_string = False
            continue
        if ch == '"':
            in_string = True
        elif ch == "{" or ch == "[":
            depth += 1
            if depth > _MAX_JSON_DEPTH:
                raise ValueError(
                    f"JSON nesting exceeds the {_MAX_JSON_DEPTH}-level limit"
                )
        elif ch == "}" or ch == "]":
            depth -= 1


def _load_record(path: str) -> dict[str, Any]:
    """Load and minimally validate a decision-record JSON file at ``path``.

    Strict ingest: the file must hold exactly one JSON object with unique
    member names at every depth, every member name must belong to the
    schema's closed member set, no bytes may follow the top-level value,
    non-finite numbers are rejected, and nesting is depth-limited.
    """
    with open(path, "r", encoding="utf-8") as handle:
        content = handle.read()

    _enforce_depth_limit(content)

    # JSONDecoder.decode parses exactly one value (skipping surrounding
    # whitespace) and raises on trailing non-whitespace itself;
    # object_pairs_hook rejects duplicate member names at every depth, and
    # parse_constant rejects the non-finite literals the stock decoder would
    # otherwise accept.
    decoder = json.JSONDecoder(
        object_pairs_hook=_reject_duplicates,
        parse_constant=_reject_nonfinite,
    )
    try:
        data = decoder.decode(content)
    except json.JSONDecodeError as err:
        if err.msg == "Extra data":
            raise ValueError("trailing data after JSON value") from err
        raise
    except RecursionError as err:
        # Nesting inside the cap can still exceed the interpreter's
        # recursion ceiling; classify it the same as crossing the cap.
        raise ValueError("JSON nesting exceeds the decoder's recursion limit") from err

    if not isinstance(data, dict):
        raise ValueError("decision record must be a JSON object")

    unknown = sorted(set(data) - _KNOWN_FIELDS)
    if unknown:
        raise ValueError(f"unknown field: {unknown[0]}")
    claim = data.get("execution_claim")
    if isinstance(claim, dict):
        unknown_claim = sorted(set(claim) - _KNOWN_CLAIM_FIELDS)
        if unknown_claim:
            raise ValueError(
                f"unknown field: execution_claim.{unknown_claim[0]}"
            )
    check = data.get("check")
    if isinstance(check, dict):
        unknown_check = sorted(set(check) - _KNOWN_CHECK_FIELDS)
        if unknown_check:
            raise ValueError(f"unknown field: check.{unknown_check[0]}")
    return data


def _load_error_reason(err: BaseException) -> str:
    """Map a ``_load_record`` failure to its shared rejection class."""
    if isinstance(err, OSError):
        return _REASON_READ
    message = str(err)
    if message.startswith("duplicate key:"):
        return _REASON_DUPLICATE_KEY
    if message.startswith("trailing data"):
        return _REASON_TRAILING_DATA
    if message.startswith("unknown field:"):
        return _REASON_UNKNOWN_FIELD
    if "must be a JSON object" in message:
        return _REASON_NOT_OBJECT
    return _REASON_PARSE


def _verify_error_reason(message: str) -> str:
    """Map a ``verify_record`` failure message to its shared rejection class."""
    if message.startswith("decision_hash missing or empty"):
        return _REASON_MISSING_HASH
    if message.startswith("decision_hash mismatch:"):
        return _REASON_HASH_MISMATCH
    return "verify-fail"


def main(argv: list[str]) -> int:
    """CLI entry point. Returns the process exit status (0 ok, 1 otherwise).

    On failure a ``reason=<code>`` line is written to stderr naming the shared
    machine-readable rejection class; success prints only the ok line.
    """
    if len(argv) != 2:
        sys.stderr.write("usage: python3 boundary_verify.py <record.json>\n")
        return 1

    path = argv[1]
    try:
        record = _load_record(path)
    except (OSError, ValueError, json.JSONDecodeError) as err:
        sys.stderr.write(f"error: could not load {path}: {err}\n")
        sys.stderr.write(f"reason={_load_error_reason(err)}\n")
        return 1

    try:
        ok, message = verify_record(record)
    except (RecursionError, rfc8785.CanonicalizationError, UnicodeError) as err:
        # Canonicalization recurses, and rfc8785 raises domain errors on input
        # that passed strict ingest: numbers outside the RFC 8785 range
        # (FloatDomainError for 1e999 -> inf, IntegerDomainError for integer
        # literals too large for exact IEEE-754 representation) and
        # non-UTF-8 code points such as a lone surrogate
        # (CanonicalizationError). All classify as parse-error, matching the
        # other verifiers; none may escape as an unclassified traceback.
        sys.stderr.write(f"error: could not verify {path}: {err}\n")
        sys.stderr.write(f"reason={_REASON_PARSE}\n")
        return 1
    print(message)
    if not ok:
        sys.stderr.write(f"reason={_verify_error_reason(message)}\n")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
