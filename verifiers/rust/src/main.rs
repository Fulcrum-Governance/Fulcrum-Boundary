//! Standalone verifier for Fulcrum Boundary decision records.
//!
//! Recomputes `decision_hash` with no Boundary code on the path: reads a
//! decision-record JSON file, canonicalizes it with a stock RFC 8785 / JCS
//! implementation ([`serde_jcs`]), SHA-256s the canonical bytes, and compares
//! the result to the record's own stored `decision_hash`.
//!
//! # What this proves and does not prove
//!
//! The decision record is RFC 8785 / JCS-canonicalized, and its
//! `decision_hash` is an unkeyed SHA-256 over that canonical form.
//! Recomputing it here is an **integrity** check: it detects whether the
//! covered fields of the record were altered after emission. It is **not** an
//! **authenticity** check — an unkeyed hash does not prove who produced the
//! record, and editing the record yields a new, internally consistent hash.
//! The optional `signature` / `signature_key_id` fields (which this verifier
//! intentionally excludes from the hash, mirroring Boundary) are where
//! authorship would be attested; this verifier does not check them. A passing
//! check is also not evidence that the governed action was executed or
//! prevented.
//!
//! # How the hash is reproduced (mirrors governance/receipt.go ComputeDecisionHash)
//!
//! Boundary computes `decision_hash` over the record with four fields
//! neutralized first, so the hash is self-excluding and signature-excluding:
//!
//! * `record_id`         -> set to `""` (Boundary always emits this key)
//! * `decision_hash`     -> set to `""` (Boundary always emits this key)
//! * `signature`         -> dropped  (Boundary emits this key only when set)
//! * `signature_key_id`  -> dropped  (Boundary emits this key only when set)
//!
//! Then canonicalize with RFC 8785 / JCS and take `"sha256:" + hex(sha256(canonical))`.
//!
//! # Float formatting
//!
//! `serde_jcs` uses `ryu-js` for ECMAScript shortest-round-trip number
//! formatting per RFC 8785 §3.2.4, so a `trust_score` of `1.0/3.0` serializes
//! as `0.3333333333333333` — the same value Boundary's Go implementation emits.
//! The `v1_float_trust_score.json` conformance vector is the regression proof
//! for this path.
//!
//! # Usage
//!
//! ```text
//! cargo run --manifest-path verifiers/rust/Cargo.toml -- <record.json> [more...]
//! ```
//!
//! Exit status: 0 when every supplied record's recomputed hash equals its
//! stored `decision_hash`; 1 on any mismatch, missing/empty `decision_hash`,
//! or load error.

use serde::de::{self, DeserializeSeed, Deserializer, MapAccess, SeqAccess, Visitor};
use sha2::{Digest, Sha256};
use std::{env, fmt, fs, process};

// Fields blanked to "" before hashing. Boundary always emits these keys, and
// its ComputeDecisionHash sets them to the empty string, so the canonical
// preimage contains them as "".
const BLANK_TO_EMPTY: &[&str] = &["record_id", "decision_hash"];

// Fields dropped entirely before hashing. Boundary declares these omitempty,
// so after blanking they are absent from the marshaled preimage.
const DROP: &[&str] = &["signature", "signature_key_id"];

/// Recompute the `decision_hash` Boundary would produce for a parsed record.
///
/// Applies Boundary's field neutralization (blank `record_id` /
/// `decision_hash` to `""`, drop `signature` / `signature_key_id`),
/// canonicalizes with RFC 8785 / JCS via `serde_jcs`, and returns
/// `"sha256:" + hex(sha256(canonical_bytes))`.
///
/// The caller's value is not mutated; a shallow clone of the top-level object
/// map is made before modification.
pub fn compute_decision_hash(record: &serde_json::Value) -> Result<String, String> {
    let obj = record
        .as_object()
        .ok_or_else(|| "decision record must be a JSON object".to_string())?;

    // Shallow clone so the caller's value is not mutated.
    let mut preimage: serde_json::Map<String, serde_json::Value> = obj.clone();

    for key in BLANK_TO_EMPTY {
        preimage.insert(key.to_string(), serde_json::Value::String(String::new()));
    }
    for key in DROP {
        preimage.remove(*key);
    }

    // serde_jcs::to_vec accepts any T: Serialize. serde_json::Value (and Map)
    // implement Serialize, so we pass the preimage map directly — no
    // hand-rolled canonicalization.
    let canonical = serde_jcs::to_vec(&preimage)
        .map_err(|e| format!("JCS canonicalization failed: {e}"))?;

    let digest = Sha256::digest(&canonical);
    Ok(format!("sha256:{}", hex::encode(digest)))
}

