package verifier

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

const resultsSchemaVersion = "witnessed-verifier-results-v1"

type bundleArtifact struct {
	present bool
	data    []byte
	lines   []exactJSONLine
	err     error
}

type inspectedDecision struct {
	record decisionRecordV1
	valid  bool
}

// VerifyDir independently verifies a witnessed bundle using only its exported
// bytes and the caller-supplied public keys.
func VerifyDir(dir string, keys *KeySet) (Results, error) {
	bundleDir, err := validateBundleDir(dir)
	if err != nil {
		return Results{}, err
	}
	manifestData, err := readRegularFile(filepath.Join(bundleDir, manifestFilename))
	if err != nil {
		return Results{}, fmt.Errorf("read manifest: %w", err)
	}
	bundleManifest, err := decodeManifest(manifestData)
	if err != nil {
		return Results{}, fmt.Errorf("decode manifest: %w", err)
	}

	artifacts := make(map[string]bundleArtifact, len(registeredBundleFiles))
	for _, name := range registeredBundleFiles {
		artifacts[name] = loadArtifact(filepath.Join(bundleDir, name))
	}

	decisionChecks, decisions, decisionManifestOK := inspectDecisions(
		artifacts[decisionsFile], bundleManifest,
	)
	proof, proofOK := inspectProof(artifacts[proofFile])
	head, headOK := inspectTreeHead(artifacts[treeHeadFile])
	aggregate, aggregateOK := inspectAggregate(artifacts[cosignaturesFile])
	decisionChecks = bindSelectedDecision(decisionChecks, decisions, proof, proofOK)

	checks := append([]Check(nil), decisionChecks...)
	for i, registration := range bundleManifest.Files {
		ok := registrationMatches(registration, artifacts[registration.Name])
		switch registration.Name {
		case decisionsFile:
			ok = ok && decisionManifestOK
		case declinesFile:
			ok = ok && len(artifacts[declinesFile].data) == 0 && bundleManifest.DeclinesCount == 0
		case cosignaturesFile:
			ok = ok && aggregateOK &&
				bundleManifest.WitnessedLog.ConfiguredWitnesses == len(aggregate.ConfiguredWitnesses) &&
				bundleManifest.WitnessedLog.PresentCosignatures == aggregate.PresentCosignatures
		}
		checks = append(checks, Check{ID: "manifest:" + registeredBundleFiles[i], Status: passFail(ok)})
	}

	checks = append(checks, sourceCheck(artifacts[proofFile], proof, proofOK))
	checks = append(checks, inclusionCheck(artifacts[proofFile], proof, proofOK, artifacts[treeHeadFile], head, headOK))
	checks = append(checks, bundleTenantBindingCheck(bundleManifest, head, headOK))
	checks = append(checks, treeHeadCheck(artifacts[treeHeadFile], head, headOK, keys))
	checks = append(checks, witnessChecks(aggregate, aggregateOK, head, headOK, keys)...)

	return Results{SchemaVersion: resultsSchemaVersion, Checks: checks}, nil
}

func validateBundleDir(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("bundle directory is required")
	}
	cleaned := filepath.Clean(dir)
	info, err := os.Lstat(cleaned)
	if err != nil {
		return "", fmt.Errorf("inspect bundle directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("bundle path is not a directory")
	}
	return cleaned, nil
}

func loadArtifact(path string) bundleArtifact {
	data, err := readRegularFile(path)
	if err != nil {
		return bundleArtifact{present: !errors.Is(err, os.ErrNotExist), err: err}
	}
	lines, lineErr := splitExactJSONLines(data)
	return bundleArtifact{present: true, data: data, lines: lines, err: lineErr}
}

func registrationMatches(registration manifestFile, artifact bundleArtifact) bool {
	if !artifact.present || artifact.err != nil || len(artifact.lines) != registration.Lines ||
		len(registration.LineSHA256) != len(artifact.lines) {
		return false
	}
	for i, line := range artifact.lines {
		registered := registration.LineSHA256[i]
		if registered.Line != i+1 || exactLineSHA256(line.complete) != registered.SHA256 {
			return false
		}
	}
	return true
}

