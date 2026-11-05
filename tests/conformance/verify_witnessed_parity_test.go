package conformance

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// verify-witnessed is a separate Go module that must stay dependency-
// independent of this one (it cannot import governance), so it carries its own
// mirror of the decision-record type and a pinned copy of this corpus. The
// tests below are the bridge: they fail in the root module's CI when the mirror
// or the pinned corpus falls behind, which is how a field added here without a
// verifier update turns CI red instead of shipping a verifier that rejects the
// records this module produces.

const (
	verifyWitnessedDir        = "../../verify-witnessed"
	verifyWitnessedVectorsDir = verifyWitnessedDir + "/testdata/verifier-vectors"
	verifyWitnessedWireFile   = verifyWitnessedDir + "/verifier/wire.go"
)

// TestVerifyWitnessedCorpusMatchesConformanceCorpus asserts the verifier
// vectors pinned inside verify-witnessed are a byte-for-byte copy of the
// corpus in testdata/verifier-vectors: same file set, same bytes.
func TestVerifyWitnessedCorpusMatchesConformanceCorpus(t *testing.T) {
	source := readCorpusDir(t, vectorsDir)
	pinned := readCorpusDir(t, verifyWitnessedVectorsDir)
	if len(source) == 0 {
		t.Fatalf("no corpus files found under %s", vectorsDir)
	}

	const fix = "copy the corpus: rm -rf verify-witnessed/testdata/verifier-vectors && cp -R tests/conformance/testdata/verifier-vectors verify-witnessed/testdata/verifier-vectors"
	for name, want := range source {
		got, ok := pinned[name]
		if !ok {
			t.Errorf("verify-witnessed pinned corpus is missing %s; %s", name, fix)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("verify-witnessed pinned corpus differs from the conformance corpus at %s; %s", name, fix)
		}
	}
	for name := range pinned {
		if _, ok := source[name]; !ok {
			t.Errorf("verify-witnessed pinned corpus has extra file %s not in the conformance corpus; %s", name, fix)
		}
	}
}

func readCorpusDir(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus dir %s: %v", dir, err)
	}
	out := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			t.Errorf("unexpected directory %s in corpus dir %s", entry.Name(), dir)
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name())) // #nosec G304 -- fixed test-data path.
		if err != nil {
			t.Fatalf("read corpus file %s: %v", entry.Name(), err)
		}
		out[entry.Name()] = raw
	}
	return out
}

// wireField is one JSON member of a record type: its wire name, whether it is
// omitempty, and a type shape comparable across the two modules.
type wireField struct {
	omitEmpty bool
	shape     string
}

type wireStruct map[string]wireField

// TestVerifyWitnessedRecordTypeMatchesDecisionRecord asserts verify-witnessed's
// record type (read from its source, not imported) has exactly the JSON member
// set, omitempty semantics, and nested structure of governance.DecisionRecordV1,
// including the nested check and execution_claim objects. A governance field
// added, renamed, retagged, or re-typed without the matching verifier change
// fails here.
func TestVerifyWitnessedRecordTypeMatchesDecisionRecord(t *testing.T) {
	structs := parseVerifierStructs(t)

	want := governanceWireStruct(reflect.TypeOf(governance.DecisionRecordV1{}))
	got, err := verifierWireStruct(structs, "decisionRecordV1")
	if err != nil {
		t.Fatal(err)
	}
	compareWireStructs(t, "decisionRecordV1", want, got)

	// The member sets must also be the ones this module's strict decoder
	// enforces: a closed set per nested object.
	if _, ok := want["check"]; !ok {
		t.Error("governance.DecisionRecordV1 no longer has a check member; update this parity test")
	}
	if _, ok := want["execution_claim"]; !ok {
		t.Error("governance.DecisionRecordV1 no longer has an execution_claim member; update this parity test")
	}
}

// parseVerifierStructs returns every struct type declared in verify-witnessed's
// wire.go, read from source so this module never imports the verifier.
func parseVerifierStructs(t *testing.T) map[string]*ast.StructType {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, verifyWitnessedWireFile, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", verifyWitnessedWireFile, err)
	}
	structs := map[string]*ast.StructType{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec := spec.(*ast.TypeSpec)
			if st, ok := typeSpec.Type.(*ast.StructType); ok {
				structs[typeSpec.Name.Name] = st
			}
		}
	}
	return structs
}

