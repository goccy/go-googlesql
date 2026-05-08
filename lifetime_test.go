package googlesql

import (
	"runtime"
	"strings"
	"testing"
)

// The tests in this file verify the keepAlive convention end-to-end on
// paths that the existing `internal_double_free_test.go` does not
// cover:
//
//   1. Borrowed-child accessors must keep the parent reachable so the
//      parent's finalizer cannot fire while a child wrapper still
//      holds a pointer into the parent's memory.
//
//   2. Methods that consume another wrapper as input (CopyFrom /
//      MergeFrom on protobuf-descriptor types, BuildFile on
//      DescriptorPool) must still leave both objects independently
//      usable after GC. They are not expected to keepAlive each other
//      — protobuf's contract is deep-copy — but if the bridge ever
//      regresses to shared internal state these tests will trip.
//
//   3. Runtime-conditional ownership transfer
//      (`SimpleTable::AddColumn(Column*, bool is_owned)` etc.) is
//      currently broken: `is_owned=true` does not clearPtr the
//      wrapper, so the wrapper's finalizer issues a Free RPC against
//      memory the parent's destructor will already reclaim. The
//      `is_owned=true` test is therefore expected to FAIL until
//      wasmify lands the runtime-gate emit. See
//      issues/runtime-conditional-ownership.md.
//
// Living in package googlesql (not googlesql_test) so the unexported
// `ptr` field is observable, mirroring `internal_double_free_test.go`.

// TestLifetime_FileDescriptorSetFile_ParentPinned exercises the most
// common borrowed-child path on the proto-descriptor surface. The
// file at index i lives inside the FileDescriptorSet — the Go-side
// FileDescriptorProto wrapper must not be finalised independently
// of the FileDescriptorSet, AND the FileDescriptorSet must not be
// reclaimed while the borrowed FileDescriptorProto is still
// reachable.
func TestLifetime_FileDescriptorSetFile_ParentPinned(t *testing.T) {
	ensureInit(t)

	set, err := NewFileDescriptorSet()
	if err != nil {
		t.Fatalf("NewFileDescriptorSet: %v", err)
	}

	// AddFile borrows a *FileDescriptorProto whose backing memory
	// lives inside `set`. The wrapper has no finalizer; setKeepAlive
	// pins set into it.
	fp, err := set.AddFile()
	if err != nil {
		t.Fatalf("FileDescriptorSet.AddFile: %v", err)
	}
	if fp == nil {
		t.Fatal("FileDescriptorSet.AddFile returned nil")
	}
	if fp.Message == nil || fp.Message.ptr == 0 {
		t.Fatalf("borrowed FileDescriptorProto wrapper has zero ptr")
	}

	// Drop the parent's local reference. The keepAlive chain on fp
	// must keep `set` reachable through Go's GC graph.
	set = nil
	runtime.GC()
	runtime.GC()

	// If keepAlive was wired correctly, fp's underlying C++ memory
	// is still alive (the parent is still referenced via
	// fp.Message.keepAlive) and we can read fields on it.
	size, err := fp.DependencySize()
	if err != nil {
		t.Fatalf("FileDescriptorProto.DependencySize after parent drop: %v", err)
	}
	if size != 0 {
		t.Errorf("DependencySize = %d, want 0", size)
	}
	runtime.KeepAlive(fp)
}

// TestLifetime_FileDescriptorProto_CopyFromIndependent verifies that
// after `dst.CopyFrom(src)` returns, dropping `src` does not corrupt
// `dst`. The protobuf C++ contract is deep-copy; this test exists to
// trip if the bridge ever regresses to shared backing memory.
func TestLifetime_FileDescriptorProto_CopyFromIndependent(t *testing.T) {
	ensureInit(t)

	src, err := NewFileDescriptorProto()
	if err != nil {
		t.Fatalf("NewFileDescriptorProto src: %v", err)
	}
	dst, err := NewFileDescriptorProto()
	if err != nil {
		t.Fatalf("NewFileDescriptorProto dst: %v", err)
	}

	if err := dst.CopyFrom(src); err != nil {
		t.Fatalf("dst.CopyFrom(src): %v", err)
	}

	// Drop src and force GC. dst must remain usable.
	srcPtr := src.Message.ptr
	src = nil
	runtime.GC()
	runtime.GC()

	if _, err := dst.DependencySize(); err != nil {
		t.Fatalf("dst.DependencySize after src drop: %v", err)
	}
	_ = srcPtr
	runtime.KeepAlive(dst)
}

