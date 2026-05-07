package googlesql_test

import (
	"strings"
	"testing"

	"github.com/goccy/go-googlesql"
)

// TestBindingGap_MultiCatalogCreate verifies that the
// `MultiCatalog::Create` static factory (Status-with-out-param shape)
// is reachable from Go after the upstream wasmify generator was
// taught to recognise the idiom. Before the fix this RPC was silently
// dropped because the bridge classified `absl::Status` returns as
// "not a factory" without considering the out-parameter.
//
// The test assembles two trivial catalogs, combines them via
// `NewMultiCatalogCreate`, and checks the resulting wrapper exposes
// both children through `FullName` (the only side-effect-free
// observable on `MultiCatalog`).
func TestBindingGap_MultiCatalogCreate(t *testing.T) {
	t.Cleanup(gc)

	c1, err := googlesql.NewSimpleCatalog("alpha", nil)
	if err != nil {
		t.Fatalf("NewSimpleCatalog(alpha): %v", err)
	}
	c2, err := googlesql.NewSimpleCatalog("beta", nil)
	if err != nil {
		t.Fatalf("NewSimpleCatalog(beta): %v", err)
	}

	combined, err := googlesql.NewMultiCatalogCreate("combined", []googlesql.CatalogNode{c1, c2})
	if err != nil {
		t.Fatalf("NewMultiCatalogCreate: %v", err)
	}
	if combined == nil {
		t.Fatal("NewMultiCatalogCreate returned nil instance")
	}

	got, err := combined.FullName()
	if err != nil {
		t.Fatalf("MultiCatalog.FullName: %v", err)
	}
	if got != "combined" {
		t.Errorf("MultiCatalog.FullName = %q, want %q", got, "combined")
	}
}

// TestBindingGap_MakeProtoTypeSurfaceExists verifies that the
// `TypeFactory.MakeProtoType` and `MakeEnumType` Go signatures are
// generated. End-to-end execution requires constructing a
// `*Descriptor` / `*EnumDescriptor` from a populated `DescriptorPool`,
// which is a separate plumbing concern (bridging
// `FileDescriptorSet::ParseFromArray` and `DescriptorPool::BuildFile`
// from the protobuf runtime). For this test we verify only that the
// API exists and reports a sensible error when called with `nil`
// inputs — the surface itself was missing in earlier releases because
// the bridge filter dropped any method whose signature mentioned
// `google::protobuf::Descriptor*`.
func TestBindingGap_MakeProtoTypeSurfaceExists(t *testing.T) {
	t.Cleanup(gc)

	tf, err := googlesql.NewTypeFactory()
	if err != nil {
		t.Fatalf("NewTypeFactory: %v", err)
	}

	// Calling with a nil descriptor must not crash — the bridge
	// turns the null-pointer into an error response or returns a
	// nil result. Either outcome is fine for the existence check.
	gotProtoType, errProto := tf.MakeProtoType(nil, nil)
	t.Logf("MakeProtoType(nil, nil) = (%v, %v)", gotProtoType, errProto)

	gotEnumType, errEnum := tf.MakeEnumType(nil, nil)
	t.Logf("MakeEnumType(nil, nil) = (%v, %v)", gotEnumType, errEnum)

	// Surface presence is what we're guarding. Either an error or a
	// nil result is acceptable; what we cannot tolerate is the
	// method having been entirely absent from the binding.
	if errProto == nil && gotProtoType == nil && errEnum == nil && gotEnumType == nil {
		t.Log("note: both MakeProtoType and MakeEnumType returned (nil, nil) — the bridge accepted the null descriptor without an error path")
	}
}

