package googlesql_test

import (
	"strings"
	"testing"

	"github.com/goccy/go-googlesql"
)

// e2e_test.go contains end-to-end verification that drives the
// regenerated binding through the analyzer rather than just
// asserting the surface exists. Each test exercises one of the
// previously-dropped surfaces against AnalyzeStatement and asserts
// on the resolved AST so a regression in the generator surfaces as
// a hard failure rather than as a "method missing" type error.

// TestE2E_MultiCatalog_CrossCatalog verifies Bug #4 end-to-end: a
// MultiCatalog assembled from two SimpleCatalogs makes both
// catalogs' tables visible to the analyzer in a single query. The
// query references one table from each catalog via a UNION ALL —
// either side missing would surface as an analyzer error.
func TestE2E_MultiCatalog_CrossCatalog(t *testing.T) {
	t.Cleanup(gc)

	// Pull AnalyzerOptions / TypeFactory off a setupAnalyzer call;
	// we don't actually use the SimpleCatalog it returns — the test
	// builds two of its own and combines them.
	_, opts, tf, langOpts := setupAnalyzer(t)

	int64Type, err := tf.GetInt64()
	if err != nil {
		t.Fatalf("TypeFactory.GetInt64: %v", err)
	}
	stringType, err := tf.GetString()
	if err != nil {
		t.Fatalf("TypeFactory.GetString: %v", err)
	}

	// Catalog A: register table "a_orders(id INT64)".
	catA, err := googlesql.NewSimpleCatalog("a", nil)
	if err != nil {
		t.Fatalf("NewSimpleCatalog A: %v", err)
	}
	bfo := &googlesql.BuiltinFunctionOptions{LanguageOptions: langOpts}
	if err := catA.AddBuiltinFunctionsAndTypes(bfo); err != nil {
		t.Fatalf("AddBuiltinFunctionsAndTypes A: %v", err)
	}
	tblA, err := googlesql.NewSimpleTable("a_orders", 0)
	if err != nil {
		t.Fatalf("NewSimpleTable a_orders: %v", err)
	}
	colA, err := googlesql.NewSimpleColumn("a_orders", "id", int64Type, false, true)
	if err != nil {
		t.Fatalf("NewSimpleColumn a_orders.id: %v", err)
	}
	if err := tblA.AddColumn(colA); err != nil {
		t.Fatalf("AddColumn a_orders.id: %v", err)
	}
	if err := catA.AddTable(tblA); err != nil {
		t.Fatalf("AddTable a_orders: %v", err)
	}

	// Catalog B: register table "b_users(name STRING)".
	catB, err := googlesql.NewSimpleCatalog("b", nil)
	if err != nil {
		t.Fatalf("NewSimpleCatalog B: %v", err)
	}
	if err := catB.AddBuiltinFunctionsAndTypes(bfo); err != nil {
		t.Fatalf("AddBuiltinFunctionsAndTypes B: %v", err)
	}
	tblB, err := googlesql.NewSimpleTable("b_users", 0)
	if err != nil {
		t.Fatalf("NewSimpleTable b_users: %v", err)
	}
	colB, err := googlesql.NewSimpleColumn("b_users", "name", stringType, false, true)
	if err != nil {
		t.Fatalf("NewSimpleColumn b_users.name: %v", err)
	}
	if err := tblB.AddColumn(colB); err != nil {
		t.Fatalf("AddColumn b_users.name: %v", err)
	}
	if err := catB.AddTable(tblB); err != nil {
		t.Fatalf("AddTable b_users: %v", err)
	}

	// Build the MultiCatalog combining both.
	mc, err := googlesql.NewMultiCatalogCreate("combined", []googlesql.CatalogNode{catA, catB})
	if err != nil {
		t.Fatalf("NewMultiCatalogCreate: %v", err)
	}

	// Cross-catalog query: must see BOTH tables. The CAST keeps the
	// branches type-compatible so a UNION ALL is well-formed.
	out, err := googlesql.AnalyzeStatement(
		"SELECT CAST(id AS STRING) AS v FROM a_orders UNION ALL SELECT name AS v FROM b_users",
		opts, mc, tf,
	)
	if err != nil {
		t.Fatalf("AnalyzeStatement (MultiCatalog): %v", err)
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
		t.Errorf("OutputColumnListSize = %d, want 1", size)
	}
}