var timeType = reflect.TypeOf(time.Time{})

// governanceWireStruct flattens a governance struct into its wire shape.
// Nested pointer-to-struct members become "ptr(<name>)" with their own entry
// under "<path>.<member>" so the comparison covers every closed object.
func governanceWireStruct(t reflect.Type) wireStruct {
	out := wireStruct{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		out[name] = wireField{
			omitEmpty: strings.Contains(","+opts+",", ",omitempty,"),
			shape:     governanceShape(field.Type),
		}
	}
	return out
}

func governanceShape(t reflect.Type) string {
	if t == timeType {
		// governance marshals time.Time as an RFC 3339 string; the verifier
		// holds the wire string so it can require canonical UTC.
		return "string"
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Float64:
		return "float64"
	case reflect.Bool:
		return "bool"
	case reflect.Pointer:
		if t.Elem().Kind() == reflect.Struct {
			return "ptr(" + nestedKey(t.Elem().Name()) + ")"
		}
	}
	return "unsupported(" + t.String() + ")"
}

// nestedKey maps a governance nested type name to the verifier's name for the
// same wire object.
func nestedKey(governanceName string) string {
	switch governanceName {
	case "CheckFailure":
		return "check"
	case "ExecutionClaim":
		return "execution_claim"
	}
	return governanceName
}

func verifierWireStruct(structs map[string]*ast.StructType, name string) (wireStruct, error) {
	st, ok := structs[name]
	if !ok {
		return nil, fmt.Errorf("verify-witnessed wire.go has no struct %s", name)
	}
	out := wireStruct{}
	for _, field := range st.Fields.List {
		if field.Tag == nil {
			return nil, fmt.Errorf("%s has an untagged field; every member must carry a json tag", name)
		}
		unquoted, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			return nil, err
		}
		tag := reflect.StructTag(unquoted).Get("json")
		member, opts, _ := strings.Cut(tag, ",")
		if member == "" || member == "-" {
			return nil, fmt.Errorf("%s has a field without a json member name", name)
		}
		shape, err := verifierShape(field.Type)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", name, member, err)
		}
		out[member] = wireField{
			omitEmpty: strings.Contains(","+opts+",", ",omitempty,"),
			shape:     shape,
		}
	}
	return out, nil
}

func verifierShape(expr ast.Expr) (string, error) {
	switch typed := expr.(type) {
	case *ast.Ident:
		switch typed.Name {
		case "string", "float64", "bool":
			return typed.Name, nil
		}
	case *ast.StarExpr:
		if ident, ok := typed.X.(*ast.Ident); ok {
			switch ident.Name {
			case "checkFailure":
				return "ptr(check)", nil
			case "executionClaim":
				return "ptr(execution_claim)", nil
			}
		}
	}
	return "", fmt.Errorf("unsupported verifier field type %T", expr)
}

func compareWireStructs(t *testing.T, label string, want, got wireStruct) {
	t.Helper()
	var names []string
	for name := range want {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		g, ok := got[name]
		if !ok {
			t.Errorf("%s: governance defines member %q that verify-witnessed does not; its strict decoder would reject records carrying it", label, name)
			continue
		}
		w := want[name]
		if g.omitEmpty != w.omitEmpty {
			t.Errorf("%s.%s: omitempty = %v in verify-witnessed, %v in governance; the recomputed decision_hash would diverge", label, name, g.omitEmpty, w.omitEmpty)
		}
		if g.shape != w.shape {
			t.Errorf("%s.%s: type shape %s in verify-witnessed, %s in governance", label, name, g.shape, w.shape)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s: verify-witnessed defines member %q that governance does not", label, name)
		}
	}
}

