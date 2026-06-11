/**
 * Standalone verifier for a Fulcrum Boundary decision record.
 *
 * This script reproduces a decision record's `decision_hash` with no Boundary
 * code on the path: it reads a decision-record JSON file, canonicalizes it with
 * a stock RFC 8785 / JCS implementation (the `canonicalize` npm package — the
 * JCS reference implementation), SHA-256s the canonical bytes via node:crypto,
 * and compares the result to the record's own stored `decision_hash`.
 *
 * What this proves and does not prove
 * ------------------------------------
 * The decision record is RFC 8785 / JCS-canonicalized, and its `decision_hash`
 * is an unkeyed SHA-256 over that canonical form. Recomputing it here is an
 * INTEGRITY check: it detects whether the covered fields of the record were
 * altered after emission. It is NOT an AUTHENTICITY check — an unkeyed hash
 * does not prove who produced the record, and editing the record yields a new,
 * internally consistent hash. The optional `signature` / `signature_key_id`
 * fields (which this verifier intentionally excludes from the hash, mirroring
 * Boundary) are where authorship would be attested; this verifier does not
 * check them. A passing check is also not evidence that the governed action
 * was executed or prevented.
 *
 * How the hash is reproduced (mirrors governance/receipt.go ComputeDecisionHash)
 * -------------------------------------------------------------------------------
 * Boundary computes `decision_hash` over the record with four fields
 * neutralized first, so the hash is self-excluding and signature-excluding:
 *
 *   * `record_id`        -> set to "" (Boundary always emits this key)
 *   * `decision_hash`    -> set to "" (Boundary always emits this key)
 *   * `signature`        -> dropped  (Boundary emits this key only when set)
 *   * `signature_key_id` -> dropped  (Boundary emits this key only when set)
 *
 * then it canonicalizes the result with RFC 8785 / JCS and takes
 * `"sha256:" + hex(sha256(canonical))`.
 *
 * Usage
 * -----
 *   node --experimental-strip-types boundary_verify.ts <record.json> [more...]
 *
 * Exit status: 0 when all records pass; 1 when any record fails or cannot be
 * loaded. See README.md in this directory.
 */

import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { argv, exit, stderr, stdout } from 'node:process';
import canonicalize from 'canonicalize';

// Fields blanked to "" before hashing. Boundary always emits these keys, and
// its ComputeDecisionHash sets them to the empty string, so the canonical
// preimage contains them as "".
const BLANK_TO_EMPTY = ['record_id', 'decision_hash'] as const;

// Fields dropped entirely before hashing. Boundary declares these omitempty,
// so after blanking they are absent from the marshaled preimage. Removing the
// keys here reproduces that exactly.
const DROP = ['signature', 'signature_key_id'] as const;

type DecisionRecord = Record<string, unknown>;

/**
 * Return the decision_hash Boundary would compute for the given record.
 *
 * The caller's object is not mutated: a shallow copy is made, then
 * BLANK_TO_EMPTY keys are set to "" and DROP keys are removed. The copy is
 * canonicalized with RFC 8785 / JCS via the `canonicalize` package, then
 * SHA-256'd. Returns `"sha256:" + hex`.
 */
export function computeDecisionHash(record: DecisionRecord): string {
  const preimage: DecisionRecord = { ...record };
  for (const key of BLANK_TO_EMPTY) {
    preimage[key] = '';
  }
  for (const key of DROP) {
    delete preimage[key];
  }

  const canonical = canonicalize(preimage);
  if (canonical === undefined) {
    throw new Error('canonicalize returned undefined — record may contain undefined values');
  }
  const digest = createHash('sha256').update(canonical, 'utf8').digest('hex');
  return 'sha256:' + digest;
}

/**
 * Verify a parsed decision record.
 *
 * Returns `[ok, message]`. `ok` is true only when the record carries a
 * non-empty `decision_hash` and the recomputed hash equals it. `message` is a
 * human-readable line suitable for printing.
 */
export function verifyRecord(record: DecisionRecord): [boolean, string] {
  const stored = record['decision_hash'];
  if (!stored || typeof stored !== 'string') {
    return [false, 'decision_hash missing or empty: nothing to verify against'];
  }

  const recomputed = computeDecisionHash(record);
  if (recomputed === stored) {
    return [true, 'record verification: ok'];
  }
  return [false, `decision_hash mismatch: got ${recomputed} want ${stored}`];
}

/**
 * Load and minimally validate a decision-record JSON file at `path`.
 */
function loadRecord(path: string): DecisionRecord {
  const raw = readFileSync(path, 'utf8');
  const data: unknown = JSON.parse(raw);
  if (typeof data !== 'object' || data === null || Array.isArray(data)) {
    throw new Error('decision record must be a JSON object');
  }
  return data as DecisionRecord;
}

/**
 * CLI entry point. Accepts one or more record paths. Exits 0 when all pass,
 * 1 when any fail or cannot be loaded.
 */
function main(): number {
  const paths = argv.slice(2);
  if (paths.length === 0) {
    stderr.write('usage: node --experimental-strip-types boundary_verify.ts <record.json> [more...]\n');
    return 1;
  }

  let allOk = true;
  for (const path of paths) {
    let record: DecisionRecord;
    try {
      record = loadRecord(path);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      stderr.write(`error: could not load ${path}: ${msg}\n`);
      allOk = false;
      continue;
    }

    const [ok, message] = verifyRecord(record);
    stdout.write(message + '\n');
    if (!ok) {
      allOk = false;
    }
  }

  return allOk ? 0 : 1;
}

// Run only when this file is the entry point (not when imported by tests).
if (argv[1] !== undefined && argv[1].endsWith('boundary_verify.ts')) {
  exit(main());
}