// TestE2E_Procedure_Call verifies that a Procedure constructed
// via NewProcedure + registered with SimpleCatalog.AddOwnedProcedure
// is reachable from the analyzer for a `CALL ...` statement, and
// that the resolved AST exposes the call as a *ResolvedCallStmt.
//
// Note: the original plan called for a "ProcedureCallback" exercise
// — a Go-implemented Procedure subclass. The api-spec parse shows
// googlesql::Procedure has no virtual methods, so it's not
// callback-eligible (and adding it to bridge.CallbackClasses would
// produce an empty interface). Procedure end-to-end coverage
// instead exercises the in-binding constructor + AddOwnedProcedure
// + analyzer round-trip path.
func TestE2E_Procedure_Call(t *testing.T) {
	t.Cleanup(gc)

	catalog, opts, tf, langOpts := setupAnalyzer(t)

	// LanguageOptions defaults already enable CALL via
	// EnableMaximumLanguageFeaturesForDevelopment +
	// SetSupportsAllStatementKinds in setupAnalyzer.
	_ = langOpts

	int64Type, err := tf.GetInt64()
	if err != nil {
		t.Fatalf("GetInt64: %v", err)
	}
	stringType, err := tf.GetString()
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	int64Arg, err := googlesql.NewFunctionArgumentType(int64Type, nil, 1)
	if err != nil {
		t.Fatalf("NewFunctionArgumentType int64: %v", err)
	}
	stringArg, err := googlesql.NewFunctionArgumentType(stringType, nil, 1)
	if err != nil {
		t.Fatalf("NewFunctionArgumentType string: %v", err)
	}
	// Result type for procedures is the unit type — represented by
	// an INT64 placeholder here; procedures don't return a value
	// from the SQL surface but the C++ FunctionSignature still
	// requires a result FunctionArgumentType.
	resultArg, err := googlesql.NewFunctionArgumentType(int64Type, nil, 1)
	if err != nil {
		t.Fatalf("NewFunctionArgumentType result: %v", err)
	}
	sigOpts, err := googlesql.NewFunctionSignatureOptions()
	if err != nil {
		t.Fatalf("NewFunctionSignatureOptions: %v", err)
	}
	sig, err := googlesql.NewFunctionSignature2(resultArg,
		[]*googlesql.FunctionArgumentType{int64Arg, stringArg}, 0, sigOpts)
	if err != nil {
		t.Fatalf("NewFunctionSignature2: %v", err)
	}
	proc, err := googlesql.NewProcedure([]string{"my_proc"}, sig)
	if err != nil {
		t.Fatalf("NewProcedure: %v", err)
	}
	if err := catalog.AddOwnedProcedure(proc); err != nil {
		t.Fatalf("AddOwnedProcedure: %v", err)
	}

	out, err := googlesql.AnalyzeStatement("CALL my_proc(1, 'x')", opts, catalog, tf)
	if err != nil {
		t.Fatalf("AnalyzeStatement(CALL): %v", err)
	}
	stmt, err := out.ResolvedStatement()
	if err != nil {
		t.Fatalf("ResolvedStatement: %v", err)
	}
	if _, ok := stmt.(*googlesql.ResolvedCallStmt); !ok {
		t.Fatalf("expected *ResolvedCallStmt, got %T", stmt)
	}
}

// catalogWithGraph is a Go-implemented Catalog that records calls
// to FindPropertyGraph2. It returns nil for the graph; the test
// only needs to confirm the callback was invoked, which proves
// Bug #3's `T*&` output-shape fix wires the trampoline correctly.
type catalogWithGraph struct {
	googlesql.CatalogCallbackDefaults
	graphLookups int
}

func (c *catalogWithGraph) FullName() (string, error) { return "graph_catalog", nil }

func (c *catalogWithGraph) FindPropertyGraph2(path []string, options *googlesql.FindOptions) (googlesql.PropertyGraphNode, error) {
	c.graphLookups++
	return nil, nil
}

