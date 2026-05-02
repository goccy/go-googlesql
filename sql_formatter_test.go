package googlesql_test

import (
	"strings"
	"testing"

	"github.com/goccy/go-googlesql"
)

// TestSQLFormatter round-trips a SELECT statement through the parser
// followed by Unparse(root) and verifies the canonical form is
// recognizably SQL. googlesql::Unparse is the parser-level formatter
// that public FormatSql wraps; we point at Unparse directly because the
// analyzer bazel target this project is built from does not pull in the
// stand-alone sql_formatter.cc object.
func TestSQLFormatter(t *testing.T) {
	t.Cleanup(gc)

	opts, err := googlesql.NewParserOptions()
	if err != nil {
		t.Fatalf("NewParserOptions: %v", err)
	}
	out, err := googlesql.ParseStatement("SELECT * FROM Samples WHERE id = 1", opts)
	if err != nil {
		t.Fatalf("ParseStatement: %v", err)
	}
	stmt, err := out.Statement()
	if err != nil {
		t.Fatalf("Statement: %v", err)
	}

	formatted, err := googlesql.Unparse(stmt)
	if err != nil {
		t.Fatalf("Unparse: %v", err)
	}
	upper := strings.ToUpper(formatted)
	for _, tok := range []string{"SELECT", "FROM", "SAMPLES", "WHERE", "ID"} {
		if !strings.Contains(upper, tok) {
			t.Errorf("Unparse output missing token %q: %q", tok, formatted)
		}
	}
}
