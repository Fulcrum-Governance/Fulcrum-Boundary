//go:build cgo

package sql

// This file holds the full Postgres AST classification backed by pg_query_go,
// the cgo binding for the PostgreSQL parser. It only builds when cgo is
// enabled; binaries built with CGO_ENABLED=0 use the fail-safe stub in
// ast_classifier_nocgo.go, which routes every statement to the UNKNOWN (deny)
// bucket.

import (
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// ClassifyPostgres parses sqlText with the PostgreSQL AST parser and returns
// the highest-severity class across all parsed statements. Empty, invalid, or
// unparsable SQL classifies as UNKNOWN, which the Postgres guard denies
// fail-closed.
func ClassifyPostgres(sqlText string) Classification {
	if strings.TrimSpace(sqlText) == "" {
		return Classification{Class: ClassUnknown, ParseError: "empty SQL"}
	}
	tree, err := pg_query.Parse(sqlText)
	if err != nil {
		return Classification{Class: ClassUnknown, ParseError: err.Error()}
	}
	if tree == nil || len(tree.GetStmts()) == 0 {
		return Classification{Class: ClassUnknown, ParseError: "no statements"}
	}

	out := Classification{Class: ClassRead}
	for _, stmt := range tree.GetStmts() {
		stmtClass, stmtType := classifyStmt(stmt.GetStmt())
		out.StatementTypes = append(out.StatementTypes, stmtType)
		out.Class = higherClass(out.Class, stmtClass)
	}
	return out
}

func classifyStmt(node *pg_query.Node) (class Class, stmtType string) {
	if node == nil || node.GetNode() == nil {
		return ClassUnknown, "UNKNOWN"
	}
	switch node.GetNode().(type) {
	case *pg_query.Node_SelectStmt, *pg_query.Node_VariableShowStmt:
		return ClassRead, strings.TrimPrefix(fmt.Sprintf("%T", node.GetNode()), "*pg_query.Node_")
	case *pg_query.Node_InsertStmt, *pg_query.Node_UpdateStmt, *pg_query.Node_DeleteStmt, *pg_query.Node_MergeStmt, *pg_query.Node_CopyStmt:
		return ClassWrite, strings.TrimPrefix(fmt.Sprintf("%T", node.GetNode()), "*pg_query.Node_")
	case *pg_query.Node_DropStmt, *pg_query.Node_TruncateStmt, *pg_query.Node_DropdbStmt, *pg_query.Node_DropRoleStmt, *pg_query.Node_DropOwnedStmt, *pg_query.Node_DropSubscriptionStmt, *pg_query.Node_DropTableSpaceStmt, *pg_query.Node_DropUserMappingStmt:
		return ClassDestructive, strings.TrimPrefix(fmt.Sprintf("%T", node.GetNode()), "*pg_query.Node_")
	case *pg_query.Node_AlterTableStmt,
		*pg_query.Node_AlterDatabaseStmt,
		*pg_query.Node_AlterDatabaseRefreshCollStmt,
		*pg_query.Node_AlterDatabaseSetStmt,
		*pg_query.Node_AlterDefaultPrivilegesStmt,
		*pg_query.Node_AlterDomainStmt,
		*pg_query.Node_AlterEnumStmt,
		*pg_query.Node_AlterEventTrigStmt,
		*pg_query.Node_AlterExtensionStmt,
		*pg_query.Node_AlterExtensionContentsStmt,
		*pg_query.Node_AlterFdwStmt,
		*pg_query.Node_AlterForeignServerStmt,
		*pg_query.Node_AlterFunctionStmt,
		*pg_query.Node_AlterObjectDependsStmt,
		*pg_query.Node_AlterObjectSchemaStmt,
		*pg_query.Node_AlterOperatorStmt,
		*pg_query.Node_AlterOpFamilyStmt,
		*pg_query.Node_AlterOwnerStmt,
		*pg_query.Node_AlterPolicyStmt,
		*pg_query.Node_AlterPublicationStmt,
		*pg_query.Node_AlterRoleStmt,
		*pg_query.Node_AlterRoleSetStmt,
		*pg_query.Node_AlterSeqStmt,
		*pg_query.Node_AlterStatsStmt,
		*pg_query.Node_AlterSubscriptionStmt,
		*pg_query.Node_AlterSystemStmt,
		*pg_query.Node_AlterTableMoveAllStmt,
		*pg_query.Node_AlterTableSpaceOptionsStmt,
		*pg_query.Node_AlterTsdictionaryStmt,
		*pg_query.Node_AlterTsconfigurationStmt,
		*pg_query.Node_AlterTypeStmt,
		*pg_query.Node_AlterUserMappingStmt,
		*pg_query.Node_ClusterStmt,
		*pg_query.Node_CommentStmt,
		*pg_query.Node_ConstraintsSetStmt,
		*pg_query.Node_CreateAmStmt,
		*pg_query.Node_CreateCastStmt,
		*pg_query.Node_CreateConversionStmt,
		*pg_query.Node_CreatedbStmt,
		*pg_query.Node_CreateDomainStmt,
		*pg_query.Node_CreateEnumStmt,
		*pg_query.Node_CreateEventTrigStmt,
		*pg_query.Node_CreateExtensionStmt,
		*pg_query.Node_CreateFdwStmt,
		*pg_query.Node_CreateForeignServerStmt,
		*pg_query.Node_CreateForeignTableStmt,
		*pg_query.Node_CreateFunctionStmt,
		*pg_query.Node_CreateOpClassStmt,
		*pg_query.Node_CreateOpFamilyStmt,
		*pg_query.Node_CreatePolicyStmt,
		*pg_query.Node_CreatePublicationStmt,
		*pg_query.Node_CreateRangeStmt,
		*pg_query.Node_CreateRoleStmt,
		*pg_query.Node_CreateSchemaStmt,
		*pg_query.Node_CreateSeqStmt,
		*pg_query.Node_CreateStatsStmt,
		*pg_query.Node_CreateStmt,
		*pg_query.Node_CreateSubscriptionStmt,
		*pg_query.Node_CreateTableAsStmt,
		*pg_query.Node_CreateTableSpaceStmt,
		*pg_query.Node_CreateTransformStmt,
		*pg_query.Node_CreateTrigStmt,
		*pg_query.Node_CreateUserMappingStmt,
		*pg_query.Node_DeclareCursorStmt,
		*pg_query.Node_DiscardStmt,
		*pg_query.Node_DoStmt,
		*pg_query.Node_ExplainStmt,
		*pg_query.Node_GrantRoleStmt,
		*pg_query.Node_GrantStmt,
		*pg_query.Node_ImportForeignSchemaStmt,
		*pg_query.Node_IndexStmt,
		*pg_query.Node_ListenStmt,
		*pg_query.Node_LoadStmt,
		*pg_query.Node_LockStmt,
		*pg_query.Node_NotifyStmt,
		*pg_query.Node_PrepareStmt,
		*pg_query.Node_ReassignOwnedStmt,
		*pg_query.Node_RefreshMatViewStmt,
		*pg_query.Node_ReindexStmt,
		*pg_query.Node_RenameStmt,
		*pg_query.Node_ReplicaIdentityStmt,
		*pg_query.Node_RuleStmt,
		*pg_query.Node_SecLabelStmt,
		*pg_query.Node_TransactionStmt,
		*pg_query.Node_UnlistenStmt,
		*pg_query.Node_VacuumStmt,
		*pg_query.Node_VariableSetStmt,
		*pg_query.Node_ViewStmt:
		return ClassAdmin, strings.TrimPrefix(fmt.Sprintf("%T", node.GetNode()), "*pg_query.Node_")
	default:
		return ClassUnknown, strings.TrimPrefix(fmt.Sprintf("%T", node.GetNode()), "*pg_query.Node_")
	}
}

func higherClass(left, right Class) Class {
	if severity(right) > severity(left) {
		return right
	}
	return left
}

func severity(class Class) int {
	switch class {
	case ClassRead:
		return 1
	case ClassWrite:
		return 2
	case ClassAdmin:
		return 3
	case ClassDestructive:
		return 4
	case ClassUnknown:
		return 5
	default:
		return 0
	}
}
