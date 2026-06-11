/**
 * Tests for the standalone Boundary decision-record verifier (TypeScript).
 *
 * Ports every test case in verifiers/python/test_boundary_verify.py.
 * Run with:
 *
 *   node --experimental-strip-types --test boundary_verify.test.ts
 *
 * It asserts three things:
 *
 *   1. The committed example decision record
 *      (docs/examples/decision-record.example.json) verifies ok and the CLI
 *      exits 0.
 *   2. A one-field forgery — flipping `"action": "deny"` to
 *      `"action": "allow"` — is caught: the recomputed hash no longer matches,
 *      the verifier reports a mismatch, and the CLI exits 1.
 *   3. Every record in the shared Go/Python conformance corpus
 *      (tests/conformance/testdata/verifier-vectors/) recomputes to exactly its
 *      committed `decision_hash`. This is the SAME corpus the Go conformance
 *      gate asserts, so Go, Python, and TypeScript are pinned to one source of
 *      truth.
 */

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, writeFileSync, mkdtempSync, rmSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';

import { computeDecisionHash, verifyRecord } from './boundary_verify.ts';

// Repo root: verifiers/typescript/<this file> -> up two levels.
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const REPO_ROOT = resolve(__dirname, '..', '..');

const EXAMPLE_RECORD = join(REPO_ROOT, 'docs', 'examples', 'decision-record.example.json');
const CORPUS_DIR = join(REPO_ROOT, 'tests', 'conformance', 'testdata', 'verifier-vectors');
const MANIFEST = join(CORPUS_DIR, 'manifest.json');
const VERIFIER = join(__dirname, 'boundary_verify.ts');

type DecisionRecord = Record<string, unknown>;

function runCli(recordPath: string): { stdout: string; stderr: string; status: number } {
  const result = spawnSync(
    process.execPath,
    ['--experimental-strip-types', VERIFIER, recordPath],
    { encoding: 'utf8' },
  );
  return {
    stdout: result.stdout ?? '',
    stderr: result.stderr ?? '',
    status: result.status ?? 1,
  };
}

// ── Test 1: example record verifies ok ───────────────────────────────────────

test('example record verifies in-process (ok)', () => {
  const record: DecisionRecord = JSON.parse(readFileSync(EXAMPLE_RECORD, 'utf8'));

  const [ok, message] = verifyRecord(record);
  assert.ok(ok, `example record failed in-process verification: ${message}`);
  assert.equal(message, 'record verification: ok');
});

test('example record verifies via CLI (exit 0)', () => {
  const result = runCli(EXAMPLE_RECORD);
  assert.equal(
    result.status,
    0,
    `CLI exit ${result.status} on clean record\nstdout: ${result.stdout}\nstderr: ${result.stderr}`,
  );
  assert.ok(
    result.stdout.includes('record verification: ok'),
    `expected "record verification: ok" in stdout, got: ${result.stdout}`,
  );
});

// ── Test 2: one-field forgery is caught ──────────────────────────────────────

test('tampered action (deny->allow) fails in-process', () => {
  const record: DecisionRecord = JSON.parse(readFileSync(EXAMPLE_RECORD, 'utf8'));
  assert.equal(
    record['action'],
    'deny',
    'example record is expected to be a deny; update this test if it changed',
  );

  const forged: DecisionRecord = { ...record, action: 'allow' };
  const [ok, message] = verifyRecord(forged);
  assert.ok(!ok, 'forged record (action deny->allow) unexpectedly verified ok');
  assert.ok(
    message.startsWith('decision_hash mismatch:'),
    `expected message to start with "decision_hash mismatch:", got: ${message}`,
  );
});

test('tampered action (deny->allow) fails via CLI (exit 1)', () => {
  const record: DecisionRecord = JSON.parse(readFileSync(EXAMPLE_RECORD, 'utf8'));
  const forged: DecisionRecord = { ...record, action: 'allow' };

  const tmp = mkdtempSync(join(tmpdir(), 'boundary-test-'));
  const forgedPath = join(tmp, 'forged.json');
  try {
    writeFileSync(forgedPath, JSON.stringify(forged));
    const result = runCli(forgedPath);
    assert.equal(
      result.status,
      1,
      `CLI exit ${result.status} on forged record; expected 1\nstdout: ${result.stdout}\nstderr: ${result.stderr}`,
    );
    assert.ok(
      result.stdout.includes('decision_hash mismatch:'),
      `expected "decision_hash mismatch:" in stdout, got: ${result.stdout}`,
    );
  } finally {
    rmSync(tmp, { recursive: true, force: true });
  }
});

