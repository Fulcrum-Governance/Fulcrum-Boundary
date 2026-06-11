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

use sha2::{Digest, Sha256};
use std::{env, fs, process};

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

/// Load and minimally validate a decision-record JSON file.
fn load_record(path: &str) -> Result<serde_json::Value, String> {
    let contents =
        fs::read_to_string(path).map_err(|e| format!("could not read {path}: {e}"))?;
    let value: serde_json::Value =
        serde_json::from_str(&contents).map_err(|e| format!("JSON parse error in {path}: {e}"))?;
    if !value.is_object() {
        return Err(format!("{path}: decision record must be a JSON object"));
    }
    Ok(value)
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
                eprintln!("error: {e}");
                all_ok = false;
                continue;
            }
        };

        let (ok, message) = verify_record(&record);
        println!("{message}");
        if !ok {
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

            let record = load_json(&record_path);

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
