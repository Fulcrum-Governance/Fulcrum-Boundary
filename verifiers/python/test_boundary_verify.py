#!/usr/bin/env python3
"""Tests for the standalone Boundary decision-record verifier.

Pure standard library plus the ``rfc8785`` dependency (the same dependency the
verifier itself needs). Run directly:

    pip install rfc8785
    python3 verifiers/python/test_boundary_verify.py

It asserts three things:

  1. The committed example decision record
     (docs/examples/decision-record.example.json) verifies ok and the CLI exits
     0.
  2. A one-field forgery -- flipping ``"action": "deny"`` to ``"action":
     "allow"`` -- is caught: the recomputed hash no longer matches, the verifier
     reports a mismatch, and the CLI exits 1.
  3. Every record in the shared Go/Python conformance corpus
     (tests/conformance/testdata/verifier-vectors/) recomputes to exactly its
     committed ``decision_hash``. This is the SAME corpus the Go conformance gate
     asserts, so the two implementations are pinned to one source of truth.

On success it prints a summary and exits 0; any failed assertion raises and
exits non-zero.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

# Import the verifier module that lives next to this test.
sys.path.insert(0, str(Path(__file__).resolve().parent))
import boundary_verify  # noqa: E402  (path set up immediately above)

# Repo root is three levels up from this file: verifiers/python/<this>.
REPO_ROOT = Path(__file__).resolve().parents[2]
EXAMPLE_RECORD = REPO_ROOT / "docs" / "examples" / "decision-record.example.json"
CORPUS_DIR = REPO_ROOT / "tests" / "conformance" / "testdata" / "verifier-vectors"
MANIFEST = CORPUS_DIR / "manifest.json"
VERIFIER = Path(__file__).resolve().parent / "boundary_verify.py"


def _run_cli(record_path: Path) -> subprocess.CompletedProcess[str]:
    """Run the verifier CLI on ``record_path`` and capture its result."""
    return subprocess.run(
        [sys.executable, str(VERIFIER), str(record_path)],
        capture_output=True,
        text=True,
        check=False,
    )


def test_example_record_verifies_ok() -> None:
    """The committed example record verifies in-process and via the CLI (exit 0)."""
    with open(EXAMPLE_RECORD, "r", encoding="utf-8") as handle:
        record = json.load(handle)

    ok, message = boundary_verify.verify_record(record)
    assert ok, f"example record failed in-process verification: {message}"
    assert message == "record verification: ok", message

    result = _run_cli(EXAMPLE_RECORD)
    assert result.returncode == 0, (
        f"CLI exit {result.returncode} on clean record\n"
        f"stdout: {result.stdout}\nstderr: {result.stderr}"
    )
    assert "record verification: ok" in result.stdout, result.stdout
    print("ok: example record verifies (in-process + CLI exit 0)")


def test_one_field_forgery_is_caught() -> None:
    """Flipping action deny->allow must produce a mismatch and CLI exit 1."""
    with open(EXAMPLE_RECORD, "r", encoding="utf-8") as handle:
        record = json.load(handle)

    assert record.get("action") == "deny", (
        "example record is expected to be a deny; update this test if it changed"
    )

    forged = dict(record)
    forged["action"] = "allow"  # the one-field forgery.

    ok, message = boundary_verify.verify_record(forged)
    assert not ok, "forged record unexpectedly verified ok"
    assert message.startswith("decision_hash mismatch:"), message

    # And through the CLI, against a forged file on disk: exit 1 + mismatch line.
    with tempfile.TemporaryDirectory() as tmp:
        forged_path = Path(tmp) / "forged.json"
        with open(forged_path, "w", encoding="utf-8") as handle:
            json.dump(forged, handle)
        result = _run_cli(forged_path)

    assert result.returncode == 1, (
        f"CLI exit {result.returncode} on forged record; expected 1\n"
        f"stdout: {result.stdout}\nstderr: {result.stderr}"
    )
    assert "decision_hash mismatch:" in result.stdout, result.stdout
    print("ok: one-field forgery (action deny->allow) caught (mismatch + CLI exit 1)")


def test_conformance_corpus_recomputes_to_committed_hashes() -> None:
    """Every corpus vector recomputes to its committed decision_hash.

    This reads the SAME files the Go conformance gate asserts, via the committed
    manifest, so Go and Python are bound to one shared source of truth.
    """
    assert MANIFEST.exists(), (
        f"conformance manifest missing at {MANIFEST}; generate the corpus with "
        f"BOUNDARY_WRITE_VECTORS=1 go test ./tests/conformance/"
    )
    with open(MANIFEST, "r", encoding="utf-8") as handle:
        manifest = json.load(handle)

    vectors = manifest.get("vectors", [])
    assert vectors, "manifest lists no vectors"

    checked = 0
    for entry in vectors:
        file_name = entry["file"]
        expected_hash = entry["decision_hash"]
        record_path = CORPUS_DIR / file_name
        assert record_path.exists(), f"corpus file missing: {record_path}"

        with open(record_path, "r", encoding="utf-8") as handle:
            record = json.load(handle)

        # The file's own stored decision_hash must match the manifest.
        assert record.get("decision_hash") == expected_hash, (
            f"{file_name}: stored decision_hash {record.get('decision_hash')} "
            f"!= manifest {expected_hash}"
        )

        # And the Python re-implementation must reproduce that exact hash.
        recomputed = boundary_verify.compute_decision_hash(record)
        assert recomputed == expected_hash, (
            f"{file_name} ({entry.get('why', '')}):\n"
            f"  recomputed: {recomputed}\n"
            f"  committed:  {expected_hash}"
        )

        ok, message = boundary_verify.verify_record(record)
        assert ok, f"{file_name}: verify_record failed: {message}"
        checked += 1

    assert checked == len(vectors)
    print(f"ok: all {checked} conformance corpus vectors recompute to committed hashes")


def main() -> int:
    """Run every test function; return 0 if all pass."""
    tests = [
        test_example_record_verifies_ok,
        test_one_field_forgery_is_caught,
        test_conformance_corpus_recomputes_to_committed_hashes,
    ]
    for test in tests:
        test()
    print(f"\nPASS: {len(tests)} test functions, all assertions held")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