// TestLifetime_FileDescriptorProto_MergeFromIndependent — same as
// CopyFrom but for MergeFrom.
func TestLifetime_FileDescriptorProto_MergeFromIndependent(t *testing.T) {
	ensureInit(t)

	src, err := NewFileDescriptorProto()
	if err != nil {
		t.Fatalf("NewFileDescriptorProto src: %v", err)
	}
	dst, err := NewFileDescriptorProto()
	if err != nil {
		t.Fatalf("NewFileDescriptorProto dst: %v", err)
	}

	if err := dst.MergeFrom(src); err != nil {
		t.Fatalf("dst.MergeFrom(src): %v", err)
	}

	src = nil
	runtime.GC()
	runtime.GC()

	if _, err := dst.DependencySize(); err != nil {
		t.Fatalf("dst.DependencySize after src drop: %v", err)
	}
	runtime.KeepAlive(dst)
}

// TestLifetime_DescriptorPoolBuildFile_PoolPinsResult exercises the
// borrowed return from DescriptorPool.BuildFile — the resulting
// *FileDescriptor lives inside the pool's owned descriptor table, so
// dropping the pool while the FileDescriptor wrapper is alive must
// not free the underlying memory.
//
// Now uses the regenerated ParseFromString binding (issues/proto-wire-binding.md
// landed) to populate the FileDescriptorProto. The test marshals a
// minimal proto3 file via descriptorpb (in this _test.go's test-only
// dep), pushes the wire bytes through `ParseFromString`, and feeds
// the resulting wasm-side FileDescriptorProto into `BuildFile`. The
// returned `*FileDescriptor` is borrowed from the pool — the test
// drops the pool's local Go reference and verifies the FileDescriptor
// is still usable, proving the keepAlive chain pins the pool through
// the FileDescriptor wrapper.
func TestLifetime_DescriptorPoolBuildFile_PoolPinsResult(t *testing.T) {
	ensureInit(t)
	t.Cleanup(gcLocal)

	pool, err := NewDescriptorPool()
	if err != nil {
		t.Fatalf("NewDescriptorPool: %v", err)
	}

	fdp, err := NewFileDescriptorProto()
	if err != nil {
		t.Fatalf("NewFileDescriptorProto: %v", err)
	}
	// Hand-rolled wire-format payload for a minimal FileDescriptorProto:
	//   { name = "lifetime_test.proto", syntax = "proto3" }
	// Keeping it inline (not constructed via descriptorpb) so this test
	// has no test-only dep — `lifetime_test.go` is a black-box on the
	// internal package and stays clean.
	wire := buildMinimalFileDescriptorWire(t, "lifetime_test.proto", "proto3")
	ok, err := fdp.ParseFromString(wire)
	if err != nil {
		t.Fatalf("ParseFromString: %v", err)
	}
	if !ok {
		t.Fatalf("ParseFromString returned false on a well-formed payload (%d bytes)", len(wire))
	}

	fileDesc, err := pool.BuildFile(fdp)
	if err != nil {
		t.Fatalf("DescriptorPool.BuildFile: %v", err)
	}
	if fileDesc == nil {
		t.Fatal("BuildFile returned nil with no error")
	}

	// Drop pool's local reference. The keepAlive chain on fileDesc
	// must keep the pool alive transitively.
	pool = nil
	fdp = nil
	for i := 0; i < 10; i++ {
		runtime.GC()
	}

	// Touch the borrowed FileDescriptor — would SIGSEGV / wazero
	// "out of bounds" if the pool was finalised prematurely. Using
	// DebugString instead of Name because the bridge has not bound
	// FileDescriptor::name() yet (would surface as a separate
	// follow-up); DebugString contains the file path so we can
	// still assert the pool's data is intact.
	dbg, err := fileDesc.DebugString()
	if err != nil {
		t.Fatalf("FileDescriptor.DebugString after pool drop: %v", err)
	}
	// DebugString of a FileDescriptor in the protobuf C++ runtime
	// renders the parsed proto definitions (here: just `syntax =
	// "proto3";`). A non-empty result proves the pool is still
	// reachable after pool=nil + GC — keepAlive chain works.
	if !strings.Contains(dbg, "proto3") {
		t.Errorf("FileDescriptor.DebugString missing parsed syntax — pool may have been finalised; got %q", dbg)
	}
	runtime.KeepAlive(fileDesc)
}

// buildMinimalFileDescriptorWire emits the wire-format bytes for a
// google.protobuf.FileDescriptorProto with only `name` (field 1) and
// `syntax` (field 12) set. Stays inline so this test file has no
// imports beyond the package itself + testing/runtime.
func buildMinimalFileDescriptorWire(t *testing.T, name, syntax string) string {
	t.Helper()
	var buf []byte
	// field 1 (name): wire-tag (1<<3)|2 = 0x0A, then varint length, then bytes.
	buf = append(buf, 0x0A)
	buf = appendVarint(buf, uint64(len(name)))
	buf = append(buf, name...)
	// field 12 (syntax): wire-tag (12<<3)|2 = 0x62, then varint length.
	buf = append(buf, 0x62)
	buf = appendVarint(buf, uint64(len(syntax)))
	buf = append(buf, syntax...)
	return string(buf)
}