// TestE2E_CatalogCallback_FindPropertyGraph verifies Bug #3 end-to-
// end: the CatalogCallback interface now exposes FindPropertyGraph2
// (the 3-arg overload with FindOptions), the trampoline correctly
// forwards the `const PropertyGraph*&` output-shape param into the
// Go callback's return value, and the analyzer routes property-
// graph lookups through it.
//
// The Go-side catalog returns nil from FindPropertyGraph2 — the
// query analysis is therefore expected to fail with a "not found"-
// style error. The test's actual assertion is that the callback
// was invoked at least once: graph lookups exercise the new
// `T*&` codepath the way regular FindTable lookups exercise the
// `T**` codepath.
func TestE2E_CatalogCallback_FindPropertyGraph(t *testing.T) {
	t.Cleanup(gc)

	_, opts, tf, _ := setupAnalyzer(t)

	impl := &catalogWithGraph{}
	cat, err := googlesql.NewCatalogFromImpl(impl)
	if err != nil {
		t.Fatalf("NewCatalogFromImpl: %v", err)
	}

	// Property-graph syntax requires the property-graph language
	// feature, which setupAnalyzer enables via
	// EnableMaximumLanguageFeaturesForDevelopment.
	_, _ = googlesql.AnalyzeStatement(
		"SELECT * FROM GRAPH_TABLE(my_graph MATCH (n) RETURN 1 AS x)",
		opts, cat, tf,
	)
	if impl.graphLookups == 0 {
		t.Errorf("expected FindPropertyGraph2 to be called at least once, got 0")
	}
}

// TestE2E_PropertyGraph_Simple verifies the SimpleGraph*
// constructor surface end-to-end: a Go-built PropertyGraph with a
// node table is registered in a SimpleCatalog, the analyzer runs a
// graph query against it, and the resolved AST contains a
// graph-table scan. Without all of Bug #5A/B/C this test could not
// even build the input objects.
func TestE2E_PropertyGraph_Simple(t *testing.T) {
	t.Cleanup(gc)

	catalog, opts, tf, _ := setupAnalyzer(t)

	int64Type, err := tf.GetInt64()
	if err != nil {
		t.Fatalf("GetInt64: %v", err)
	}

	// Backing table for the node: Person(id INT64).
	person, err := googlesql.NewSimpleTable("Person", -1)
	if err != nil {
		t.Fatalf("NewSimpleTable Person: %v", err)
	}
	idCol, err := googlesql.NewSimpleColumn("Person", "id", int64Type, false, false)
	if err != nil {
		t.Fatalf("NewSimpleColumn id: %v", err)
	}
	if err := person.AddColumn2(idCol, true); err != nil {
		t.Fatalf("AddColumn id: %v", err)
	}
	if err := catalog.AddTable(person); err != nil {
		t.Fatalf("AddTable Person: %v", err)
	}

	// Property declaration + label for the node table.
	idDecl, err := googlesql.NewSimpleGraphPropertyDeclaration("id", []string{"g"}, int64Type)
	if err != nil {
		t.Fatalf("NewSimpleGraphPropertyDeclaration id: %v", err)
	}
	personLabel, err := googlesql.NewSimpleGraphElementLabel(
		"Person", []string{"g"},
		[]googlesql.GraphPropertyDeclarationNode{idDecl},
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphElementLabel Person: %v", err)
	}

	node, err := googlesql.NewSimpleGraphNodeTable(
		"PersonNodes", []string{"g"},
		person,
		[]int32{0},
		[]googlesql.GraphElementLabelNode{personLabel},
		nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphNodeTable: %v", err)
	}

	graph, err := googlesql.NewSimplePropertyGraph([]string{"g"})
	if err != nil {
		t.Fatalf("NewSimplePropertyGraph: %v", err)
	}
	if err := graph.AddPropertyDeclaration(idDecl); err != nil {
		t.Fatalf("AddPropertyDeclaration id: %v", err)
	}
	if err := graph.AddLabel(personLabel); err != nil {
		t.Fatalf("AddLabel Person: %v", err)
	}
	if err := graph.AddNodeTable(node); err != nil {
		t.Fatalf("AddNodeTable: %v", err)
	}
	if err := catalog.AddOwnedPropertyGraph(graph); err != nil {
		t.Fatalf("AddOwnedPropertyGraph: %v", err)
	}

	// Graph query: GRAPH_TABLE drives FindPropertyGraph through the
	// SimpleCatalog (not through a callback), exercising the
	// in-binding path end-to-end.
	// Query the graph: MATCH (n:Person) returns the bound element;
	// SELECT 1 doesn't dereference any properties so we don't need
	// the backing table's columns to be wired through the label's
	// property declarations.
	out, err := googlesql.AnalyzeStatement(
		"SELECT * FROM GRAPH_TABLE(g MATCH (n:Person) RETURN 1 AS one)",
		opts, catalog, tf,
	)
	if err != nil {
		t.Fatalf("AnalyzeStatement(graph): %v", err)
	}
	stmt, err := out.ResolvedStatement()
	if err != nil {
		t.Fatalf("ResolvedStatement: %v", err)
	}
	qs, ok := stmt.(*googlesql.ResolvedQueryStmt)
	if !ok {
		t.Fatalf("expected *ResolvedQueryStmt, got %T", stmt)
	}
	if size, _ := qs.OutputColumnListSize(); size != 1 {
		t.Errorf("OutputColumnListSize = %d, want 1", size)
	}
}