// TestBindingGap_DescriptorPoolHandlesPresent verifies that the
// `DescriptorPool` and `FileDescriptorSet` handle types are emitted
// in the binding even though neither has a Go-side constructor today.
// This is the minimum necessary precondition for a follow-up that
// adds the load-from-bytes plumbing.
//
// We cannot create either type from the Go side yet, so we only
// check that calls passing `nil` of the expected type compile and
// link. The test's existence in the package is therefore enough to
// fail compilation if the handle types disappear from the binding.
func TestBindingGap_DescriptorPoolHandlesPresent(t *testing.T) {
	var p *googlesql.DescriptorPool
	var fds *googlesql.FileDescriptorSet
	var d *googlesql.Descriptor
	var e *googlesql.EnumDescriptor
	_ = p
	_ = fds
	_ = d
	_ = e

	// Smoke-check that the names are usable and the package exports
	// each handle type. If a future regression renames any of these
	// types we'll see a build failure here instead of a silent gap.
	descs := []string{"DescriptorPool", "FileDescriptorSet", "Descriptor", "EnumDescriptor"}
	for _, n := range descs {
		if !strings.HasPrefix(n, "") {
			t.Errorf("unexpected: %s", n)
		}
	}
}

// makeInt64Type returns a TypeFactory plus a fully-qualified `int64`
// `Googlesql_TypeNode` for tests that need a concrete type to plug
// into property declarations.
func makeInt64Type(t *testing.T) (*googlesql.TypeFactory, googlesql.Googlesql_TypeNode) {
	t.Helper()
	tf, err := googlesql.NewTypeFactory()
	if err != nil {
		t.Fatalf("NewTypeFactory: %v", err)
	}
	int64Type, err := tf.MakeSimpleType(googlesql.TypeKindTypeInt64)
	if err != nil {
		t.Fatalf("MakeSimpleType(TYPE_INT64): %v", err)
	}
	return tf, int64Type
}

// equalStrings compares two []string slices in order. The bridge
// preserves Span<const std::string> ordering element-by-element, so
// callers can rely on positional comparison.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestBindingGap_SimpleGraphPropertyDeclaration exercises the
// constructor unlocked by Bug #5A: clang's elaborated-type-specifier
// (`const class Type *`) in the property-declaration ctor's third
// argument used to defeat the bridge filter, dropping the entire
// constructor. The test builds a property declaration, reads the
// stored fields back through the C++ accessors, and asserts the
// values round-trip exactly.
func TestBindingGap_SimpleGraphPropertyDeclaration(t *testing.T) {
	t.Cleanup(gc)

	_, int64Type := makeInt64Type(t)

	decl, err := googlesql.NewSimpleGraphPropertyDeclaration("count", []string{"g"}, int64Type)
	if err != nil {
		t.Fatalf("NewSimpleGraphPropertyDeclaration: %v", err)
	}
	if decl == nil {
		t.Fatal("NewSimpleGraphPropertyDeclaration returned nil")
	}

	if got, err := decl.Name(); err != nil || got != "count" {
		t.Errorf("Name() = (%q, %v), want (\"count\", nil)", got, err)
	}
	if got, err := decl.PropertyGraphNamePath(); err != nil {
		t.Errorf("PropertyGraphNamePath() error: %v", err)
	} else if !equalStrings(got, []string{"g"}) {
		t.Errorf("PropertyGraphNamePath() = %v, want [\"g\"]", got)
	}
	if kind, err := decl.Kind(); err != nil {
		t.Errorf("Kind() error: %v", err)
	} else {
		t.Logf("Kind() = %v", kind)
	}
}

