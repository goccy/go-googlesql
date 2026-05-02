package googlesql_test

import (
	"testing"

	"github.com/goccy/go-googlesql"
)

// goCatalog is a tiny in-process CatalogCallback implementation used to
// prove the Go -> C++ -> Go round trip for abstract-class virtuals.
// FullName is the only pure-virtual on googlesql::Catalog whose
// signature survives the trampoline's marshalling support (primitive/
// string/handle); the other pure virtuals on the base have
// absl::Status returns and absl::flat_hash_set output parameters and
// disqualify their holding classes (EnumerableCatalog) from becoming
// callback candidates in the current generator.
type goCatalog struct {
	googlesql.CatalogCallbackDefaults

	calls    int
	fullName string
}

func (g *goCatalog) FullName() (string, error) {
	g.calls++
	return g.fullName, nil
}

func TestCatalogCallback(t *testing.T) {
	t.Cleanup(gc)

	impl := &goCatalog{fullName: "mycatalog"}
	cat, err := googlesql.NewCatalogFromImpl(impl)
	if err != nil {
		t.Fatalf("NewCatalogFromImpl: %v", err)
	}
	if cat == nil {
		t.Fatal("NewCatalogFromImpl returned nil handle")
	}

	got, err := cat.FullName()
	if err != nil {
		t.Fatalf("FullName: %v", err)
	}
	if got != "mycatalog" {
		t.Errorf("FullName = %q, want %q", got, "mycatalog")
	}
	if impl.calls != 1 {
		t.Errorf("Go impl called %d times, want 1", impl.calls)
	}
}
