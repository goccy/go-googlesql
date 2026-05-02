package googlesql_test

import (
	"strings"
	"testing"

	"github.com/goccy/go-googlesql"
)

// setupAnalyzer builds the analysis infrastructure (TypeFactory,
// LanguageOptions, Catalog with googlesql built-ins registered,
// AnalyzerOptions) once per test. If the wasm doesn't support
// NewAnalyzerOptions2 the test is skipped — the end-to-end path needs
// the analyzer linked in.
func setupAnalyzer(t *testing.T) (*googlesql.SimpleCatalog, *googlesql.AnalyzerOptions, *googlesql.TypeFactory, *googlesql.LanguageOptions) {
	t.Helper()

	catalog, err := googlesql.NewSimpleCatalog("test_catalog", nil)
	if err != nil {
		t.Fatalf("NewSimpleCatalog: %v", err)
	}
	langOpts, err := googlesql.NewLanguageOptions()
	if err != nil {
		t.Fatalf("NewLanguageOptions: %v", err)
	}
	if err := langOpts.EnableMaximumLanguageFeaturesForDevelopment(); err != nil {
		t.Fatalf("EnableMaximumLanguageFeaturesForDevelopment: %v", err)
	}
	if err := langOpts.SetSupportsAllStatementKinds(); err != nil {
		t.Fatalf("SetSupportsAllStatementKinds: %v", err)
	}

	// Register builtins via the BuiltinFunctionOptions value-type struct.
	// This exercises the plugin → bridge value-type path end-to-end.
	bfo := &googlesql.BuiltinFunctionOptions{LanguageOptions: langOpts}
	if err := catalog.AddBuiltinFunctionsAndTypes(bfo); err != nil {
		t.Fatalf("AddBuiltinFunctionsAndTypes: %v", err)
	}

	opts, err := googlesql.NewAnalyzerOptions2()
	if err != nil {
		t.Skipf("NewAnalyzerOptions2 not supported in this wasm env: %v", err)
	}
	if err := opts.SetLanguage(langOpts); err != nil {
		t.Fatalf("SetLanguage: %v", err)
	}

	tf, err := googlesql.NewTypeFactory()
	if err != nil {
		t.Fatalf("NewTypeFactory: %v", err)
	}
	return catalog, opts, tf, langOpts
}

// TestAnalyzer mirrors go-zetasql/analyzer_test.go: analyze a SELECT
// that returns two typed columns (an integer literal and a string
// literal) and walk the ResolvedQueryStmt's output columns. A
// user-provided table isn't used because the public API doesn't yet
// expose a Column upcast from SimpleColumn — wiring that up is tracked
// as a follow-up in the plugin.
func TestAnalyzer(t *testing.T) {
	t.Cleanup(gc)
	catalog, opts, tf, _ := setupAnalyzer(t)

	out, err := googlesql.AnalyzeStatement("SELECT 1 AS col1, 'hi' AS col2", opts, catalog, tf)
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
	if size != 2 {
		t.Fatalf("output column count = %d, want 2", size)
	}

	wantNames := []string{"col1", "col2"}
	for i := int32(0); i < size; i++ {
		col, err := qs.OutputColumnList2(i)
		if err != nil {
			t.Fatalf("OutputColumnList2(%d): %v", i, err)
		}
		rc, err := col.Column()
		if err != nil {
			t.Fatalf("OutputColumnList2(%d).Column: %v", i, err)
		}
		name, err := rc.Name()
		if err != nil {
			t.Fatalf("ResolvedColumn[%d].Name: %v", i, err)
		}
		if name != wantNames[i] {
			t.Errorf("column %d name = %q, want %q", i, name, wantNames[i])
		}
	}
}

// TestAnalyzeStatementError asserts that a query referencing an unknown
// table surfaces an analyzer-level error (as opposed to a syntax
// error).
func TestAnalyzeStatementError(t *testing.T) {
	t.Cleanup(gc)
	catalog, opts, tf, _ := setupAnalyzer(t)
	_, err := googlesql.AnalyzeStatement("SELECT * FROM no_such_table", opts, catalog, tf)
	if err == nil {
		t.Fatal("expected analyzer error for unknown table, got nil")
	}
	if !strings.Contains(err.Error(), "no_such_table") && !strings.Contains(err.Error(), "Table") {
		t.Errorf("expected error mentioning the missing table, got: %v", err)
	}
}

// TestAnalyzeStatementResolved runs analysis on a statement using only
// built-in types and checks the resolved AST has the expected shape.
func TestAnalyzeStatementResolved(t *testing.T) {
	t.Cleanup(gc)
	catalog, opts, tf, _ := setupAnalyzer(t)

	out, err := googlesql.AnalyzeStatement("SELECT 1 AS num, 'hello' AS greeting", opts, catalog, tf)
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
	if size != 2 {
		t.Errorf("OutputColumnListSize = %d, want 2", size)
	}
	isVT, err := qs.IsValueTable()
	if err != nil {
		t.Fatalf("IsValueTable: %v", err)
	}
	if isVT {
		t.Error("IsValueTable = true, want false for scalar SELECT")
	}

	maxColID, err := out.MaxColumnId()
	if err != nil {
		t.Fatalf("MaxColumnId: %v", err)
	}
	if maxColID < 1 {
		t.Errorf("MaxColumnId = %d, want >= 1", maxColID)
	}
}