// TestVerifyWitnessedNestedRecordTypesMatch applies the same comparison to the
// nested objects (check and execution_claim) whose member sets are closed in
// both decoders.
func TestVerifyWitnessedNestedRecordTypesMatch(t *testing.T) {
	structs := parseVerifierStructs(t)
	for _, nested := range []struct {
		label    string
		verifier string
		gov      reflect.Type
	}{
		{"check", "checkFailure", reflect.TypeOf(governance.CheckFailure{})},
		{"execution_claim", "executionClaim", reflect.TypeOf(governance.ExecutionClaim{})},
	} {
		got, err := verifierWireStruct(structs, nested.verifier)
		if err != nil {
			t.Fatal(err)
		}
		compareWireStructs(t, nested.label, governanceWireStruct(nested.gov), got)
	}
}

// caseLiterals returns the string literals in every case clause of the first
// switch statement in the named function of verify-witnessed's wire.go.
func caseLiterals(t *testing.T, funcName string) map[string]bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), verifyWitnessedWireFile, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", verifyWitnessedWireFile, err)
	}
	out := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName || fn.Recv != nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			clause, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, expr := range clause.List {
				lit, ok := expr.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("%s has a non-literal case expression; the parity test cannot read it", funcName)
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				out[value] = true
			}
			return true
		})
		if len(out) == 0 {
			t.Fatalf("%s has no string case literals", funcName)
		}
		return out
	}
	t.Fatalf("verify-witnessed wire.go has no function %s", funcName)
	return nil
}

// TestVerifyWitnessedDecisionModesMatchGovernance pins the verifier's accepted
// decision_mode values to the governance.DecisionMode constants: every
// constant declared in governance/decision_mode.go must be accepted by the
// verifier, the verifier must accept nothing else, and each accepted value
// must satisfy governance.DecisionMode.Valid.
func TestVerifyWitnessedDecisionModesMatchGovernance(t *testing.T) {
	got := caseLiterals(t, "validDecisionMode")

	file, err := parser.ParseFile(token.NewFileSet(), "../../governance/decision_mode.go", nil, 0)
	if err != nil {
		t.Fatalf("parse governance/decision_mode.go: %v", err)
	}
	want := map[string]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec := spec.(*ast.ValueSpec)
			ident, ok := valueSpec.Type.(*ast.Ident)
			if !ok || ident.Name != "DecisionMode" {
				continue
			}
			for _, expr := range valueSpec.Values {
				value, err := strconv.Unquote(expr.(*ast.BasicLit).Value)
				if err != nil {
					t.Fatal(err)
				}
				want[value] = true
			}
		}
	}
	if len(want) == 0 {
		t.Fatal("found no DecisionMode constants in governance/decision_mode.go")
	}
	for mode := range want {
		if !governance.DecisionMode(mode).Valid() {
			t.Errorf("governance constant %q is not Valid(); update this parity test", mode)
		}
		if !got[mode] {
			t.Errorf("governance defines decision_mode %q that verify-witnessed does not accept", mode)
		}
	}
	for mode := range got {
		if !want[mode] || !governance.DecisionMode(mode).Valid() {
			t.Errorf("verify-witnessed accepts decision_mode %q that governance does not define as valid", mode)
		}
	}
}

// TestVerifyWitnessedSchemaVersionsMatchGovernance pins the verifier's
// accepted schema_version values to
// governance.SupportedDecisionRecordSchemaVersion.
func TestVerifyWitnessedSchemaVersionsMatchGovernance(t *testing.T) {
	got := caseLiterals(t, "supportedRecordSchemaVersion")
	for _, version := range []string{governance.DecisionRecordSchemaVersion, governance.DecisionRecordSchemaV2} {
		if !got[version] {
			t.Errorf("governance supports schema_version %q that verify-witnessed does not accept", version)
		}
	}
	for version := range got {
		if !governance.SupportedDecisionRecordSchemaVersion(version) {
			t.Errorf("verify-witnessed accepts schema_version %q that governance does not support", version)
		}
	}
	for _, unsupported := range []string{"", "0", "3", "v1"} {
		if governance.SupportedDecisionRecordSchemaVersion(unsupported) && !got[unsupported] {
			t.Errorf("governance supports schema_version %q that verify-witnessed does not accept", unsupported)
		}
	}
}