/// Verify a parsed decision record.
///
/// Returns `(ok, message)`. `ok` is true only when the record carries a
/// non-empty `decision_hash` and the recomputed hash equals it. `message` is
/// a human-readable line suitable for printing to stdout.
pub fn verify_record(record: &serde_json::Value) -> (bool, String) {
    let stored = match record.get("decision_hash").and_then(|v| v.as_str()) {
        Some(s) if !s.is_empty() => s.to_string(),
        _ => {
            return (
                false,
                "decision_hash missing or empty: nothing to verify against".to_string(),
            )
        }
    };

    match compute_decision_hash(record) {
        Ok(recomputed) if recomputed == stored => (true, "record verification: ok".to_string()),
        Ok(recomputed) => (
            false,
            format!("decision_hash mismatch: got {recomputed} want {stored}"),
        ),
        Err(e) => (false, format!("error computing hash: {e}")),
    }
}

/// Machine-readable rejection classes, printed on stderr as `reason=<code>`
/// when verification or parsing fails. This is the shared vocabulary the
/// cross-language verifier parity harness (scripts/ci/run-verifier-parity.sh,
/// docs/VERIFIER_PARITY.md) compares across the Go, Python, TypeScript, and
/// Rust verifiers.
const REASON_DUPLICATE_KEY: &str = "duplicate-key";
const REASON_TRAILING_DATA: &str = "trailing-data";
const REASON_NOT_OBJECT: &str = "not-object";
const REASON_UNKNOWN_FIELD: &str = "unknown-field";
const REASON_PARSE: &str = "parse-error";
const REASON_READ: &str = "read-error";
const REASON_MISSING_HASH: &str = "missing-hash";
const REASON_HASH_MISMATCH: &str = "hash-mismatch";

/// The decision record's member set is closed: every object position the
/// schema defines accepts only the member names the Go type declares
/// (DecisionRecordV1 at the top level, ExecutionClaim inside
/// execution_claim). A verifier that dropped unknown members before hashing
/// would accept attacker-added content under a valid stored hash; the record
/// is rejected at ingest instead, matching Go's strict decode.
const KNOWN_FIELDS: &[&str] = &[
    "schema_version", "event_type", "record_id", "timestamp",
    "boundary_version", "boundary_build_digest", "adapter", "agent_id",
    "tenant_id", "trace_id", "tool", "action", "reason", "decision_mode",
    "matched_rule", "policy_file", "policy_bundle_hash", "request_hash",
    "raw_shape_hash", "decision_hash", "trust_score", "trust_state",
    "signature", "signature_key_id",
    "adapter_id", "route_id", "topology_profile", "execution_claim",
];

const KNOWN_CLAIM_FIELDS: &[&str] = &["upstream_called", "executed", "source"];

/// A load failure carrying its shared machine-readable rejection class.
struct LoadError {
    reason: &'static str,
    message: String,
}

impl fmt::Display for LoadError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.message)
    }
}

/// Strict JSON value decoding: identical to `serde_json::Value` semantics
/// except that a repeated member name in any object is an error instead of a
/// silent last-wins overwrite. serde_json's own deserializer keeps the last
/// value for a repeated key — fine for config files, unacceptable for a
/// decision record whose bytes must mean exactly one thing.
struct StrictValueVisitor;

impl<'de> Visitor<'de> for StrictValueVisitor {
    type Value = serde_json::Value;

    fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
        formatter.write_str("any JSON value with unique object member names")
    }

    fn visit_bool<E>(self, value: bool) -> Result<serde_json::Value, E> {
        Ok(serde_json::Value::Bool(value))
    }

    fn visit_i64<E>(self, value: i64) -> Result<serde_json::Value, E> {
        Ok(serde_json::Value::Number(value.into()))
    }

    fn visit_u64<E>(self, value: u64) -> Result<serde_json::Value, E> {
        Ok(serde_json::Value::Number(value.into()))
    }

    fn visit_f64<E>(self, value: f64) -> Result<serde_json::Value, E>
    where
        E: de::Error,
    {
        // Mirror serde_json: a non-finite float is unrepresentable and is
        // replaced with null. serde_json's own parser cannot produce one.
        Ok(serde_json::Number::from_f64(value).map_or(serde_json::Value::Null, serde_json::Value::Number))
    }

    fn visit_str<E>(self, value: &str) -> Result<serde_json::Value, E>
    where
        E: de::Error,
    {
        Ok(serde_json::Value::String(value.to_owned()))
    }

    fn visit_string<E>(self, value: String) -> Result<serde_json::Value, E> {
        Ok(serde_json::Value::String(value))
    }

    fn visit_none<E>(self) -> Result<serde_json::Value, E> {
        Ok(serde_json::Value::Null)
    }

    fn visit_unit<E>(self) -> Result<serde_json::Value, E> {
        Ok(serde_json::Value::Null)
    }

    fn visit_seq<A>(self, mut seq: A) -> Result<serde_json::Value, A::Error>
    where
        A: SeqAccess<'de>,
    {
        let mut items = Vec::new();
        while let Some(item) = seq.next_element_seed(StrictValueSeed)? {
            items.push(item);
        }
        Ok(serde_json::Value::Array(items))
    }

    fn visit_map<A>(self, mut map: A) -> Result<serde_json::Value, A::Error>
    where
        A: MapAccess<'de>,
    {
        let mut object = serde_json::Map::new();
        while let Some(key) = map.next_key::<String>()? {
            if object.contains_key(&key) {
                return Err(de::Error::custom(format!("duplicate key: {key}")));
            }
            let value = map.next_value_seed(StrictValueSeed)?;
            object.insert(key, value);
        }
        Ok(serde_json::Value::Object(object))
    }
}

struct StrictValueSeed;

impl<'de> DeserializeSeed<'de> for StrictValueSeed {
    type Value = serde_json::Value;

    fn deserialize<D>(self, deserializer: D) -> Result<Self::Value, D::Error>
    where
        D: Deserializer<'de>,
    {
        deserializer.deserialize_any(StrictValueVisitor)
    }
}

/// Parse one complete JSON value from `contents` with strict member-name
/// uniqueness, rejecting trailing data after the top-level value.
fn parse_strict(contents: &str, path: &str) -> Result<serde_json::Value, LoadError> {
    let mut deserializer = serde_json::Deserializer::from_str(contents);
    let value = StrictValueSeed.deserialize(&mut deserializer).map_err(|e| {
        let message = e.to_string();
        let reason = if message.contains("duplicate key:") {
            REASON_DUPLICATE_KEY
        } else {
            REASON_PARSE
        };
        LoadError {
            reason,
            message: format!("JSON parse error in {path}: {e}"),
        }
    })?;
    deserializer.end().map_err(|e| LoadError {
        reason: REASON_TRAILING_DATA,
        message: format!("trailing data after JSON value in {path}: {e}"),
    })?;
    Ok(value)
}

