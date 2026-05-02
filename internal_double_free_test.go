package googlesql

import (
	"runtime"
	"sync"
	"testing"
)

// These tests live in package googlesql (not googlesql_test) so they can
// read the unexported `ptr` field of generated handle structs. The
// observable signal of the double-free bug is that a method whose C++
// counterpart absorbs the wrapped pointer (a `unique_ptr<T>` parameter)
// leaves the Go-side handle's `ptr` non-zero — the per-instance
// finalizer would then free the same memory the C++ destructor of the
// owning parent will (or already did) reclaim. After the fix the same
// scenario must end with the child's ptr cleared to 0, so the
// finalizer's `free()` short-circuits via its `if h.ptr != 0` guard.
//
// Each test exercises a different unique_ptr-taking method to guard
// against an accidental "fixed only for one site" regression.

var initOnceForTests sync.Once

func ensureInit(t *testing.T) {
	t.Helper()
	initOnceForTests.Do(func() {
		if err := Init(WithCompilationMode(CompilationModeCompiler)); err != nil {
			t.Fatalf("googlesql.Init: %v", err)
		}
	})
}

// TestSimpleTableAddColumn_TransfersOwnership exercises the exact
// scenario from the original bug report: SimpleTable::AddColumn takes
// a `std::unique_ptr<const Column>`. After the call, the Go-side
// SimpleColumn must have ptr==0 so the finalizer cannot double-free.
func TestSimpleTableAddColumn_TransfersOwnership(t *testing.T) {
	ensureInit(t)

	tf, err := NewTypeFactory()
	if err != nil {
		t.Fatalf("NewTypeFactory: %v", err)
	}
	intType, err := tf.GetInt64()
	if err != nil {
		t.Fatalf("GetInt64: %v", err)
	}

	col, err := NewSimpleColumn("t", "c", intType, false, true)
	if err != nil {
		t.Fatalf("NewSimpleColumn: %v", err)
	}
	if col.ptr == 0 {
		t.Fatalf("freshly-constructed column should have ptr != 0")
	}
	ptrBefore := col.ptr

	tbl, err := NewSimpleTable("t", -1)
	if err != nil {
		t.Fatalf("NewSimpleTable: %v", err)
	}

	if err := tbl.AddColumn(col); err != nil {
		t.Fatalf("AddColumn: %v", err)
	}

	if col.ptr != 0 {
		t.Fatalf("after AddColumn (unique_ptr take-ownership) col.ptr must be 0; got %d (was %d before the call). Without this clear, the per-instance finalizer would later issue a Free RPC against memory the owning SimpleTable's destructor already deletes — classic double-free.",
			col.ptr, ptrBefore)
	}

	// Force GC twice: the column wrapper is now unreachable (we do not
	// keep a Go reference). Its finalizer fires; with ptr==0 the
	// generated free() short-circuits. If it freed, the next access
	// through tbl would touch garbage.
	col = nil
	runtime.GC()
	runtime.GC()

	// Sanity: the parent should still serve accessors. If the
	// double-free corrupted the heap, this is where wazero typically
	// trips (vtable read against freed memory).
	name, err := tbl.Name()
	if err != nil {
		t.Fatalf("tbl.Name after GC: %v", err)
	}
	if name != "t" {
		t.Fatalf("tbl.Name after GC: got %q want %q", name, "t")
	}

	runtime.KeepAlive(tbl)
}

// TestSimpleCatalogAddOwnedCatalog_TransfersOwnership exercises the
// 1-arg unique_ptr variant of SimpleCatalog::AddOwnedCatalog (the
// `AddOwnedCatalog2` rpc in the proto, since proto disambiguates
// overloads by suffix). Same shape as the SimpleTable case but a
// different parent/child pair, to prove the fix is not site-specific.
func TestSimpleCatalogAddOwnedCatalog_TransfersOwnership(t *testing.T) {
	ensureInit(t)

	tf, err := NewTypeFactory()
	if err != nil {
		t.Fatalf("NewTypeFactory: %v", err)
	}
	parent, err := NewSimpleCatalog("parent", tf)
	if err != nil {
		t.Fatalf("NewSimpleCatalog parent: %v", err)
	}
	child, err := NewSimpleCatalog("child", tf)
	if err != nil {
		t.Fatalf("NewSimpleCatalog child: %v", err)
	}
	if child.ptr == 0 {
		t.Fatalf("freshly-constructed child catalog should have ptr != 0")
	}

	if err := parent.AddOwnedCatalog2(child); err != nil {
		t.Fatalf("AddOwnedCatalog2: %v", err)
	}

	if child.ptr != 0 {
		t.Fatalf("after AddOwnedCatalog2 (unique_ptr take-ownership) child.ptr must be 0; got %d", child.ptr)
	}

	child = nil
	runtime.GC()
	runtime.GC()

	if name, err := parent.FullName(); err != nil {
		t.Fatalf("parent.FullName after GC: %v", err)
	} else if name == "" {
		t.Fatalf("parent.FullName after GC was empty")
	}
	runtime.KeepAlive(parent)
}

// TestSimpleTableAddColumn2_DoesNotTransferOwnership pins the negative
// case: SimpleTable::AddColumn(Column*, bool is_owned) takes a raw
// pointer plus a runtime ownership flag. The bridge cannot statically
// prove the C++ side will absorb the pointer (it depends on
// is_owned), so the generator must NOT emit the auto-clearPtr.
// Otherwise the safe path (is_owned=false, where the caller still
// holds the Column) would lose its handle.
func TestSimpleTableAddColumn2_DoesNotTransferOwnership(t *testing.T) {
	ensureInit(t)

	tf, err := NewTypeFactory()
	if err != nil {
		t.Fatalf("NewTypeFactory: %v", err)
	}
	intType, err := tf.GetInt64()
	if err != nil {
		t.Fatalf("GetInt64: %v", err)
	}
	col, err := NewSimpleColumn("t", "c", intType, false, true)
	if err != nil {
		t.Fatalf("NewSimpleColumn: %v", err)
	}
	tbl, err := NewSimpleTable("t", -1)
	if err != nil {
		t.Fatalf("NewSimpleTable: %v", err)
	}

	// is_owned=false: the C++ side does NOT take ownership. The Go
	// side must keep col.ptr intact so its finalizer can free it.
	if err := tbl.AddColumn2(col, false); err != nil {
		t.Fatalf("AddColumn2: %v", err)
	}
	if col.ptr == 0 {
		t.Fatalf("AddColumn2(is_owned=false) must NOT clear col.ptr; got 0")
	}
	runtime.KeepAlive(tbl)
	runtime.KeepAlive(col)
}