func inspectDecisions(artifact bundleArtifact, bundleManifest manifest) ([]Check, []inspectedDecision, bool) {
	lineCount := len(artifact.lines)
	checkCount := lineCount
	if bundleManifest.Decisions.Total > checkCount {
		checkCount = bundleManifest.Decisions.Total
	}
	checks := make([]Check, 0, checkCount)
	decisions := make([]inspectedDecision, lineCount)
	claimedHashes := make(map[string]int, lineCount)

	manifestOK := artifact.present && artifact.err == nil && lineCount == bundleManifest.Decisions.Total
	verdictCounts := make(map[string]int)
	modeCounts := make(map[string]int)
	verificationCounts := make(map[string]int)
	signedCount := 0
	signingKeySet := make(map[string]struct{})

	for i := 0; i < checkCount; i++ {
		idHash := ""
		if i < len(bundleManifest.Index) {
			idHash = bundleManifest.Index[i].DecisionHash
			verificationCounts[bundleManifest.Index[i].VerificationStatus]++
		}
		valid := false
		if i < lineCount {
			record, err := decodeDecisionRecord(artifact.lines[i].body)
			if err == nil {
				computed, hashErr := computeDecisionHash(record)
				valid = hashErr == nil && computed == record.DecisionHash
				decisions[i] = inspectedDecision{record: record, valid: valid}
				if idHash == "" {
					idHash = record.DecisionHash
				}
				claimedHashes[record.DecisionHash]++
				verdictCounts[record.Action]++
				modeCounts[record.DecisionMode]++
				if record.Signature != "" {
					signedCount++
					signingKeySet[record.SignatureKeyID] = struct{}{}
				}
				if i >= len(bundleManifest.Index) || !indexMatchesRecord(bundleManifest.Index[i], record) {
					manifestOK = false
				}
			} else {
				manifestOK = false
			}
		}
		if idHash == "" {
			idHash = fmt.Sprintf("line-%d", i+1)
		}
		checks = append(checks, Check{ID: "decision_record_integrity:" + idHash, Status: passFail(valid)})
	}
	for hash, count := range claimedHashes {
		if hash == "" || count != 1 {
			manifestOK = false
		}
	}
	if !reflect.DeepEqual(verdictCounts, bundleManifest.Decisions.ByVerdict) ||
		!reflect.DeepEqual(modeCounts, bundleManifest.Decisions.ByDecisionMode) ||
		!reflect.DeepEqual(verificationCounts, bundleManifest.Decisions.ByVerificationStatus) ||
		signedCount != bundleManifest.Signing.SignedCount ||
		bundleManifest.Signing.AnySigned != (signedCount > 0) ||
		!reflect.DeepEqual(sortedSet(signingKeySet), nonnilStrings(bundleManifest.Signing.KeyIDs)) {
		manifestOK = false
	}
	return checks, decisions, manifestOK
}