// ── Test 3: tampered reason is caught ────────────────────────────────────────

test('tampered reason fails', () => {
  const record: DecisionRecord = JSON.parse(readFileSync(EXAMPLE_RECORD, 'utf8'));
  const forged: DecisionRecord = { ...record, reason: 'i did not say that' };
  const [ok, message] = verifyRecord(forged);
  assert.ok(!ok, 'forged record (tampered reason) unexpectedly verified ok');
  assert.ok(
    message.startsWith('decision_hash mismatch:'),
    `expected mismatch message, got: ${message}`,
  );
});

// ── Test 4: decision_hash mismatch in stored value ────────────────────────────

test('stored decision_hash mismatch is caught', () => {
  const record: DecisionRecord = JSON.parse(readFileSync(EXAMPLE_RECORD, 'utf8'));
  const forged: DecisionRecord = { ...record, decision_hash: 'sha256:0000000000000000000000000000000000000000000000000000000000000000' };
  const [ok, message] = verifyRecord(forged);
  assert.ok(!ok, 'record with wrong decision_hash unexpectedly verified ok');
  assert.ok(
    message.startsWith('decision_hash mismatch:'),
    `expected mismatch message, got: ${message}`,
  );
});

// ── Test 5: missing decision_hash ────────────────────────────────────────────

test('missing decision_hash returns failure', () => {
  const record: DecisionRecord = JSON.parse(readFileSync(EXAMPLE_RECORD, 'utf8'));
  const { decision_hash: _removed, ...withoutHash } = record as DecisionRecord & { decision_hash: unknown };
  const [ok, message] = verifyRecord(withoutHash as DecisionRecord);
  assert.ok(!ok, 'record with missing decision_hash unexpectedly verified ok');
  assert.ok(
    message.includes('missing or empty'),
    `expected "missing or empty" message, got: ${message}`,
  );
});

// ── Test 6: signature fields are excluded from the hash ──────────────────────

test('signature fields are excluded from the hash', () => {
  const record: DecisionRecord = JSON.parse(readFileSync(EXAMPLE_RECORD, 'utf8'));
  // Adding signature and signature_key_id must not change the hash.
  const withSig: DecisionRecord = {
    ...record,
    signature: 'some-opaque-signature-value',
    signature_key_id: 'key-2026-01',
  };
  const hashOriginal = computeDecisionHash(record);
  const hashWithSig = computeDecisionHash(withSig);
  assert.equal(
    hashWithSig,
    hashOriginal,
    'signature fields must not affect decision_hash (they are excluded before hashing)',
  );
});

// ── Test 7: all 9 conformance corpus vectors ──────────────────────────────────

test('conformance corpus: all vectors recompute to committed hashes', () => {
  type ManifestEntry = { file: string; decision_hash: string; why?: string };
  type Manifest = { vectors: ManifestEntry[] };

  assert.ok(
    (() => { try { readFileSync(MANIFEST); return true; } catch { return false; } })(),
    `conformance manifest missing at ${MANIFEST}; generate with BOUNDARY_WRITE_VECTORS=1 go test ./tests/conformance/`,
  );

  const manifest: Manifest = JSON.parse(readFileSync(MANIFEST, 'utf8'));
  const vectors = manifest.vectors;
  assert.ok(vectors && vectors.length > 0, 'manifest lists no vectors');

  let checked = 0;
  for (const entry of vectors) {
    const { file: fileName, decision_hash: expectedHash, why } = entry;
    const recordPath = join(CORPUS_DIR, fileName);

    const record: DecisionRecord = JSON.parse(readFileSync(recordPath, 'utf8'));

    // The file's own stored decision_hash must match the manifest.
    assert.equal(
      record['decision_hash'],
      expectedHash,
      `${fileName}: stored decision_hash ${record['decision_hash']} != manifest ${expectedHash}`,
    );

    // The TypeScript re-implementation must reproduce that exact hash.
    const recomputed = computeDecisionHash(record);
    assert.equal(
      recomputed,
      expectedHash,
      `${fileName} (${why ?? ''}):\n  recomputed: ${recomputed}\n  committed:  ${expectedHash}`,
    );

    const [ok, message] = verifyRecord(record);
    assert.ok(ok, `${fileName}: verifyRecord failed: ${message}`);

    checked++;
  }

  assert.equal(checked, vectors.length, `checked ${checked} but manifest has ${vectors.length}`);
});