/// Load and minimally validate a decision-record JSON file.
fn load_record(path: &str) -> Result<serde_json::Value, LoadError> {
    let contents = fs::read_to_string(path).map_err(|e| LoadError {
        reason: REASON_READ,
        message: format!("could not read {path}: {e}"),
    })?;
    let value = parse_strict(&contents, path)?;
    if !value.is_object() {
        return Err(LoadError {
            reason: REASON_NOT_OBJECT,
            message: format!("{path}: decision record must be a JSON object"),
        });
    }
    let object = value.as_object().expect("checked is_object above");
    if let Some(key) = object
        .keys()
        .find(|key| !KNOWN_FIELDS.contains(&key.as_str()))
    {
        return Err(LoadError {
            reason: REASON_UNKNOWN_FIELD,
            message: format!("{path}: unknown field: {key}"),
        });
    }
    if let Some(claim) = object.get("execution_claim").and_then(|v| v.as_object()) {
        if let Some(key) = claim
            .keys()
            .find(|key| !KNOWN_CLAIM_FIELDS.contains(&key.as_str()))
        {
            return Err(LoadError {
                reason: REASON_UNKNOWN_FIELD,
                message: format!("{path}: unknown field: execution_claim.{key}"),
            });
        }
    }
    Ok(value)
}

/// Map a `verify_record` failure message to its shared rejection class.
fn verify_error_reason(message: &str) -> &'static str {
    if message.starts_with("decision_hash missing or empty") {
        REASON_MISSING_HASH
    } else if message.starts_with("decision_hash mismatch:") {
        REASON_HASH_MISMATCH
    } else {
        "verify-fail"
    }
}

