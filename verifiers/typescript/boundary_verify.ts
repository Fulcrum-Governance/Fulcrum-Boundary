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

// Machine-readable rejection classes, emitted on stderr as `reason=<code>`
// when verification or parsing fails. This is the shared vocabulary the
// cross-language verifier parity harness (scripts/ci/run-verifier-parity.sh,
// docs/VERIFIER_PARITY.md) compares across the Go, Python, TypeScript, and
// Rust verifiers.
const REASON_DUPLICATE_KEY = 'duplicate-key';
const REASON_TRAILING_DATA = 'trailing-data';
const REASON_NOT_OBJECT = 'not-object';
const REASON_UNKNOWN_FIELD = 'unknown-field';
const REASON_PARSE = 'parse-error';
const REASON_READ = 'read-error';
const REASON_MISSING_HASH = 'missing-hash';
const REASON_HASH_MISMATCH = 'hash-mismatch';

/**
 * The decision record's member set is closed: every object position the
 * schema defines accepts only the member names the Go type declares
 * (DecisionRecordV1 at the top level, CheckFailure inside check, and
 * ExecutionClaim inside execution_claim). A verifier that dropped unknown
 * members before hashing would accept attacker-added content under a valid
 * stored hash; the record is rejected at ingest instead, matching Go's
 * strict decode.
 */
const KNOWN_FIELDS = new Set([
  'schema_version', 'event_type', 'record_id', 'timestamp',
  'boundary_version', 'boundary_build_digest', 'adapter', 'agent_id',
  'tenant_id', 'trace_id', 'request_id', 'tool', 'action', 'reason',
  'decision_mode',
  'matched_rule', 'policy_file', 'policy_bundle_hash', 'request_hash',
  'raw_shape_hash', 'decision_hash', 'trust_score', 'trust_state',
  'signature', 'signature_key_id', 'check',
  'adapter_id', 'route_id', 'topology_profile', 'execution_claim',
]);

const KNOWN_CLAIM_FIELDS = new Set(['upstream_called', 'executed', 'source']);

// CheckFailure's serialized member set (governance/request.go): Detail is
// json:"-" and never serialized, so it is not a member.
const KNOWN_CHECK_FIELDS = new Set(['stage', 'class', 'category', 'cause']);

/**
 * Nesting ceiling mirroring Go's encoding/json decoder (which fails beyond
 * 10,000 levels). Inputs deeper than this are rejected with parse-error
 * before JSON.parse or canonicalization can exhaust the call stack.
 */
const MAX_JSON_DEPTH = 10_000;

class DuplicateKeyError extends Error {}
class TrailingDataError extends Error {}
class UnknownFieldError extends Error {}
class DepthLimitError extends Error {}
class CanonicalizationError extends Error {}

const JSON_ESCAPES: Record<string, string> = {
  '"': '"',
  '\\': '\\',
  '/': '/',
  b: '\b',
  f: '\f',
  n: '\n',
  r: '\r',
  t: '\t',
};

const JSON_WHITESPACE = new Set([' ', '\t', '\n', '\r']);
const JSON_VALUE_END = new Set([',', '}', ']', ' ', '\t', '\n', '\r']);

/**
 * Scan raw JSON text and throw when any object repeats a member name at any
 * depth, or when non-whitespace bytes follow the top-level value. This is a
 * pre-flight pass ahead of JSON.parse, which silently keeps the last value
 * for a repeated key — fine for config files, unacceptable for a decision
 * record whose bytes must mean exactly one thing.
 *
 * The scanner only needs to be correct on well-formed JSON; on malformed
 * input it may bail early, because JSON.parse runs afterward and produces
 * the authoritative syntax error. Its two affirmative rejections —
 * DuplicateKeyError and TrailingDataError — are the ones callers classify.
 */
