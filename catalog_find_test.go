package googlesql_test

import (
	"fmt"
	"testing"

	"github.com/goccy/go-googlesql"
)

// catalogWithUsers is a Go-implemented Catalog that resolves a single
// table `users`. FindTable is the non-pure virtual the analyzer calls
// during name resolution; everything else falls through to the
// stub-only CatalogCallbackDefaults embedding.
type catalogWithUsers struct {
	googlesql.CatalogCallbackDefaults

	t     *testing.T
	users *googlesql.SimpleTable

	findCalls int
}

func (c *catalogWithUsers) FullName() (string, error) { return "custom_catalog", nil }

func (c *catalogWithUsers) FindTable(path []string, options *googlesql.FindOptions) (googlesql.TableNode, error) {
	c.findCalls++
	if len(path) == 1 && path[0] == "users" {
		return c.users, nil
	}
	return nil, fmt.Errorf("table not found: %v", path)
}

// TestAnalyzeWithGoCatalog drives AnalyzeStatement against a Go-side
// CatalogCallback. The SELECT references a `users` table that lives
// only in the Go impl's memory — resolution has to round-trip through
// the callback mechanism:
//
//  1. Go creates a SimpleTable (a real C++ handle) and configures
//     it with one INT64 column.
//  2. Go registers a catalogWithUsers impl via NewCatalogFromImpl;
//     the resulting *Catalog handle is a trampoline whose vtable
//     forwards FindTable etc. back into Go via wasmify_callback_invoke.
//  3. Go calls AnalyzeStatement passing that *Catalog. The C++
//     resolver invokes catalog->FindTable({"users"}, &table, ...)
//     which lands in the trampoline, which marshals the path and
//     calls back into Go.
//  4. The Go impl returns the pre-built SimpleTable; the trampoline
//     unpacks the handle and writes it through the out pointer.
//  5. Resolution proceeds and the analyzer reports the output column.
//
// The assertion on `findCalls > 0` confirms the callback round trip
// actually happened (without this, the test would pass if the
// resolver found the table via some fallback path).
func TestAnalyzeWithGoCatalog(t *testing.T) {
	t.Cleanup(gc)

	// Build a SimpleTable the Go impl will hand out on FindTable.
	// Setup uses the bridged SimpleCatalog only to borrow the
	// TypeFactory / AnalyzerOptions — the analyzer itself receives
	// the Go-backed *Catalog.
	_, opts, tf, _ := setupAnalyzer(t)

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

	impl := &catalogWithUsers{t: t, users: table}
	gocat, err := googlesql.NewCatalogFromImpl(impl)
	if err != nil {
		t.Fatalf("NewCatalogFromImpl: %v", err)
	}

	out, err := googlesql.AnalyzeStatement("SELECT id FROM users", opts, gocat, tf)
	if err != nil {
		t.Fatalf("AnalyzeStatement: %v", err)
	}
	if impl.findCalls == 0 {
		t.Error("expected FindTable to be called at least once, got 0")
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
		t.Errorf("output column count = %d, want 1", size)
	}
}
