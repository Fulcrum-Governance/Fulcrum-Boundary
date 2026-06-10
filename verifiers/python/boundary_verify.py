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


def _load_record(path: str) -> dict[str, Any]:
    """Load and minimally validate a decision-record JSON file at ``path``."""
    with open(path, "r", encoding="utf-8") as handle:
        data = json.load(handle)
    if not isinstance(data, dict):
        raise ValueError("decision record must be a JSON object")
    return data


def main(argv: list[str]) -> int:
    """CLI entry point. Returns the process exit status (0 ok, 1 otherwise)."""
    if len(argv) != 2:
        sys.stderr.write("usage: python3 boundary_verify.py <record.json>\n")
        return 1

    path = argv[1]
    try:
        record = _load_record(path)
    except (OSError, ValueError, json.JSONDecodeError) as err:
        sys.stderr.write(f"error: could not load {path}: {err}\n")
        return 1

    ok, message = verify_record(record)
    print(message)
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
