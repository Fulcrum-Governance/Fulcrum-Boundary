//go:build !cgo

package sql

import "strings"

// astUnavailableReason explains why every statement classifies as UNKNOWN in
// binaries built without cgo. The Postgres guard surfaces it in deny reasons.
const astUnavailableReason = "sql ast classification unavailable in this build (CGO disabled)"

// ClassifyPostgres is the fail-safe stub used when the binary is built with
// CGO_ENABLED=0 and the PostgreSQL AST parser (pg_query_go, a cgo binding) is
// unavailable. Every statement — including SQL the full classifier would label
// READ — classifies as UNKNOWN, the bucket the Postgres guard denies
// fail-closed. The stub never assigns a more permissive class than the cgo
// classifier would, so a no-cgo build can deny SQL the cgo build allows, but
// never allow SQL the cgo build denies.
func ClassifyPostgres(sqlText string) Classification {
	if strings.TrimSpace(sqlText) == "" {
		return Classification{Class: ClassUnknown, ParseError: "empty SQL"}
	}
	return Classification{Class: ClassUnknown, ParseError: astUnavailableReason}
}