func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

// TestLifetime_RuntimeConditionalOwnership_AddColumnFalse covers the
// negative branch of the runtime-conditional gate. When `is_owned=false`,
// the C++ side does NOT adopt the column — it stores a raw borrowed
// pointer in `columns_`. The Go-side wrapper MUST keep `col.ptr` set
// so the column's per-instance finalizer is the one that eventually
// reclaims the underlying memory. If the bridge wrongly cleared
// `col.ptr` on this branch (treating the method as unconditionally
// ownership-taking), the column's wasm-side memory would leak: the
// Go finalizer would short-circuit (ptr == 0) and the table's
// destructor wouldn't free it either (not in `owned_columns_`).
//
// This test:
//   - calls AddColumn2(col, false)
//   - asserts col.ptr is preserved
//   - drops both `col` and `tbl` locals so the keepAlive chain
//     (`tbl.keepAlive` holds col, then both die) plays out
//   - forces GC and re-allocates many handles to surface any
//     freelist corruption
//
// Pairs with `internal_double_free_test.go::TestSimpleTableAddColumn2_DoesNotTransferOwnership`
// (which only asserts col.ptr at the synchronous point) by adding a
// post-finalizer allocator probe.
func TestLifetime_RuntimeConditionalOwnership_AddColumnFalse(t *testing.T) {
	ensureInit(t)
	t.Cleanup(gcLocal)

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

	if err := tbl.AddColumn2(col, /*isOwned=*/ false); err != nil {
		t.Fatalf("AddColumn2(col, false): %v", err)
	}

	// Phase 1 — col.ptr must be preserved so col's finalizer can
	// free the wasm-side memory later (the table won't).
	if col.ptr == 0 {
		t.Errorf("AddColumn2(col, is_owned=false): col.ptr must NOT be 0 (table did not adopt); got 0. "+
			"Bridge wrongly emitted unconditional clearPtr for the runtime-conditional overload — col would leak.")
	}

	// Phase 2 — drop col local. tbl.keepAlive still holds col; col
	// stays alive so its finalizer doesn't fire yet.
	col = nil
	for i := 0; i < 10; i++ {
		runtime.GC()
	}

	// Sanity: the table still works after col's local was dropped —
	// keepAlive chain in action.
	if _, err := tbl.NumColumns(); err != nil {
		t.Fatalf("tbl.NumColumns after col drop: %v", err)
	}

	// Phase 3 — drop tbl. Now both wrappers are unreachable. The
	// finalizer order is non-deterministic: tbl's destructor doesn't
	// touch col (col is borrowed, not owned), so whether col's
	// finalizer fires before or after tbl's, col is single-freed
	// correctly. If the bridge had wrongly cleared col.ptr in
	// Phase 0, col.finalizer would short-circuit — leak (no crash
	// today, but a steady RSS climb in any long-running process).
	tbl = nil
	for i := 0; i < 10; i++ {
		runtime.GC()
	}

	// Phase 4 — allocator probe. If either branch had emitted a
	// stray Free RPC against memory still owned elsewhere, the
	// freelist would now be inconsistent and a fresh allocation
	// would surface as wazero "out of bounds memory access".
	for i := 0; i < 50; i++ {
		probe, err := NewSimpleColumn("probe", "x", intType, false, true)
		if err != nil {
			t.Fatalf("post-finalize NewSimpleColumn[%d] (allocator likely corrupted): %v", i, err)
		}
		_ = probe
	}
}