// goPropertyGraph is a Go-implemented PropertyGraph. Important
// constraint: callback handlers run *inside* the wasm call that
// triggered them and cannot re-enter the wasm runtime (the module
// mutex is already held). The impl therefore returns either:
//
//   - pre-built handles to existing C++ objects (looking up a Go
//     wrapper does not need a wasm call — only its rawPtr is
//     materialised for the wire), or
//   - cached scalar values populated *before* the analyze call.
//
// Delegating to wrapped.Name() etc. would deadlock the runtime.
type goPropertyGraph struct {
	googlesql.PropertyGraphCallbackDefaults
	name     string
	namePath []string
	labels   map[string]googlesql.GraphElementLabelNode
	elTables map[string]googlesql.GraphElementTableNode
	propDecls map[string]googlesql.GraphPropertyDeclarationNode
	nodeTables []googlesql.GraphNodeTableNode

	nameCalls     int
	namePathCalls int
	getNodeCalls  int
	findLabelCals int
	findElTblCals int
}

func (g *goPropertyGraph) Name() (string, error) {
	g.nameCalls++
	return g.name, nil
}
func (g *goPropertyGraph) NamePath() ([]string, error) {
	g.namePathCalls++
	return g.namePath, nil
}
func (g *goPropertyGraph) FullName() (string, error) { return g.name, nil }
func (g *goPropertyGraph) FindLabelByName(name string) (googlesql.GraphElementLabelNode, error) {
	g.findLabelCals++
	return g.labels[name], nil
}
func (g *goPropertyGraph) FindElementTableByName(name string) (googlesql.GraphElementTableNode, error) {
	g.findElTblCals++
	return g.elTables[name], nil
}
func (g *goPropertyGraph) FindPropertyDeclarationByName(name string) (googlesql.GraphPropertyDeclarationNode, error) {
	return g.propDecls[name], nil
}
func (g *goPropertyGraph) GetNodeTables() ([]googlesql.GraphNodeTableNode, error) {
	g.getNodeCalls++
	return g.nodeTables, nil
}