fn main() {
    let args: Vec<String> = env::args().collect();
    if args.len() < 2 {
        eprintln!("usage: boundary-verify <record.json> [more...]");
        process::exit(1);
    }

    let mut all_ok = true;

    for path in &args[1..] {
        let record = match load_record(path) {
            Ok(r) => r,
            Err(e) => {
                eprintln!("error: {}", e.message);
                eprintln!("reason={}", e.reason);
                all_ok = false;
                continue;
            }
        };

        let (ok, message) = verify_record(&record);
        println!("{message}");
        if !ok {
            eprintln!("reason={}", verify_error_reason(&message));
            all_ok = false;
        }
    }

    if !all_ok {
        process::exit(1);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::{Path, PathBuf};

    /// Resolve the repo root relative to this source file's location.
    /// The crate lives at <repo>/verifiers/rust/, so two levels up is the root.
    fn repo_root() -> PathBuf {
        // CARGO_MANIFEST_DIR is set by Cargo during test builds.
        let manifest_dir = std::env::var("CARGO_MANIFEST_DIR")
            .expect("CARGO_MANIFEST_DIR not set; run via cargo test");
        Path::new(&manifest_dir)
            .parent() // verifiers/
            .and_then(|p| p.parent()) // repo root
            .expect("unexpected directory depth")
            .to_path_buf()
    }

    fn load_json(path: &Path) -> serde_json::Value {
        let contents = fs::read_to_string(path)
            .unwrap_or_else(|e| panic!("could not read {}: {e}", path.display()));
        serde_json::from_str(&contents)
            .unwrap_or_else(|e| panic!("JSON parse error in {}: {e}", path.display()))
    }

    fn corpus_dir() -> PathBuf {
        repo_root()
            .join("tests")
            .join("conformance")
            .join("testdata")
            .join("verifier-vectors")
    }

    fn example_record_path() -> PathBuf {
        repo_root()
            .join("docs")
            .join("examples")
            .join("decision-record.example.json")
    }

    // -------------------------------------------------------------------------
    // 1. Example record verifies ok
    // -------------------------------------------------------------------------

    #[test]
    fn test_example_record_verifies_ok() {
        let record = load_json(&example_record_path());
        let (ok, message) = verify_record(&record);
        assert!(
            ok,
            "example record failed verification: {message}"
        );
        assert_eq!(message, "record verification: ok");
    }

    // -------------------------------------------------------------------------
    // 2. Tampered action: deny -> allow must fail
    // -------------------------------------------------------------------------

    #[test]
    fn test_tampered_action_fails() {
        let record = load_json(&example_record_path());
        assert_eq!(
            record.get("action").and_then(|v| v.as_str()),
            Some("deny"),
            "example record expected to be action=deny; update test if changed"
        );

        let mut forged = record.as_object().unwrap().clone();
        forged.insert(
            "action".to_string(),
            serde_json::Value::String("allow".to_string()),
        );
        let forged_val = serde_json::Value::Object(forged);

        let (ok, message) = verify_record(&forged_val);
        assert!(!ok, "forged record (action deny->allow) unexpectedly verified ok");
        assert!(
            message.starts_with("decision_hash mismatch:"),
            "expected mismatch message, got: {message}"
        );
    }

    // -------------------------------------------------------------------------
    // 3. Tampered reason must fail
    // -------------------------------------------------------------------------

    #[test]
    fn test_tampered_reason_fails() {
        let record = load_json(&example_record_path());

        let mut forged = record.as_object().unwrap().clone();
        forged.insert(
            "reason".to_string(),
            serde_json::Value::String("TAMPERED REASON".to_string()),
        );
        let forged_val = serde_json::Value::Object(forged);

        let (ok, message) = verify_record(&forged_val);
        assert!(!ok, "forged record (tampered reason) unexpectedly verified ok");
        assert!(
            message.starts_with("decision_hash mismatch:"),
            "expected mismatch message, got: {message}"
        );
    }

    // -------------------------------------------------------------------------
    // 4. Hash mismatch: stored hash replaced with wrong value
    // -------------------------------------------------------------------------

    #[test]
    fn test_hash_mismatch_fails() {
        let record = load_json(&example_record_path());

        let mut tampered = record.as_object().unwrap().clone();
        tampered.insert(
            "decision_hash".to_string(),
            serde_json::Value::String(
                "sha256:0000000000000000000000000000000000000000000000000000000000000000"
                    .to_string(),
            ),
        );
        let tampered_val = serde_json::Value::Object(tampered);

        let (ok, message) = verify_record(&tampered_val);
        assert!(!ok, "wrong stored hash unexpectedly verified ok");
        assert!(
            message.starts_with("decision_hash mismatch:"),
            "expected mismatch message, got: {message}"
        );
    }

    // -------------------------------------------------------------------------
    // 5. Missing decision_hash
    // -------------------------------------------------------------------------

    #[test]
    fn test_missing_decision_hash_fails() {
        let record = load_json(&example_record_path());

        let mut stripped = record.as_object().unwrap().clone();
        stripped.remove("decision_hash");
        let stripped_val = serde_json::Value::Object(stripped);

        let (ok, message) = verify_record(&stripped_val);
        assert!(!ok, "record missing decision_hash should not verify ok");
        assert!(
            message.contains("decision_hash missing or empty"),
            "expected missing message, got: {message}"
        );
    }

    // -------------------------------------------------------------------------
    // 6. Signature fields are excluded from the hash
    //    Adding signature / signature_key_id must NOT change the hash.
    // -------------------------------------------------------------------------

    #[test]
    fn test_signature_fields_excluded_from_hash() {
        let record = load_json(&example_record_path());
        let original_hash = compute_decision_hash(&record).expect("hash should compute");

        let mut with_sig = record.as_object().unwrap().clone();
        with_sig.insert(
            "signature".to_string(),
            serde_json::Value::String("some-sig-value".to_string()),
        );
        with_sig.insert(
            "signature_key_id".to_string(),
            serde_json::Value::String("key-id-1".to_string()),
        );
        let with_sig_val = serde_json::Value::Object(with_sig);

        let new_hash = compute_decision_hash(&with_sig_val).expect("hash should compute");
        assert_eq!(
            original_hash, new_hash,
            "signature fields should not affect decision_hash"
        );
    }

    // -------------------------------------------------------------------------
    // 7–9. Conformance corpus: all 9 manifest vectors recompute to committed hashes
    //      This is the same corpus the Go conformance gate asserts.
    // -------------------------------------------------------------------------

    #[test]
    fn test_conformance_corpus_all_vectors() {
        let manifest_path = corpus_dir().join("manifest.json");
        assert!(
            manifest_path.exists(),
            "conformance manifest missing at {}; regenerate with BOUNDARY_WRITE_VECTORS=1 go test ./tests/conformance/",
            manifest_path.display()
        );

        let manifest = load_json(&manifest_path);
        let vectors = manifest
            .get("vectors")
            .and_then(|v| v.as_array())
            .expect("manifest must have a 'vectors' array");

        assert!(!vectors.is_empty(), "manifest lists no vectors");

        let mut checked = 0usize;
        for entry in vectors {
            let file_name = entry
                .get("file")
                .and_then(|v| v.as_str())
                .expect("vector entry missing 'file'");
            let expected_hash = entry
                .get("decision_hash")
                .and_then(|v| v.as_str())
                .expect("vector entry missing 'decision_hash'");
            let expect = entry
                .get("expect")
                .and_then(|v| v.as_str())
                .unwrap_or("verify");
            let why = entry
                .get("why")
                .and_then(|v| v.as_str())
                .unwrap_or("(no why)");

            let record_path = corpus_dir().join(file_name);
            assert!(
                record_path.exists(),
                "corpus file missing: {}",
                record_path.display()
            );

            if let Some(want_reason) = expect.strip_prefix("reject:") {
                // Reject vectors: strict load must fail with the manifest's
                // reason class, or (for hash-mismatch) verification must fail
                // with it.
                match load_record(record_path.to_str().unwrap()) {
                    Err(e) => assert_eq!(
                        e.reason, want_reason,
                        "{file_name} ({why}): rejected with reason={}, want {want_reason}",
                        e.reason
                    ),
                    Ok(record) => {
                        let (ok, message) = verify_record(&record);
                        assert!(
                            !ok,
                            "{file_name} ({why}): unexpectedly verified ok, want reject:{want_reason}"
                        );
                        let reason = verify_error_reason(&message);
                        assert_eq!(
                            reason, want_reason,
                            "{file_name} ({why}): verify failed with reason={reason}, want {want_reason}"
                        );
                    }
                }
                checked += 1;
                continue;
            }

            let record = load_record(record_path.to_str().unwrap())
                .unwrap_or_else(|e| panic!("{file_name}: strict load failed: {}", e.message));

            // The file's own stored decision_hash must match the manifest.
            let stored = record
                .get("decision_hash")
                .and_then(|v| v.as_str())
                .unwrap_or("");
            assert_eq!(
                stored, expected_hash,
                "{file_name}: stored decision_hash {stored} != manifest {expected_hash}"
            );

            // The Rust re-implementation must reproduce that exact hash.
            let recomputed = compute_decision_hash(&record)
                .unwrap_or_else(|e| panic!("{file_name}: compute_decision_hash failed: {e}"));
            assert_eq!(
                recomputed, expected_hash,
                "{file_name} ({why}):\n  recomputed: {recomputed}\n  committed:  {expected_hash}"
            );

            let (ok, message) = verify_record(&record);
            assert!(ok, "{file_name}: verify_record failed: {message}");

            checked += 1;
        }

        assert_eq!(
            checked,
            vectors.len(),
            "checked {checked} but manifest has {} vectors",
            vectors.len()
        );
    }

    // -------------------------------------------------------------------------
    // Float regression: v1_float_trust_score.json
    //   trust_score 1.0/3.0 must canonicalize as 0.3333333333333333
    // -------------------------------------------------------------------------

    #[test]
    fn test_v1_float_trust_score_vector() {
        let path = corpus_dir().join("v1_float_trust_score.json");
        let record = load_json(&path);

        let expected = "sha256:749a05ec9252e584e01e78f7ef2219511725b5d5664f9d01300e0edee860722f";
        let recomputed = compute_decision_hash(&record)
            .expect("compute_decision_hash should not fail on float vector");
        assert_eq!(
            recomputed, expected,
            "float trust_score ECMAScript round-trip regression failed:\n  got:  {recomputed}\n  want: {expected}"
        );

        let (ok, message) = verify_record(&record);
        assert!(ok, "v1_float_trust_score.json verify_record failed: {message}");
    }
}