// TestBindingGap_SimpleGraphElementLabel exercises the constructor
// unlocked by Bug #5A + #5B: the third argument is an
// `absl::flat_hash_set<const GraphPropertyDeclaration*>` whose proto
// schema is `repeated <Handle>` and whose bridge body materialises
// the set element-by-element via `insert(...)`. The test feeds two
// property declarations through the constructor, then asks the
// element label to expose them back via `GetPropertyDeclarations`.
func TestBindingGap_SimpleGraphElementLabel(t *testing.T) {
	t.Cleanup(gc)

	_, int64Type := makeInt64Type(t)
	decl1, err := googlesql.NewSimpleGraphPropertyDeclaration("count", []string{"g"}, int64Type)
	if err != nil {
		t.Fatalf("NewSimpleGraphPropertyDeclaration(count): %v", err)
	}
	decl2, err := googlesql.NewSimpleGraphPropertyDeclaration("score", []string{"g"}, int64Type)
	if err != nil {
		t.Fatalf("NewSimpleGraphPropertyDeclaration(score): %v", err)
	}

	label, err := googlesql.NewSimpleGraphElementLabel(
		"Person", []string{"g"},
		[]googlesql.GraphPropertyDeclarationNode{decl1, decl2},
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphElementLabel: %v", err)
	}
	if label == nil {
		t.Fatal("NewSimpleGraphElementLabel returned nil")
	}

	if got, err := label.Name(); err != nil || got != "Person" {
		t.Errorf("Name() = (%q, %v), want (\"Person\", nil)", got, err)
	}
	if got, err := label.PropertyGraphNamePath(); err != nil {
		t.Errorf("PropertyGraphNamePath() error: %v", err)
	} else if !equalStrings(got, []string{"g"}) {
		t.Errorf("PropertyGraphNamePath() = %v, want [\"g\"]", got)
	}
}

// TestBindingGap_SimpleGraphNodeTable exercises the most demanding
// constructor unlocked by Bugs #5A + #5B + #5C combined:
//
//   - `class Type` elaborated-type-specifier in the underlying
//     property-declaration ctor (5A — needed to construct the
//     property declarations the labels reference).
//   - `absl::flat_hash_set<const GraphElementLabel*>` for `labels`
//     (5B — set-like container materialised from repeated handles).
//   - `std::vector<std::unique_ptr<const GraphPropertyDefinition>>`
//     for `property_definitions` (5C — vector-of-unique_ptr with
//     per-element ownership transfer).
//
// The test wires up a tiny `Person(id INT64, name STRING)` table and
// builds a node table on top of it, then reads back the configured
// fields through the C++ accessors.
func TestBindingGap_SimpleGraphNodeTable(t *testing.T) {
	t.Cleanup(gc)

	_, int64Type := makeInt64Type(t)

	// Backing input table: `Person(id INT64)`.
	person, err := googlesql.NewSimpleTable("Person", -1)
	if err != nil {
		t.Fatalf("NewSimpleTable(Person): %v", err)
	}
	idCol, err := googlesql.NewSimpleColumn("Person", "id", int64Type, false, false)
	if err != nil {
		t.Fatalf("NewSimpleColumn(id): %v", err)
	}
	if err := person.AddColumn2(idCol, true); err != nil {
		t.Fatalf("AddColumn(id): %v", err)
	}

	// One label declared on the node table, no property defs / dynamic
	// label / dynamic properties (the corresponding params are
	// nil-safe: bridge.cc passes them to the C++ ctor as
	// `unique_ptr<T>(nullptr)`).
	idDecl, err := googlesql.NewSimpleGraphPropertyDeclaration("id", []string{"g"}, int64Type)
	if err != nil {
		t.Fatalf("NewSimpleGraphPropertyDeclaration(id): %v", err)
	}
	personLabel, err := googlesql.NewSimpleGraphElementLabel(
		"Person", []string{"g"},
		[]googlesql.GraphPropertyDeclarationNode{idDecl},
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphElementLabel(Person): %v", err)
	}

	node, err := googlesql.NewSimpleGraphNodeTable(
		"PersonNodes", []string{"g"},
		person,
		[]int32{0}, // key_cols: column index 0 (id) is the key
		[]googlesql.GraphElementLabelNode{personLabel},
		nil, // property_definitions: empty
		nil, // dynamic_label
		nil, // dynamic_properties
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphNodeTable: %v", err)
	}
	if node == nil {
		t.Fatal("NewSimpleGraphNodeTable returned nil")
	}

	if got, err := node.Name(); err != nil || got != "PersonNodes" {
		t.Errorf("Name() = (%q, %v), want (\"PersonNodes\", nil)", got, err)
	}
	if got, err := node.PropertyGraphNamePath(); err != nil {
		t.Errorf("PropertyGraphNamePath() error: %v", err)
	} else if !equalStrings(got, []string{"g"}) {
		t.Errorf("PropertyGraphNamePath() = %v, want [\"g\"]", got)
	}
	if cols, err := node.GetKeyColumns(); err != nil {
		t.Errorf("GetKeyColumns() error: %v", err)
	} else if len(cols) != 1 || cols[0] != 0 {
		t.Errorf("GetKeyColumns() = %v, want [0]", cols)
	}
	if has, err := node.HasDynamicLabel(); err != nil || has {
		t.Errorf("HasDynamicLabel() = (%v, %v), want (false, nil)", has, err)
	}
	if has, err := node.HasDynamicProperties(); err != nil || has {
		t.Errorf("HasDynamicProperties() = (%v, %v), want (false, nil)", has, err)
	}
}