function assertStrictJson(raw: string): void {
  let i = 0;
  const n = raw.length;
  // Set when the scanner hits a construct it does not fully understand; the
  // trailing-data verdict then stays with JSON.parse, which reports it as a
  // syntax error rather than mislabeled trailing data.
  let bailed = false;
  const bail = () => {
    bailed = true;
  };

  const skipWs = () => {
    while (i < n && JSON_WHITESPACE.has(raw[i])) i++;
  };

  // readString assumes raw[i] === '"'; returns the decoded string value.
  const readString = (): string => {
    i++;
    let out = '';
    while (i < n) {
      const c = raw[i];
      if (c === '"') {
        i++;
        return out;
      }
      if (c === '\\') {
        const esc = raw[i + 1];
        if (esc === 'u') {
          out += String.fromCharCode(parseInt(raw.slice(i + 2, i + 6), 16));
          i += 6;
        } else {
          out += JSON_ESCAPES[esc] ?? esc;
          i += 2;
        }
        continue;
      }
      out += c;
      i++;
    }
    throw new Error('unterminated string');
  };

  const readValue = (depth: number): void => {
    skipWs();
    if (depth > MAX_JSON_DEPTH) {
      throw new DepthLimitError(`JSON nesting exceeds the ${MAX_JSON_DEPTH}-level limit`);
    }
    const c = raw[i];
    if (c === '{') {
      i++;
      const keys = new Set<string>();
      skipWs();
      if (raw[i] === '}') {
        i++;
        return;
      }
      for (;;) {
        skipWs();
        const key = readString();
        // Uniqueness is case-folded: "action" and "Action" in one object
        // are a duplicate, matching the Go, Python, and Rust verifiers.
        const folded = key.toLowerCase();
        if (keys.has(folded)) {
          throw new DuplicateKeyError(`duplicate key: ${key}`);
        }
        keys.add(folded);
        skipWs();
        if (raw[i] !== ':') {
          bail();
          return;
        }
        i++;
        readValue(depth + 1);
        if (bailed) return;
        skipWs();
        if (raw[i] === ',') {
          i++;
          continue;
        }
        if (raw[i] === '}') {
          i++;
          return;
        }
        bail();
        return;
      }
    }
    if (c === '[') {
      i++;
      skipWs();
      if (raw[i] === ']') {
        i++;
        return;
      }
      for (;;) {
        readValue(depth + 1);
        if (bailed) return;
        skipWs();
        if (raw[i] === ',') {
          i++;
          continue;
        }
        if (raw[i] === ']') {
          i++;
          return;
        }
        bail();
        return;
      }
    }
    if (c === '"') {
      readString();
      return;
    }
    if (c === '-' || (c >= '0' && c <= '9') || c === 't' || c === 'f' || c === 'n') {
      while (i < n && !JSON_VALUE_END.has(raw[i])) i++; // number or literal
      return;
    }
    // A character that cannot start any JSON value — a leading BOM, stray
    // punctuation — is not a token at all. Bail so JSON.parse reports the
    // syntax error as parse-error instead of the scanner consuming it as a
    // literal and mislabeling the remainder as trailing data.
    bail();
  };

  try {
    readValue(0);
    skipWs();
    if (!bailed && i < n) {
      throw new TrailingDataError('trailing data after JSON value');
    }
  } catch (err) {
    // The affirmative rejections propagate, and so does a RangeError — the
    // scanner itself can exhaust the call stack on hostile input below the
    // MAX_JSON_DEPTH cap, and swallowing it would misreport the failure.
    // Anything else stays with JSON.parse, which reports it precisely.
    if (
      err instanceof DuplicateKeyError ||
      err instanceof TrailingDataError ||
      err instanceof DepthLimitError ||
      err instanceof RangeError
    ) {
      throw err;
    }
  }
}

/** Map a load failure to its shared rejection class. */
function loadErrorReason(err: unknown): string {
  if (err instanceof DuplicateKeyError) return REASON_DUPLICATE_KEY;
  if (err instanceof TrailingDataError) return REASON_TRAILING_DATA;
  if (err instanceof UnknownFieldError) return REASON_UNKNOWN_FIELD;
  if (err instanceof DepthLimitError) return REASON_PARSE;
  const message = err instanceof Error ? err.message : String(err);
  if (message.includes('must be a JSON object')) return REASON_NOT_OBJECT;
  if (
    typeof err === 'object' &&
    err !== null &&
    'code' in err &&
    typeof (err as { code: unknown }).code === 'string'
  ) {
    return REASON_READ; // Node system errors (ENOENT, EACCES, ...)
  }
  return REASON_PARSE;
}