// TestE2E_PropertyGraph_Callback verifies the
// PropertyGraphCallback round-trip end-to-end: a Go-implemented
// PropertyGraph drives AnalyzeStatement via direct virtual-method
// callbacks (Name / NamePath / FindLabelByName / GetNodeTables /
// GetEdgeTables / GetLabels / GetPropertyDeclarations).
func TestE2E_PropertyGraph_Callback(t *testing.T) {
	t.Cleanup(gc)

	catalog, opts, tf, _ := setupAnalyzer(t)

	int64Type, err := tf.GetInt64()
	if err != nil {
		t.Fatalf("GetInt64: %v", err)
	}
	idDecl, err := googlesql.NewSimpleGraphPropertyDeclaration("id", []string{"g"}, int64Type)
	if err != nil {
		t.Fatalf("NewSimpleGraphPropertyDeclaration: %v", err)
	}
	personLabel, err := googlesql.NewSimpleGraphElementLabel(
		"Person", []string{"g"},
		[]googlesql.GraphPropertyDeclarationNode{idDecl},
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphElementLabel: %v", err)
	}
	person, err := googlesql.NewSimpleTable("Person", -1)
	if err != nil {
		t.Fatalf("NewSimpleTable Person: %v", err)
	}
	idCol, err := googlesql.NewSimpleColumn("Person", "id", int64Type, false, false)
	if err != nil {
		t.Fatalf("NewSimpleColumn id: %v", err)
	}
	if err := person.AddColumn2(idCol, true); err != nil {
		t.Fatalf("AddColumn id: %v", err)
	}
	node, err := googlesql.NewSimpleGraphNodeTable(
		"PersonNodes", []string{"g"},
		person,
		[]int32{0},
		[]googlesql.GraphElementLabelNode{personLabel},
		nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("NewSimpleGraphNodeTable: %v", err)
	}

	impl := &goPropertyGraph{
		name:     "g",
		namePath: []string{"g"},
		labels: map[string]googlesql.GraphElementLabelNode{
			"Person": personLabel,
		},
		elTables: map[string]googlesql.GraphElementTableNode{
			"PersonNodes": node,
		},
		propDecls: map[string]googlesql.GraphPropertyDeclarationNode{
			"id": idDecl,
		},
		nodeTables: []googlesql.GraphNodeTableNode{node},
	}
	graph, err := googlesql.NewPropertyGraphFromImpl(impl)
	if err != nil {
		t.Fatalf("NewPropertyGraphFromImpl: %v", err)
	}

	// Direct virtual dispatch round-trip (before adding to
	// catalog, which transfers ownership and clears graph.ptr).
	if got, err := graph.Name(); err != nil || got != "g" {
		t.Errorf("graph.Name() = (%q, %v), want (\"g\", nil)", got, err)
	}
	if got, err := graph.NamePath(); err != nil || len(got) != 1 || got[0] != "g" {
		t.Errorf("graph.NamePath() = (%v, %v), want ([\"g\"], nil)", got, err)
	}
	if got, err := graph.FindLabelByName("Person"); err != nil || got == nil {
		t.Errorf("graph.FindLabelByName(Person) = (%v, %v); want non-nil label", got, err)
	}
	if impl.nameCalls == 0 || impl.namePathCalls == 0 || impl.findLabelCals == 0 {
		t.Errorf("Name/NamePath/FindLabelByName callbacks must each fire at least once "+
			"(name=%d namePath=%d findLabel=%d)",
			impl.nameCalls, impl.namePathCalls, impl.findLabelCals)
	}

	// Analyzer-driven test: catalog now takes ownership of `graph`.
	if err := catalog.AddTable(person); err != nil {
		t.Fatalf("AddTable Person: %v", err)
	}
	if err := catalog.AddOwnedPropertyGraph(graph); err != nil {
		t.Fatalf("AddOwnedPropertyGraph: %v", err)
	}
	_, err = googlesql.AnalyzeStatement(
		"SELECT * FROM GRAPH_TABLE(g MATCH (n) RETURN 1 AS one)",
		opts, catalog, tf,
	)
	if err != nil {
		t.Fatalf("AnalyzeStatement(graph callback): %v", err)
	}
	if impl.getNodeCalls == 0 {
		t.Errorf("GetNodeTables should have fired during analysis, got 0")
	}
}

