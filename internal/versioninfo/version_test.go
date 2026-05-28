package versioninfo

import "testing"

func TestCurrentReturnsStableMetadata(t *testing.T) {
	info := Current()
	if info.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q", info.SchemaVersion)
	}
	if info.Version == "" {
		t.Fatal("version must not be empty")
	}
	if info.Commit == "" {
		t.Fatal("commit must not be empty")
	}
	if info.BuildDate == "" {
		t.Fatal("build_date must not be empty")
	}
	if info.GoVersion == "" {
		t.Fatal("go_version must not be empty")
	}
	if info.Module == "" {
		t.Fatal("module must not be empty")
	}
}

func TestCurrentUsesExplicitMetadata(t *testing.T) {
	info := Current(Metadata{
		Version:   "v9.9.9",
		Commit:    "abc123",
		BuildDate: "2026-05-28T00:00:00Z",
	})
	if info.Version != "v9.9.9" {
		t.Fatalf("version = %q", info.Version)
	}
	if info.Commit != "abc123" {
		t.Fatalf("commit = %q", info.Commit)
	}
	if info.BuildDate != "2026-05-28T00:00:00Z" {
		t.Fatalf("build_date = %q", info.BuildDate)
	}
}

func TestUnknownAndDevelValuesAreIgnored(t *testing.T) {
	got := coalesceKnown("fallback", "", "unknown", "(devel)", " value ")
	if got != "value" {
		t.Fatalf("coalesceKnown = %q", got)
	}
}
