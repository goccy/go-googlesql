package googlesql_test

import (
	"testing"

	"github.com/goccy/go-googlesql"
)

// TestAnalyzeMultiStatements drives AnalyzeNextStatement on a script
// with two statements and verifies both analyze without error.
//
// The googlesql::AnalyzeNextStatement C++ API reports completion via an
// out-param `at_end_of_input`, but the plugin's Go binding returns only
// the first output (*AnalyzerOutput). We detect end-of-input by
// comparing the ParseResumeLocation's byte position against the input
// length after each call; once it reaches the end we stop iterating.
func TestAnalyzeMultiStatements(t *testing.T) {
	t.Cleanup(gc)
	catalog, opts, tf, _ := setupAnalyzer(t)

	const sql = "SELECT 1 AS a; SELECT 2 AS b;"
	loc, err := googlesql.NewParseResumeLocationFromString(sql)
	if err != nil {
		t.Fatalf("NewParseResumeLocationFromString: %v", err)
	}

	var got []string
	for i := 0; i < 5; i++ {
		pos, err := loc.BytePosition()
		if err != nil {
			t.Fatalf("BytePosition #%d: %v", i, err)
		}
		if int(pos) >= len(sql) {
			break
		}
		out, err := googlesql.AnalyzeNextStatement(loc, opts, catalog, tf)
		if err != nil {
			t.Fatalf("AnalyzeNextStatement #%d at pos=%d: %v", i, pos, err)
		}
		stmt, err := out.ResolvedStatement()
		if err != nil {
			t.Fatalf("ResolvedStatement #%d: %v", i, err)
		}
		qs, ok := stmt.(*googlesql.ResolvedQueryStmt)
		if !ok {
			t.Fatalf("expected *ResolvedQueryStmt, got %T", stmt)
		}
		kind, err := qs.NodeKindString()
		if err != nil {
			t.Fatalf("NodeKindString: %v", err)
		}
		got = append(got, kind)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 parsed statements, got %d: %v", len(got), got)
	}
}