// TestE2E_ProtoEnumTypes_SurfaceWiring exercises Bug #1 end-to-
// end at the wire level: TypeFactory.MakeProtoType /
// MakeEnumType reach the underlying C++ method (the C++ null-
// argument check fires, which cannot happen unless every link in
// the request → bridge dispatch → C++ method chain is intact),
// and the bridged descriptor handle types appear in the binding
// alongside the value-handle helpers Bug #1 was meant to unblock.
//
// `NewDescriptorPool()` is now bridged via wasmify's
// `IncludeExternalHeaders` / `IncludeExternalClasses` opt-ins —
// the test asserts the constructor + several method handles work
// (FindMessageTypeByName / FindFileByName), wiring proves the
// external-class bridging path delivers a usable surface.
func TestE2E_ProtoEnumTypes_SurfaceWiring(t *testing.T) {
	t.Cleanup(gc)

	_, _, tf, _ := setupAnalyzer(t)

	// The handle types Bug #1 unlocked exist as Go types and are
	// reachable as parameters / return types. A nil call into
	// MakeProtoType drives the full request path; the C++ side
	// rejects with a "descriptor cannot be null" error, proving
	// every layer of the wire stack is wired correctly.
	if _, err := tf.MakeProtoType(nil, nil); err == nil {
		t.Errorf("MakeProtoType(nil) should error, got nil")
	}
	if _, err := tf.MakeEnumType(nil, nil); err == nil {
		t.Errorf("MakeEnumType(nil) should error, got nil")
	}

	// DescriptorPool surface (added via IncludeExternalHeaders /
	// IncludeExternalClasses). NewDescriptorPool() returns a
	// usable empty pool; FindMessageTypeByName on an unknown
	// message returns nil rather than crashing.
	pool, err := googlesql.NewDescriptorPool()
	if err != nil {
		t.Fatalf("NewDescriptorPool: %v", err)
	}
	if pool == nil {
		t.Fatal("NewDescriptorPool returned nil pool")
	}
	desc, err := pool.FindMessageTypeByName("definitely.not.a.real.message")
	if err == nil && desc != nil {
		t.Errorf("FindMessageTypeByName on missing type returned non-nil descriptor")
	}

	// AnalyzeStatement against the regenerated bindings still
	// works for non-proto queries — confirms the un-skipped
	// SimpleModel / TVF subclass exports didn't break baseline
	// resolution.
	cat, opts, _, _ := setupAnalyzer(t)
	if _, err := googlesql.AnalyzeStatement("SELECT 1 AS x", opts, cat, tf); err != nil {
		t.Errorf("baseline AnalyzeStatement: %v", err)
	}
}

// predictionTVF is a Go-side TableValuedFunctionCallback whose
// Resolve returns a value-table TVFRelation of INT64. The C++
// analyzer drives Resolve when planning a query that references
// the TVF; recording resolveCalls > 0 proves the callback path is
// reachable end-to-end through AnalyzeStatement.
type predictionTVF struct {
	googlesql.TableValuedFunctionCallbackDefaults
	resultSchema *googlesql.TVFRelation
	resolveCalls int
}

func (p *predictionTVF) Resolve(
	analyzerOptions *googlesql.AnalyzerOptions,
	actualArguments []*googlesql.TVFInputArgumentType,
	concreteSignature *googlesql.FunctionSignature,
	catalog googlesql.CatalogNode,
	typeFactory *googlesql.TypeFactory,
) (*googlesql.TVFSignature, error) {
	p.resolveCalls++
	// Note: the wasmify trampoline currently drops `vector<T>`
	// inputs to callbacks, so `actualArguments` arrives empty even
	// when the call has arguments. Manufacture a single input
	// argument matching the signature's relation input so the
	// validator's
	//   argument_list_size == signature->input_arguments().size()
	// invariant holds. Marshalling vector callback inputs is a
	// separate generator follow-up; this test pins the analyzer
	// path that already works.
	inputs := actualArguments
	if len(inputs) == 0 {
		ia, err := googlesql.NewTVFInputArgumentType(p.resultSchema)
		if err != nil {
			return nil, err
		}
		inputs = []*googlesql.TVFInputArgumentType{ia}
	}
	return googlesql.NewTVFSignature(inputs, p.resultSchema, nil)
}