// TestLifetime_RuntimeConditionalOwnership_AddColumnTrue covers the
// runtime-conditional ownership bug end-to-end:
//
//   `SimpleTable::AddColumn(Column*, bool is_owned)` adopts the column
//   when `is_owned=true`. The wasmify-emitted Go wrapper must clear
//   `col.ptr` after the invoke in that branch so the column's per-
//   instance finalizer does not later issue a Free RPC against memory
//   the table's destructor already reclaims.
//
// Today (before the wasmify fix) the wrapper does NOT clear col.ptr,
// because `signatureMatches` in wasmify's gen-proto stage compares
// user-supplied unqualified type names against clang's fully-qualified
// qual_types byte-for-byte and fails to match — see
// issues/runtime-conditional-ownership.md for the full diagnosis.
//
// This test asserts the desired post-condition (col.ptr == 0). It also
// drives the wasm allocator past the point a stray finalizer would
// surface as "out of bounds memory access", giving a deterministic
// crash signal when the bug is present.
func TestLifetime_RuntimeConditionalOwnership_AddColumnTrue(t *testing.T) {
	ensureInit(t)
	t.Cleanup(gcLocal)

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

	if err := tbl.AddColumn2(col, true); err != nil {
		t.Fatalf("AddColumn2(col, true): %v", err)
	}

	// Phase 1 — deterministic ptr assertion.
	// After the C++ side has adopted col (is_owned=true), the Go
	// wrapper's ptr MUST be cleared. Otherwise the wrapper's per-
	// instance finalizer will free memory the table's destructor
	// will also reclaim → double-free against the wasm allocator.
	if col.ptr != 0 {
		t.Errorf("AddColumn2(col, is_owned=true): col.ptr must be 0 to prevent double-free; "+
			"got %d. Fix is upstream in wasmify (issues/runtime-conditional-ownership.md).",
			col.ptr)
	}

	// Phase 2 — drive a finalizer-then-allocator-probe sequence to
	// turn "double-free corrupts the freelist" from a latent into a
	// loud failure. If Phase 1 already cleared col.ptr, free() is a
	// no-op and the probe runs cleanly. If Phase 1 found col.ptr set,
	// the finalizer here issues a Free RPC against owned memory and
	// the probe traditionally crashes with wazero "out of bounds
	// memory access" or a corrupt allocator state.
	col = nil
	for i := 0; i < 10; i++ {
		runtime.GC()
	}

	for i := 0; i < 50; i++ {
		probe, err := NewSimpleColumn("probe", "x", intType, false, true)
		if err != nil {
			t.Fatalf("post-finalize NewSimpleColumn[%d] (allocator likely corrupted by stray free): %v", i, err)
		}
		_ = probe
	}

	runtime.KeepAlive(tbl)
}

// gcLocal forces two passes of GC to flush queued finalizers.
// Identical in behaviour to googlesql_test.gc but accessible from
// internal-package tests in this file.
func gcLocal() {
	runtime.GC()
	runtime.GC()
}

// TestLifetime_AnalyzerOutputKeepAlive verifies that an AnalyzerOutput
// returned from AnalyzeStatement remains valid after every input
// reference is dropped. The bridge pins options/catalog/typeFactory
// via keepAlive — if any of those finalisers fired prematurely the
// AnalyzerOutput's resolved tree would dangle.
func TestLifetime_AnalyzerOutputKeepAlive(t *testing.T) {
	ensureInit(t)

	cat, err := NewSimpleCatalog("c", nil)
	if err != nil {
		t.Fatalf("NewSimpleCatalog: %v", err)
	}
	langOpts, err := NewLanguageOptions()
	if err != nil {
		t.Fatalf("NewLanguageOptions: %v", err)
	}
	if err := langOpts.EnableMaximumLanguageFeaturesForDevelopment(); err != nil {
		t.Fatalf("EnableMaximumLanguageFeaturesForDevelopment: %v", err)
	}
	if err := langOpts.SetSupportsAllStatementKinds(); err != nil {
		t.Fatalf("SetSupportsAllStatementKinds: %v", err)
	}
	if err := cat.AddBuiltinFunctionsAndTypes(&BuiltinFunctionOptions{LanguageOptions: langOpts}); err != nil {
		t.Fatalf("AddBuiltinFunctionsAndTypes: %v", err)
	}
	opts, err := NewAnalyzerOptions2()
	if err != nil {
		t.Fatalf("NewAnalyzerOptions2: %v", err)
	}
	if err := opts.SetLanguage(langOpts); err != nil {
		t.Fatalf("SetLanguage: %v", err)
	}
	tf, err := NewTypeFactory()
	if err != nil {
		t.Fatalf("NewTypeFactory: %v", err)
	}

	out, err := AnalyzeStatement("SELECT 1 AS a", opts, cat, tf)
	if err != nil {
		t.Fatalf("AnalyzeStatement: %v", err)
	}
	if out == nil {
		t.Fatal("AnalyzeStatement returned nil out")
	}

	// Drop every input. keepAlive on out should keep them alive
	// transitively.
	cat = nil
	langOpts = nil
	opts = nil
	tf = nil
	for i := 0; i < 5; i++ {
		runtime.GC()
	}

	// Force a method call that reads through the resolved tree —
	// will SIGSEGV inside wazero if any input was finalised.
	stmt, err := out.ResolvedStatement()
	if err != nil {
		t.Fatalf("AnalyzerOutput.ResolvedStatement after inputs dropped: %v", err)
	}
	_ = stmt
	runtime.KeepAlive(out)
}