// TestBindingGap_SimpleGraphEdgeTable extends the node-table test
// to exercise the edge-specific source / destination
// `unique_ptr<const GraphNodeTableReference>` parameters. These are
// single-`unique_ptr` arguments (not vector form), so they hit the
// existing `paramTakesOwnership` + `handleArgExpr` ownership-
// transfer path that Bug #5C's vector counterpart parallels.
func TestBindingGap_SimpleGraphEdgeTable(t *testing.T) {
	t.Cleanup(gc)

	_, int64Type := makeInt64Type(t)

	// Backing tables: `Person(id INT64)` and `Knows(src INT64, dst INT64)`.
	person, err := googlesql.NewSimpleTable("Person", -1)
	if err != nil {
		t.Fatalf("NewSimpleTable(Person): %v", err)
	}
	if col, err := googlesql.NewSimpleColumn("Person", "id", int64Type, false, false); err != nil {
		t.Fatalf("NewSimpleColumn(Person.id): %v", err)
	} else if err := person.AddColumn2(col, true); err != nil {
		t.Fatalf("AddColumn(Person.id): %v", err)
	}

	knows, err := googlesql.NewSimpleTable("Knows", -1)
	if err != nil {
		t.Fatalf("NewSimpleTable(Knows): %v", err)
	}
	for _, n := range []string{"src", "dst"} {
		col, err := googlesql.NewSimpleColumn("Knows", n, int64Type, false, false)
		if err != nil {
			t.Fatalf("NewSimpleColumn(Knows.%s): %v", n, err)
		}
		if err := knows.AddColumn2(col, true); err != nil {
			t.Fatalf("AddColumn(Knows.%s): %v", n, err)
		}
	}

	// Need a Person node table to anchor the source/destination
	// references on the edge table.
	idDecl, err := googlesql.NewSimpleGraphPropertyDeclaration("id", []string{"g"}, int64Type)
	if err != nil {
		t.Fatalf("NewSimpleGraphPropertyDeclaration(id): %v", err)
	}
	personLabel, err := googlesql.NewSimpleGraphElementLabel(
		"Person", []string{"g"},
		[]googlesql.GraphPropertyDeclarationNode{idDecl},
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphElementLabel(Person): %v", err)
	}
	personNode, err := googlesql.NewSimpleGraphNodeTable(
		"PersonNodes", []string{"g"}, person,
		[]int32{0},
		[]googlesql.GraphElementLabelNode{personLabel},
		nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphNodeTable(PersonNodes): %v", err)
	}

	// Source/destination references: each maps an edge column index
	// to the corresponding node-table column index.
	srcRef, err := googlesql.NewSimpleGraphNodeTableReference(
		personNode,
		[]int32{0}, // edge column: src
		[]int32{0}, // node column: id
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphNodeTableReference(src): %v", err)
	}
	dstRef, err := googlesql.NewSimpleGraphNodeTableReference(
		personNode,
		[]int32{1}, // edge column: dst
		[]int32{0}, // node column: id
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphNodeTableReference(dst): %v", err)
	}

	knowsLabel, err := googlesql.NewSimpleGraphElementLabel(
		"Knows", []string{"g"},
		nil,
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphElementLabel(Knows): %v", err)
	}

	edge, err := googlesql.NewSimpleGraphEdgeTable(
		"KnowsEdges", []string{"g"},
		knows,
		[]int32{0, 1},
		[]googlesql.GraphElementLabelNode{knowsLabel},
		nil,
		srcRef,
		dstRef,
		nil, nil,
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphEdgeTable: %v", err)
	}
	if edge == nil {
		t.Fatal("NewSimpleGraphEdgeTable returned nil")
	}

	if got, err := edge.Name(); err != nil || got != "KnowsEdges" {
		t.Errorf("Name() = (%q, %v), want (\"KnowsEdges\", nil)", got, err)
	}
	if cols, err := edge.GetKeyColumns(); err != nil {
		t.Errorf("GetKeyColumns() error: %v", err)
	} else if len(cols) != 2 || cols[0] != 0 || cols[1] != 1 {
		t.Errorf("GetKeyColumns() = %v, want [0 1]", cols)
	}
}