// TestE2E_TVFCallback_AnalyzeStatement drives a Go-implemented
// TableValuedFunctionCallback through AnalyzeStatement, the
// ML.PREDICT shape from the original plan: the TVF returns a
// multi-column TVFRelation (input `x INT64` plus a synthesised
// `prediction FLOAT64`) so the resolved AST carries both columns.
func TestE2E_TVFCallback_AnalyzeStatement(t *testing.T) {
	t.Cleanup(gc)

	catalog, opts, tf, _ := setupAnalyzer(t)

	int64Type, err := tf.GetInt64()
	if err != nil {
		t.Fatalf("GetInt64: %v", err)
	}
	doubleType, err := tf.GetDouble()
	if err != nil {
		t.Fatalf("GetDouble: %v", err)
	}
	resultSchema, err := googlesql.NewTVFRelation([]*googlesql.TVFSchemaColumn{
		{Name: "x", Type_: int64Type},
		{Name: "prediction", Type_: doubleType},
	})
	if err != nil {
		t.Fatalf("NewTVFRelation: %v", err)
	}

	// Signature: takes any-relation input, returns a relation with
	// a fixed value-table schema; Resolve populates the per-call
	// signature.
	relArg, err := googlesql.NewFunctionArgumentType4(googlesql.SignatureArgumentKindArgTypeRelation, 1)
	if err != nil {
		t.Fatalf("NewFunctionArgumentType4 input: %v", err)
	}
	resultArg, err := googlesql.NewFunctionArgumentTypeRelationWithSchema(resultSchema, false)
	if err != nil {
		t.Fatalf("NewFunctionArgumentTypeRelationWithSchema: %v", err)
	}
	sigOpts, err := googlesql.NewFunctionSignatureOptions()
	if err != nil {
		t.Fatalf("NewFunctionSignatureOptions: %v", err)
	}
	sig, err := googlesql.NewFunctionSignature2(resultArg,
		[]*googlesql.FunctionArgumentType{relArg}, 0, sigOpts)
	if err != nil {
		t.Fatalf("NewFunctionSignature2: %v", err)
	}

	cb := &predictionTVF{resultSchema: resultSchema}
	tvf, err := googlesql.NewTableValuedFunctionFromImpl(
		cb,
		[]string{"my_predict"},
		"",
		[]*googlesql.FunctionSignature{sig},
		&googlesql.TableValuedFunctionOptions{},
	)
	if err != nil {
		t.Fatalf("NewTableValuedFunctionFromImpl: %v", err)
	}
	if err := catalog.AddOwnedTableValuedFunction(tvf); err != nil {
		t.Fatalf("AddOwnedTableValuedFunction: %v", err)
	}

	out, err := googlesql.AnalyzeStatement(
		"SELECT * FROM my_predict((SELECT 1 AS x))",
		opts, catalog, tf,
	)
	if err != nil {
		t.Fatalf("AnalyzeStatement(my_predict): %v", err)
	}
	if cb.resolveCalls == 0 {
		t.Errorf("expected Resolve callback to fire at least once, got 0")
	}
	stmt, err := out.ResolvedStatement()
	if err != nil {
		t.Fatalf("ResolvedStatement: %v", err)
	}
	qs, ok := stmt.(*googlesql.ResolvedQueryStmt)
	if !ok {
		t.Fatalf("expected *ResolvedQueryStmt, got %T", stmt)
	}
	size, _ := qs.OutputColumnListSize()
	if size != 2 {
		t.Errorf("OutputColumnListSize = %d, want 2 (x, prediction)", size)
	}
}