func indexMatchesRecord(index manifestIndex, record decisionRecordV1) bool {
	return index.DecisionHash == record.DecisionHash &&
		index.Action == record.Action &&
		index.DecisionMode == record.DecisionMode &&
		index.Signed == (record.Signature != "") &&
		index.SignatureKeyID == record.SignatureKeyID
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func nonnilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func inspectProof(artifact bundleArtifact) (inclusionProof, bool) {
	if !artifact.present || artifact.err != nil {
		return inclusionProof{}, false
	}
	proof, err := decodeInclusionProof(artifact.data)
	return proof, err == nil
}

func inspectTreeHead(artifact bundleArtifact) (treeHead, bool) {
	if !artifact.present || artifact.err != nil {
		return treeHead{}, false
	}
	head, err := decodeTreeHead(artifact.data)
	return head, err == nil
}

func inspectAggregate(artifact bundleArtifact) (cosignatureAggregate, bool) {
	if !artifact.present || artifact.err != nil {
		return cosignatureAggregate{}, false
	}
	aggregate, err := decodeCosignatureAggregate(artifact.data)
	return aggregate, err == nil
}

func bindSelectedDecision(checks []Check, decisions []inspectedDecision, proof inclusionProof, proofOK bool) []Check {
	if !proofOK {
		return checks
	}
	selectedID := "decision_record_integrity:" + proof.SourceHash
	represented := false
	selectedValid := false
	for _, decision := range decisions {
		if decision.record.DecisionHash == proof.SourceHash {
			selectedValid = decision.valid && decision.record.DecisionMode == proof.DecisionMode
			break
		}
	}
	for i := range checks {
		if checks[i].ID != selectedID {
			continue
		}
		represented = true
		if !selectedValid {
			checks[i].Status = StatusFail
		}
	}
	if !represented {
		checks = append(checks, Check{ID: selectedID, Status: StatusFail})
	}
	return checks
}

// bundleTenantBindingCheck prevents an unsigned manifest label from being
// detached from the signed tree head that actually commits the receipt.
func bundleTenantBindingCheck(bundleManifest manifest, head treeHead, headOK bool) Check {
	return Check{
		ID:     "bundle_tenant_binding",
		Status: passFail(headOK && bundleManifest.TenantID == head.TenantID),
	}
}

func sourceCheck(artifact bundleArtifact, proof inclusionProof, proofOK bool) Check {
	check := Check{ID: "source_hash_to_leaf"}
	if !artifact.present {
		check.Status = StatusNotPresent
		return check
	}
	if !proofOK || !verifySourceToLeaf(proof) {
		check.Status = StatusFail
		return check
	}
	check.Status = StatusPass
	return check
}

func inclusionCheck(proofArtifact bundleArtifact, proof inclusionProof, proofOK bool, headArtifact bundleArtifact, head treeHead, headOK bool) Check {
	check := Check{ID: "inclusion_proof"}
	if !proofArtifact.present || !headArtifact.present {
		check.Status = StatusNotPresent
		return check
	}
	check.Status = passFail(proofOK && headOK && verifyInclusion(proof, head))
	return check
}

func treeHeadCheck(artifact bundleArtifact, head treeHead, headOK bool, keys *KeySet) Check {
	check := Check{ID: "sth_signature"}
	if !artifact.present {
		check.Status = StatusNotPresent
		return check
	}
	check.Status = passFail(headOK && verifyTreeHeadSignature(head, keys))
	return check
}

func witnessChecks(aggregate cosignatureAggregate, aggregateOK bool, head treeHead, headOK bool, keys *KeySet) []Check {
	if !aggregateOK {
		return nil
	}
	present := make(map[string]witnessCosignature, len(aggregate.Cosignatures))
	for _, item := range aggregate.Cosignatures {
		present[item.WitnessID] = item
	}
	checks := make([]Check, 0, len(aggregate.ConfiguredWitnesses))
	for _, configured := range aggregate.ConfiguredWitnesses {
		check := Check{ID: "witness_cosignature:" + configured.WitnessID}
		item, ok := present[configured.WitnessID]
		if !ok {
			check.Status = StatusMissing
			checks = append(checks, check)
			continue
		}
		key, keyOK := keys.lookup(configured.WitnessKeyID, RoleWitness)
		matchesHead := headOK && aggregate.TreeHeadID == head.TreeHeadID &&
			item.LogID == head.LogID && item.TreeSize == head.TreeSize &&
			item.RootHash == head.RootHash && item.FulcrumKeyID == head.SignatureKeyID
		check.Status = passFail(keyOK && matchesHead && verifyWitnessSignature(item, key))
		checks = append(checks, check)
	}
	return checks
}

func passFail(ok bool) Status {
	if ok {
		return StatusPass
	}
	return StatusFail
}