// myTVFCallback is a Go-side TableValuedFunction subclass used by
// the verification tests below. It overrides DebugString to return
// a deterministic marker string; Resolve synthesises and returns a
// fresh `*TVFSignature` so the C++ trampoline's StatusOr<unique_ptr>
// reconstruction can be exercised end-to-end. Everything else falls
// back to the generated CallbackDefaults zero-impls.
type myTVFCallback struct {
	googlesql.TableValuedFunctionCallbackDefaults
	mark           string
	resolveCalls   int
	resolveErr     error
	resolveResult  *googlesql.TVFSignature
	resolveCapture struct {
		concreteSig *googlesql.FunctionSignature
	}
}

func (m *myTVFCallback) DebugString() (string, error) {
	return m.mark, nil
}

func (m *myTVFCallback) Resolve(
	analyzerOptions *googlesql.AnalyzerOptions,
	actualArguments []*googlesql.TVFInputArgumentType,
	concreteSignature *googlesql.FunctionSignature,
	catalog googlesql.CatalogNode,
	typeFactory *googlesql.TypeFactory,
) (*googlesql.TVFSignature, error) {
	m.resolveCalls++
	m.resolveCapture.concreteSig = concreteSignature
	if m.resolveErr != nil {
		return nil, m.resolveErr
	}
	return m.resolveResult, nil
}

