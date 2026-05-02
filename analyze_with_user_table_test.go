package googlesql_test

import (
	"strings"
	"testing"

	"github.com/goccy/go-googlesql"
)

// TestAnalyzeWithUserTable proves a SELECT can resolve against a user-
// supplied schema: build a SimpleTable from Go (with one INT64 column),
// register it on the analyzer's SimpleCatalog, analyze
// `SELECT id FROM users`, and verify the resolved query has the
// expected output column.
//
// This exercises the full handle bridge for schema setup (SimpleTable,
// SimpleColumn, TypeFactory, SimpleCatalog) alongside the analyzer —
// no callback is involved, the Catalog is C++-side but populated from
// Go.
func TestAnalyzeWithUserTable(t *testing.T) {
	t.Cleanup(gc)
	catalog, opts, tf, _ := setupAnalyzer(t)

	int64Type, err := tf.GetInt64()
	if err != nil {
		t.Fatalf("TypeFactory.GetInt64: %v", err)
	}

	table, err := googlesql.NewSimpleTable("users", 0)
	if err != nil {
		t.Fatalf("NewSimpleTable: %v", err)
	}

	col, err := googlesql.NewSimpleColumn("users", "id", int64Type, false, true)
	if err != nil {
		t.Fatalf("NewSimpleColumn: %v", err)
	}
	if err := table.AddColumn2(col, false); err != nil {
		t.Fatalf("SimpleTable.AddColumn2: %v", err)
	}

	if err := catalog.AddTable(table); err != nil {
		t.Fatalf("SimpleCatalog.AddTable: %v", err)
	}

	out, err := googlesql.AnalyzeStatement("SELECT id FROM users", opts, catalog, tf)
	if err != nil {
		t.Fatalf("AnalyzeStatement: %v", err)
	}
	stmt, err := out.ResolvedStatement()
	if err != nil {
		t.Fatalf("ResolvedStatement: %v", err)
	}
	qs, ok := stmt.(*googlesql.ResolvedQueryStmt)
	if !ok {
		t.Fatalf("expected *ResolvedQueryStmt, got %T", stmt)
	}

	size, err := qs.OutputColumnListSize()
	if err != nil {
		t.Fatalf("OutputColumnListSize: %v", err)
	}
	if size != 1 {
		t.Fatalf("output column count = %d, want 1", size)
	}

	oc, err := qs.OutputColumnList2(0)
	if err != nil {
		t.Fatalf("OutputColumnList2(0): %v", err)
	}
	rc, err := oc.Column()
	if err != nil {
		t.Fatalf("Column: %v", err)
	}
	name, err := rc.Name()
	if err != nil {
		t.Fatalf("ResolvedColumn.Name: %v", err)
	}
	if name != "id" {
		t.Errorf("column name = %q, want %q", name, "id")
	}
}

// TestAnalyzeWithUserTableMissing confirms analysis surfaces an error
// when the query references a table that was not registered on the
// catalog.
func TestAnalyzeWithUserTableMissing(t *testing.T) {
	t.Cleanup(gc)
	catalog, opts, tf, _ := setupAnalyzer(t)

	_, err := googlesql.AnalyzeStatement("SELECT * FROM missing_table", opts, catalog, tf)
	if err == nil {
		t.Fatal("expected analyzer error for missing table, got nil")
	}
	if !strings.Contains(err.Error(), "missing_table") {
		t.Errorf("expected error mentioning missing_table, got: %v", err)
	}
}
