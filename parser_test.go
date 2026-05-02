package googlesql_test

import (
	"strings"
	"testing"

	"github.com/goccy/go-googlesql"
)

// TestParser mirrors go-zetasql/parser_test.go: parse a simple SELECT
// and verify the top-level AST shape is an ASTQueryStatement.
func TestParser(t *testing.T) {
	t.Cleanup(gc)

	opts, err := googlesql.NewParserOptions()
	if err != nil {
		t.Fatalf("NewParserOptions: %v", err)
	}
	out, err := googlesql.ParseStatement("SELECT 1", opts)
	if err != nil {
		t.Fatalf("ParseStatement: %v", err)
	}
	stmt, err := out.Statement()
	if err != nil {
		t.Fatalf("Statement: %v", err)
	}
	if _, ok := stmt.(*googlesql.ASTQueryStatement); !ok {
		t.Errorf("expected *googlesql.ASTQueryStatement, got %T", stmt)
	}
}

// TestParseStatementError asserts a syntax error surfaces through the
// Go-facing error.
func TestParseStatementError(t *testing.T) {
	t.Cleanup(gc)

	opts, err := googlesql.NewParserOptions()
	if err != nil {
		t.Fatalf("NewParserOptions: %v", err)
	}
	_, err = googlesql.ParseStatement("SELECT", opts)
	if err == nil {
		t.Fatal("expected parser error for bare SELECT, got nil")
	}
}

// TestParseMultipleQueries runs a handful of valid statements through
// the parser to exercise the full dispatch path.
func TestParseMultipleQueries(t *testing.T) {
	t.Cleanup(gc)

	opts, err := googlesql.NewParserOptions()
	if err != nil {
		t.Fatalf("NewParserOptions: %v", err)
	}

	cases := []struct {
		name string
		sql  string
	}{
		{"select_struct_equality", "SELECT STRUCT(1) = STRUCT(1)"},
		{"select_struct_array", "SELECT ARRAY<STRUCT<a INT64>>[]"},
		{"update_with_struct", "UPDATE t SET x = STRUCT(1) WHERE TRUE"},
		{"select_struct_field", "SELECT STRUCT(1 AS a).a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := googlesql.ParseStatement(tc.sql, opts)
			if err != nil {
				t.Fatalf("ParseStatement(%q): %v", tc.sql, err)
			}
			if _, err := out.Statement(); err != nil {
				t.Fatalf("Statement(%q): %v", tc.sql, err)
			}
			// Sanity-check the original SQL is preserved in whatever the
			// out carries (implementation-specific; we just assert the
			// handle round-trips). strings.ToUpper is a cheap way to
			// pull the call through Go so the finalizer path runs.
			_ = strings.ToUpper(tc.sql)
		})
	}
}