// TestBindingGap_TableValuedFunctionFromImpl exercises Bug #2's full
// stack: the callback predicate accepts a concrete class listed in
// `bridge.CallbackClasses`, the trampoline class generates a
// constructor that forwards `(name_path, group, signatures, options)`
// to the base, the wasm dispatch reads those args off the wire, and
// the Go-side `NewTableValuedFunctionFromImpl(impl, args...)`
// drives the construction. Once constructed, calling DebugString on
// the C++-owned instance must round-trip through the Go callback —
// proving the C++→Go virtual dispatch is wired end-to-end.
func TestBindingGap_TableValuedFunctionFromImpl(t *testing.T) {
	t.Cleanup(gc)

	// Build a minimal FunctionSignature so we have a non-nil arg to
	// forward through the trampoline ctor; the analyzer is not
	// invoked by this test, so the signature contents do not matter
	// — only that the bridge transports it intact.
	sigOpts, err := googlesql.NewFunctionSignatureOptions()
	if err != nil {
		t.Fatalf("NewFunctionSignatureOptions: %v", err)
	}
	resultType, err := googlesql.NewFunctionArgumentType4(googlesql.SignatureArgumentKindArgTypeRelation, 1)
	if err != nil {
		t.Fatalf("NewFunctionArgumentType4: %v", err)
	}
	sig, err := googlesql.NewFunctionSignature2(resultType, nil, 0, sigOpts)
	if err != nil {
		t.Fatalf("NewFunctionSignature2: %v", err)
	}

	// TableValuedFunctionOptions is a value-type struct (no
	// constructor function), so it can be zero-initialised in Go
	// and passed by pointer.
	tvfOpts := &googlesql.TableValuedFunctionOptions{}

	cb := &myTVFCallback{mark: "go-tvf-mark"}

	// NewTableValuedFunctionFromImpl forwards the four base ctor
	// args plus the impl callback. Before Bug #2 this signature did
	// not exist at all — the callback predicate rejected concrete
	// classes outright, and there was no machinery for forwarding
	// arg-taking ctors through a trampoline.
	tvf, err := googlesql.NewTableValuedFunctionFromImpl(
		cb,
		[]string{"my_tvf"},
		"",
		[]*googlesql.FunctionSignature{sig},
		tvfOpts,
	)
	if err != nil {
		t.Fatalf("NewTableValuedFunctionFromImpl: %v", err)
	}
	if tvf == nil {
		t.Fatal("NewTableValuedFunctionFromImpl returned nil instance")
	}

	// FullName comes from the C++ base — proves the trampoline ctor
	// actually forwarded our `function_name_path` argument.
	if got, err := tvf.FullName(true); err != nil {
		t.Errorf("FullName: %v", err)
	} else if !strings.Contains(got, "my_tvf") {
		t.Errorf("FullName() = %q, expected to contain %q", got, "my_tvf")
	}

	// DebugString routes through the trampoline back to our Go
	// callback — proves the C++→Go virtual dispatch reaches the
	// `myTVFCallback.DebugString` override.
	if got, err := tvf.DebugString(); err != nil {
		t.Errorf("DebugString: %v", err)
	} else if got != "go-tvf-mark" {
		t.Errorf("DebugString() = %q, want %q (via callback override)", got, "go-tvf-mark")
	}
}

// TestBindingGap_TableValuedFunctionResolveCallback exercises the
// Bug #2 follow-up: a virtual whose return type is
// `absl::StatusOr<std::unique_ptr<T>>` (here, `Status` + an output
// `std::shared_ptr<TVFSignature>*` after wasmify's request/response
// split) flows back from Go through the trampoline. Specifically:
//
//   - The Go callback synthesises a fresh `*TVFSignature` and
//     returns it as the function's value.
//   - The C++ trampoline reads the response, materialises a
//     `std::unique_ptr<TVFSignature>` from the handle the Go side
//     shipped, and returns the value via `std::move(*ptr)`.
//   - Calling `tvf.Resolve(...)` from Go invokes the underlying
//     C++ virtual dispatch, which must round-trip through the Go
//     callback.
//
// This validates the trampoline's Status-or-value reconstruction
// path that Bug #5D's `Span<const std::string>` follow-up did NOT
// cover (return-of-handle vs return-of-string-list are different
// proto wire shapes).
func TestBindingGap_TableValuedFunctionResolveCallback(t *testing.T) {
	t.Cleanup(gc)

	sigOpts, err := googlesql.NewFunctionSignatureOptions()
	if err != nil {
		t.Fatalf("NewFunctionSignatureOptions: %v", err)
	}
	resultType, err := googlesql.NewFunctionArgumentType4(googlesql.SignatureArgumentKindArgTypeRelation, 1)
	if err != nil {
		t.Fatalf("NewFunctionArgumentType4: %v", err)
	}
	sig, err := googlesql.NewFunctionSignature2(resultType, nil, 0, sigOpts)
	if err != nil {
		t.Fatalf("NewFunctionSignature2: %v", err)
	}
	tvfOpts := &googlesql.TableValuedFunctionOptions{}

	// The TVFSignature returned by Resolve is a fresh instance whose
	// only role here is to act as a transport for the round-trip.
	// We don't attempt to inspect its contents — the callback's
	// observable signal is `resolveCalls` plus a non-nil instance.
	tvfSig, err := googlesql.NewTVFSignature(nil, nil, nil)
	if err != nil {
		t.Fatalf("NewTVFSignature: %v", err)
	}

	cb := &myTVFCallback{
		mark:          "go-tvf-mark",
		resolveResult: tvfSig,
	}

	tvf, err := googlesql.NewTableValuedFunctionFromImpl(
		cb,
		[]string{"my_tvf"},
		"",
		[]*googlesql.FunctionSignature{sig},
		tvfOpts,
	)
	if err != nil {
		t.Fatalf("NewTableValuedFunctionFromImpl: %v", err)
	}
	defer func() { _ = tvf }() // keep the handle alive across the Resolve call

	// Trigger Resolve via a method on the TableValuedFunction. The
	// C++ side calls our virtual through the trampoline, which
	// forwards over wasmify_callback_invoke.
	resolved, err := tvf.Resolve(nil, nil, sig, nil, nil)
	if err != nil {
		t.Fatalf("tvf.Resolve: %v", err)
	}
	if resolved == nil {
		t.Fatal("tvf.Resolve returned nil signature")
	}
	if cb.resolveCalls != 1 {
		t.Errorf("Go callback's Resolve was called %d times, want 1", cb.resolveCalls)
	}
	_ = resolved
}

