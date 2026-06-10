package sql

// Class is the severity bucket assigned to a SQL text by ClassifyPostgres.
// UNKNOWN is the fail-safe bucket: the Postgres guard denies it fail-closed,
// so anything the classifier cannot positively identify routes to deny, never
// to allow.
type Class string

const (
	ClassRead        Class = "READ"
	ClassWrite       Class = "WRITE"
	ClassAdmin       Class = "ADMIN"
	ClassDestructive Class = "DESTRUCTIVE"
	ClassUnknown     Class = "UNKNOWN"
)

// Classification reports the class assigned to a SQL text, the statement
// types observed (when AST parsing is available), and the parse error or
// unavailability reason when classification could not positively identify the
// statements.
type Classification struct {
	Class          Class
	StatementTypes []string
	ParseError     string
}

// Unknown reports whether the classification landed in the fail-safe bucket.
func (c Classification) Unknown() bool {
	return c.Class == "" || c.Class == ClassUnknown
}