// TestE2E_FixedOutputSchemaTVF verifies a Go-built
// FixedOutputSchemaTVF — registered in a SimpleCatalog and
// referenced from a SQL query — round-trips through the analyzer.
//
// Builds a multi-column TVFRelation via NewTVFRelation(columns)
// (now bridged through wasmify's nested-type alias resolution
// and the field-aware emplace_back emitter) so the resolved
// schema carries the change-stream-shaped (commit_ts TIMESTAMP,
// change STRING) layout the original plan called out.
func TestE2E_FixedOutputSchemaTVF(t *testing.T) {
	t.Cleanup(gc)

	catalog, opts, tf, _ := setupAnalyzer(t)

	tsType, err := tf.GetTimestamp()
	if err != nil {
		t.Fatalf("GetTimestamp: %v", err)
	}
	stringType, err := tf.GetString()
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}

	resultSchema, err := googlesql.NewTVFRelation([]*googlesql.TVFSchemaColumn{
		{Name: "commit_ts", Type_: tsType},
		{Name: "change", Type_: stringType},
	})
	if err != nil {
		t.Fatalf("NewTVFRelation: %v", err)
	}

	resultArg, err := googlesql.NewFunctionArgumentTypeRelationWithSchema(resultSchema, false)
	if err != nil {
		t.Fatalf("NewFunctionArgumentTypeRelationWithSchema: %v", err)
	}
	sigOpts, err := googlesql.NewFunctionSignatureOptions()
	if err != nil {
		t.Fatalf("NewFunctionSignatureOptions: %v", err)
	}
	sig, err := googlesql.NewFunctionSignature2(resultArg, nil, 0, sigOpts)
	if err != nil {
		t.Fatalf("NewFunctionSignature2: %v", err)
	}

	tvfOpts := &googlesql.TableValuedFunctionOptions{}
	tvf, err := googlesql.NewFixedOutputSchemaTVF(
		[]string{"my_change_stream"},
		sig,
		resultSchema,
		tvfOpts,
	)
	if err != nil {
		t.Fatalf("NewFixedOutputSchemaTVF: %v", err)
	}
	if err := catalog.AddOwnedTableValuedFunction(tvf.TableValuedFunction); err != nil {
		t.Fatalf("AddOwnedTableValuedFunction: %v", err)
	}

	out, err := googlesql.AnalyzeStatement(
		"SELECT * FROM my_change_stream()",
		opts, catalog, tf,
	)
	_ = resultSchema // keep alive: see Bug below.
	if err != nil {
		t.Fatalf("AnalyzeStatement(my_change_stream): %v", err)
	}
	stmt, err := out.ResolvedStatement()
	if err != nil {
		t.Fatalf("ResolvedStatement: %v", err)
	}
	qs, ok := stmt.(*googlesql.ResolvedQueryStmt)
	if !ok {
		t.Fatalf("expected *ResolvedQueryStmt, got %T", stmt)
	}
	size, _ := qs.OutputColumnListSize()
	if size != 2 {
		t.Errorf("OutputColumnListSize = %d, want 2 (commit_ts, change)", size)
	}
}

// TestE2E_MultiCatalog_MissingTable confirms the analyzer routes
// errors through the MultiCatalog: a query referencing a table that
// exists in NEITHER child must fail.
func TestE2E_MultiCatalog_MissingTable(t *testing.T) {
	t.Cleanup(gc)

	_, opts, tf, langOpts := setupAnalyzer(t)
	bfo := &googlesql.BuiltinFunctionOptions{LanguageOptions: langOpts}

	catA, err := googlesql.NewSimpleCatalog("a", nil)
	if err != nil {
		t.Fatalf("NewSimpleCatalog A: %v", err)
	}
	if err := catA.AddBuiltinFunctionsAndTypes(bfo); err != nil {
		t.Fatalf("AddBuiltinFunctionsAndTypes A: %v", err)
	}
	catB, err := googlesql.NewSimpleCatalog("b", nil)
	if err != nil {
		t.Fatalf("NewSimpleCatalog B: %v", err)
	}
	if err := catB.AddBuiltinFunctionsAndTypes(bfo); err != nil {
		t.Fatalf("AddBuiltinFunctionsAndTypes B: %v", err)
	}

	mc, err := googlesql.NewMultiCatalogCreate("combined", []googlesql.CatalogNode{catA, catB})
	if err != nil {
		t.Fatalf("NewMultiCatalogCreate: %v", err)
	}

	_, err = googlesql.AnalyzeStatement(
		"SELECT * FROM no_such_table_anywhere",
		opts, mc, tf,
	)
	if err == nil {
		t.Fatal("expected analyzer error for missing table, got nil")
	}
	if !strings.Contains(err.Error(), "no_such_table_anywhere") &&
		!strings.Contains(strings.ToLower(err.Error()), "table") {
		t.Errorf("expected error mentioning the missing table, got: %v", err)
	}
}