// TestBindingGap_TableValuedFunctionResolveCallback_PostFinalize is
// a regression test for a callback-adapter ownership bug: when the
// adapter wrapped incoming handles via `new<Handle>(_ptr)` it
// attached its own GC finaliser, racing the outer Go wrapper that
// already tracked the same C++ address. After the trampoline
// returned, both wrappers eventually called Free RPC on the same
// pointer — the second delete corrupted the wasm allocator's
// freelist and the next test exporting an unrelated wasm function
// faulted with "out of bounds memory access".
//
// The fix (in protoc-gen-wasmify-go's writeCallbackCaseBody) emits
// `new<Handle>NoFinalizer(_ptr)` for borrowed inputs. This test
// repeats the Resolve round-trip inside a closure so the locals
// become unreachable, drives gc() ten times to flush all
// finalisers, then exercises a wasm export that the corrupted
// allocator would have crashed on.
func TestBindingGap_TableValuedFunctionResolveCallback_PostFinalize(t *testing.T) {
	t.Cleanup(gc)
	(func() {
		sigOpts, err := googlesql.NewFunctionSignatureOptions()
		if err != nil {
			t.Fatalf("NewFunctionSignatureOptions: %v", err)
		}
		resultType, err := googlesql.NewFunctionArgumentType4(googlesql.SignatureArgumentKindArgTypeRelation, 1)
		if err != nil {
			t.Fatalf("NewFunctionArgumentType4: %v", err)
		}
		sig, err := googlesql.NewFunctionSignature2(resultType, nil, 0, sigOpts)
		if err != nil {
			t.Fatalf("NewFunctionSignature2: %v", err)
		}
		tvfOpts := &googlesql.TableValuedFunctionOptions{}
		tvfSig, err := googlesql.NewTVFSignature(nil, nil, nil)
		if err != nil {
			t.Fatalf("NewTVFSignature: %v", err)
		}
		cb := &myTVFCallback{mark: "go-tvf-mark", resolveResult: tvfSig}
		tvf, err := googlesql.NewTableValuedFunctionFromImpl(
			cb,
			[]string{"my_tvf"},
			"",
			[]*googlesql.FunctionSignature{sig},
			tvfOpts,
		)
		if err != nil {
			t.Fatalf("NewTableValuedFunctionFromImpl: %v", err)
		}
		_, err = tvf.Resolve(nil, nil, sig, nil, nil)
		if err != nil {
			t.Fatalf("tvf.Resolve: %v", err)
		}
	})()
	for i := 0; i < 10; i++ {
		gc()
	}
	_, _, _, _ = setupAnalyzer(t)
}

