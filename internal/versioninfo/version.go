package versioninfo

import (
	"runtime"
	"runtime/debug"
	"strings"
)

const (
	SchemaVersion = "boundary.version.v1"
	ModulePath    = "github.com/fulcrum-governance/fulcrum-boundary"
	Unknown       = "unknown"
)

var (
	Version   = Unknown
	Commit    = Unknown
	BuildDate = Unknown
)

type Metadata struct {
	Version   string
	Commit    string
	BuildDate string
}

type Info struct {
	SchemaVersion string `json:"schema_version"`
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	BuildDate     string `json:"build_date"`
	GoVersion     string `json:"go_version"`
	Module        string `json:"module"`
}

func Current(overrides ...Metadata) Info {
	buildInfo, ok := debug.ReadBuildInfo()
	info := Info{
		SchemaVersion: SchemaVersion,
		Version:       coalesceKnown(Unknown, versionCandidates(buildInfo, ok, overrides...)...),
		Commit:        coalesceKnown(Unknown, commitCandidates(buildInfo, ok, overrides...)...),
		BuildDate:     coalesceKnown(Unknown, buildDateCandidates(buildInfo, ok, overrides...)...),
		GoVersion:     runtime.Version(),
		Module:        ModulePath,
	}
	return info
}

func versionCandidates(buildInfo *debug.BuildInfo, ok bool, overrides ...Metadata) []string {
	values := []string{}
	for _, override := range overrides {
		values = append(values, override.Version)
	}
	values = append(values, Version)
	if ok {
		values = append(values, buildInfo.Main.Version)
	}
	return values
}

func commitCandidates(buildInfo *debug.BuildInfo, ok bool, overrides ...Metadata) []string {
	values := []string{}
	for _, override := range overrides {
		values = append(values, override.Commit)
	}
	values = append(values, Commit)
	if ok {
		values = append(values, buildSetting(buildInfo, "vcs.revision"))
	}
	return values
}

func buildDateCandidates(buildInfo *debug.BuildInfo, ok bool, overrides ...Metadata) []string {
	values := []string{}
	for _, override := range overrides {
		values = append(values, override.BuildDate)
	}
	values = append(values, BuildDate)
	if ok {
		values = append(values, buildSetting(buildInfo, "vcs.time"))
	}
	return values
}

func buildSetting(info *debug.BuildInfo, key string) string {
	for _, setting := range info.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}

func coalesceKnown(fallback string, values ...string) string {
	for _, value := range values {
		normalized := normalize(value)
		if normalized != Unknown {
			return normalized
		}
	}
	return fallback
}

func normalize(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == Unknown || trimmed == "(devel)" {
		return Unknown
	}
	return trimmed
}