/** Map a verifyRecord failure message to its shared rejection class. */
function verifyErrorReason(message: string): string {
  if (message.startsWith('decision_hash missing or empty')) return REASON_MISSING_HASH;
  if (message.startsWith('decision_hash mismatch:')) return REASON_HASH_MISMATCH;
  return 'verify-fail';
}

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

  let canonical: string | undefined;
  try {
    canonical = canonicalize(preimage);
  } catch (err) {
    // canonicalize throws on values outside the RFC 8785 domain — a number
    // JSON.parse read as Infinity (e.g. "1e999") is the reachable case.
    // Classify it as a parse failure, not a verification failure: the input
    // cannot be represented in the canonical form at all, matching the
    // parse-error the other verifiers emit for the same bytes.
    throw new CanonicalizationError(
      `canonicalize failed: ${err instanceof Error ? err.message : String(err)}`,
    );
  }
  if (canonical === undefined) {
    throw new CanonicalizationError('canonicalize returned undefined — record may contain undefined values');
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
 *
 * Strict ingest: exactly one JSON object, unique member names at every
 * depth, member names confined to the schema's closed member set, no
 * trailing bytes, and nesting capped at MAX_JSON_DEPTH.
 */
function loadRecord(path: string): DecisionRecord {
  const raw = readFileSync(path, 'utf8');
  assertStrictJson(raw);
  const data: unknown = JSON.parse(raw);
  if (typeof data !== 'object' || data === null || Array.isArray(data)) {
    throw new Error('decision record must be a JSON object');
  }
  for (const key of Object.keys(data)) {
    if (!KNOWN_FIELDS.has(key)) {
      throw new UnknownFieldError(`unknown field: ${key}`);
    }
  }
  const claim = (data as DecisionRecord)['execution_claim'];
  if (typeof claim === 'object' && claim !== null && !Array.isArray(claim)) {
    for (const key of Object.keys(claim)) {
      if (!KNOWN_CLAIM_FIELDS.has(key)) {
        throw new UnknownFieldError(`unknown field: execution_claim.${key}`);
      }
    }
  }
  const check = (data as DecisionRecord)['check'];
  if (typeof check === 'object' && check !== null && !Array.isArray(check)) {
    for (const key of Object.keys(check)) {
      if (!KNOWN_CHECK_FIELDS.has(key)) {
        throw new UnknownFieldError(`unknown field: check.${key}`);
      }
    }
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
      stderr.write(`reason=${loadErrorReason(err)}\n`);
      allOk = false;
      continue;
    }

    let ok: boolean;
    let message: string;
    try {
      [ok, message] = verifyRecord(record);
    } catch (err) {
      // Canonicalization can still fail on inputs that passed strict
      // ingest: stack exhaustion (RangeError, the JCS library recurses) or
      // an unrepresentable value (CanonicalizationError). Both classify as
      // parse-error; anything else stays verify-fail — fail closed with a
      // classified reason rather than an uncaught exception.
      const msg = err instanceof Error ? err.message : String(err);
      stderr.write(`error: could not verify ${path}: ${msg}\n`);
      stderr.write(
        `reason=${err instanceof RangeError || err instanceof CanonicalizationError ? REASON_PARSE : 'verify-fail'}\n`,
      );
      allOk = false;
      continue;
    }
    stdout.write(message + '\n');
    if (!ok) {
      stderr.write(`reason=${verifyErrorReason(message)}\n`);
      allOk = false;
    }
  }

  return allOk ? 0 : 1;
}

// Run only when this file is the entry point (not when imported by tests).
if (argv[1] !== undefined && argv[1].endsWith('boundary_verify.ts')) {
  exit(main());
}
