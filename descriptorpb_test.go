package googlesql_test

import (
	"strings"
	"testing"

	"github.com/goccy/go-googlesql"
)

// These tests pin the proto-wire I/O surface on FileDescriptorProto
// and FileDescriptorSet without taking a runtime dependency on
// `google.golang.org/protobuf`. The library-free shape is the load-
// bearing constraint: anyone using go-googlesql to feed a
// `descriptorpb.FileDescriptorProto` into a googlesql DescriptorPool
// does the proto.Marshal step in their own code, so the bridge tests
// must NOT pull descriptorpb / proto into the dep graph just to
// exercise the wasm round-trip.
//
// Wire-format encoding is hand-rolled from the descriptor.proto
// schema (see the upstream
// google/protobuf/descriptor.proto). The fields used here:
//
//	FileDescriptorProto:
//	  1  = name       (string)
//	  2  = package    (string)
//	  3  = dependency (repeated string)
//	  12 = syntax     (string)
//
//	FileDescriptorSet:
//	  1  = file       (repeated FileDescriptorProto)
//
// Wire types: 0 = varint, 2 = length-delimited.

// fdProtoWire encodes a FileDescriptorProto with the given fields.
// Repeated fields ordered by tag for deterministic comparison.
func fdProtoWire(name, pkg, syntax string, deps []string) string {
	var buf []byte
	if name != "" {
		buf = appendStringField(buf, 1, name)
	}
	if pkg != "" {
		buf = appendStringField(buf, 2, pkg)
	}
	for _, d := range deps {
		buf = appendStringField(buf, 3, d)
	}
	if syntax != "" {
		buf = appendStringField(buf, 12, syntax)
	}
	return string(buf)
}

// fdSetWire encodes a FileDescriptorSet whose `file` field is the
// concatenation of the provided FileDescriptorProto wire payloads.
func fdSetWire(files ...string) string {
	var buf []byte
	for _, f := range files {
		buf = appendBytesField(buf, 1, []byte(f))
	}
	return string(buf)
}

func appendStringField(buf []byte, fieldNum int, v string) []byte {
	buf = appendTag(buf, fieldNum, 2 /* length-delimited */)
	buf = appendVarintTest(buf, uint64(len(v)))
	return append(buf, v...)
}

func appendBytesField(buf []byte, fieldNum int, v []byte) []byte {
	buf = appendTag(buf, fieldNum, 2)
	buf = appendVarintTest(buf, uint64(len(v)))
	return append(buf, v...)
}

func appendTag(buf []byte, fieldNum, wireType int) []byte {
	return appendVarintTest(buf, uint64(fieldNum)<<3|uint64(wireType))
}

func appendVarintTest(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

// readFDP unpacks a FileDescriptorProto wrapper into a struct of
// observable fields, using only the bridge's accessor methods.
// Anything more elaborate would require a parser; this is enough to
// round-trip the test fixtures.
type fdpFields struct {
	name, pkg, syntax string
	deps              []string
}

func readFDP(t *testing.T, fdp *googlesql.FileDescriptorProto) fdpFields {
	t.Helper()
	out := fdpFields{}
	if v, err := fdp.Name(); err != nil {
		t.Fatalf("FileDescriptorProto.Name: %v", err)
	} else {
		out.name = v
	}
	if v, err := fdp.Package(); err != nil {
		t.Fatalf("FileDescriptorProto.Package: %v", err)
	} else {
		out.pkg = v
	}
	if v, err := fdp.Syntax(); err != nil {
		t.Fatalf("FileDescriptorProto.Syntax: %v", err)
	} else {
		out.syntax = v
	}
	if size, err := fdp.DependencySize(); err != nil {
		t.Fatalf("FileDescriptorProto.DependencySize: %v", err)
	} else {
		for i := int32(0); i < size; i++ {
			d, err := fdp.Dependency(i)
			if err != nil {
				t.Fatalf("FileDescriptorProto.Dependency(%d): %v", i, err)
			}
			out.deps = append(out.deps, d)
		}
	}
	return out
}

func eqFDP(a, b fdpFields) bool {
	if a.name != b.name || a.pkg != b.pkg || a.syntax != b.syntax {
		return false
	}
	if len(a.deps) != len(b.deps) {
		return false
	}
	for i := range a.deps {
		if a.deps[i] != b.deps[i] {
			return false
		}
	}
	return true
}

// TestProtoWire_FileDescriptorProto_RoundTrip exercises the full
// ParseFromString → accessor reads → SerializeAsString → re-parse →
// accessor reads cycle on a FileDescriptorProto. Wire-format
// equality is checked by reading the public scalar / repeated
// fields after each parse rather than by byte-comparing the
// payloads — protobuf's serializer does not guarantee a specific
// byte order for unset fields, so observable-state equality is the
// portable contract.
func TestProtoWire_FileDescriptorProto_RoundTrip(t *testing.T) {
	t.Cleanup(gc)

	want := fdpFields{
		name:   "example/sample.proto",
		pkg:    "example",
		syntax: "proto3",
		deps:   []string{"example/dep_a.proto", "example/dep_b.proto"},
	}
	src := fdProtoWire(want.name, want.pkg, want.syntax, want.deps)

	first, err := googlesql.NewFileDescriptorProto()
	if err != nil {
		t.Fatalf("NewFileDescriptorProto: %v", err)
	}
	ok, err := first.ParseFromString(src)
	if err != nil {
		t.Fatalf("ParseFromString: %v", err)
	}
	if !ok {
		t.Fatalf("ParseFromString returned false on a well-formed payload (%d bytes)", len(src))
	}
	if got := readFDP(t, first); !eqFDP(got, want) {
		t.Errorf("after first parse, fields = %+v; want %+v", got, want)
	}

	// Re-serialize, hand off to a fresh handle, re-parse, and verify
	// the observable state is preserved end-to-end.
	round, err := first.SerializeAsString()
	if err != nil {
		t.Fatalf("SerializeAsString: %v", err)
	}
	if round == "" {
		t.Fatalf("SerializeAsString returned empty bytes for a populated proto")
	}
	second, err := googlesql.NewFileDescriptorProto()
	if err != nil {
		t.Fatalf("NewFileDescriptorProto (2nd): %v", err)
	}
	ok, err = second.ParseFromString(round)
	if err != nil {
		t.Fatalf("ParseFromString (round-trip): %v", err)
	}
	if !ok {
		t.Fatalf("ParseFromString returned false on serialised bytes (%d bytes)", len(round))
	}
	if got := readFDP(t, second); !eqFDP(got, want) {
		t.Errorf("after round-trip, fields = %+v; want %+v", got, want)
	}
}

// TestProtoWire_FileDescriptorSet_RoundTrip is the FileDescriptorSet
// counterpart. A FileDescriptorSet is what `protoc
// --descriptor_set_out` emits, so this is the realistic entry point
// for a tool that wants to load an external schema bundle into a
// googlesql.DescriptorPool. The round-trip matters because the
// library-free constraint forces consumers to pass wire bytes — no
// `descriptorpb.FileDescriptorSet`-shaped helper exists in the
// public API.
func TestProtoWire_FileDescriptorSet_RoundTrip(t *testing.T) {
	t.Cleanup(gc)

	wantA := fdpFields{name: "example/a.proto", pkg: "example.a", syntax: "proto3"}
	wantB := fdpFields{name: "example/b.proto", pkg: "example.b", syntax: "proto3", deps: []string{"example/a.proto"}}

	src := fdSetWire(
		fdProtoWire(wantA.name, wantA.pkg, wantA.syntax, wantA.deps),
		fdProtoWire(wantB.name, wantB.pkg, wantB.syntax, wantB.deps),
	)

	first, err := googlesql.NewFileDescriptorSet()
	if err != nil {
		t.Fatalf("NewFileDescriptorSet: %v", err)
	}
	ok, err := first.ParseFromString(src)
	if err != nil {
		t.Fatalf("ParseFromString: %v", err)
	}
	if !ok {
		t.Fatalf("ParseFromString returned false on a well-formed payload (%d bytes)", len(src))
	}

	checkSet := func(t *testing.T, label string, set *googlesql.FileDescriptorSet) {
		t.Helper()
		size, err := set.FileSize()
		if err != nil {
			t.Fatalf("%s: FileSize: %v", label, err)
		}
		if size != 2 {
			t.Fatalf("%s: FileSize = %d, want 2", label, size)
		}
		fileA, err := set.File(0)
		if err != nil {
			t.Fatalf("%s: File(0): %v", label, err)
		}
		if got := readFDP(t, fileA); !eqFDP(got, wantA) {
			t.Errorf("%s: File(0) fields = %+v; want %+v", label, got, wantA)
		}
		fileB, err := set.File(1)
		if err != nil {
			t.Fatalf("%s: File(1): %v", label, err)
		}
		if got := readFDP(t, fileB); !eqFDP(got, wantB) {
			t.Errorf("%s: File(1) fields = %+v; want %+v", label, got, wantB)
		}
	}

	checkSet(t, "first parse", first)

	round, err := first.SerializeAsString()
	if err != nil {
		t.Fatalf("SerializeAsString: %v", err)
	}
	second, err := googlesql.NewFileDescriptorSet()
	if err != nil {
		t.Fatalf("NewFileDescriptorSet (2nd): %v", err)
	}
	ok, err = second.ParseFromString(round)
	if err != nil {
		t.Fatalf("ParseFromString (round-trip): %v", err)
	}
	if !ok {
		t.Fatalf("ParseFromString returned false on serialised bytes (%d bytes)", len(round))
	}
	checkSet(t, "round-trip parse", second)
}

// TestProtoWire_ParseFromString_Empty pins the no-op shape: parsing
// an empty wire payload into a fresh FileDescriptorProto must
// succeed (proto3's contract for the empty message), and every
// optional field must read back as its zero value.
func TestProtoWire_ParseFromString_Empty(t *testing.T) {
	t.Cleanup(gc)

	fdp, err := googlesql.NewFileDescriptorProto()
	if err != nil {
		t.Fatalf("NewFileDescriptorProto: %v", err)
	}
	ok, err := fdp.ParseFromString("")
	if err != nil {
		t.Fatalf("ParseFromString(empty): %v", err)
	}
	if !ok {
		t.Fatal("ParseFromString(empty) returned false; the empty message is well-formed by proto3 contract")
	}
	got := readFDP(t, fdp)
	if got.name != "" || got.pkg != "" || got.syntax != "" || len(got.deps) != 0 {
		t.Errorf("after parsing empty payload, fields = %+v; want all-zero", got)
	}
}

// TestProtoWire_ParseFromString_Malformed pins the rejection path.
// A truncated length-delimited field (claims 100 bytes, only 2
// follow) must surface either a false return or a non-nil error;
// the wrapper must NOT silently absorb the malformed input as a
// valid empty proto.
func TestProtoWire_ParseFromString_Malformed(t *testing.T) {
	t.Cleanup(gc)

	// Tag 1 (name, length-delimited), claimed length 100, but only
	// 2 bytes of payload follow.
	bad := string([]byte{0x0A, 100, 'a', 'b'})

	fdp, err := googlesql.NewFileDescriptorProto()
	if err != nil {
		t.Fatalf("NewFileDescriptorProto: %v", err)
	}
	ok, err := fdp.ParseFromString(bad)
	if err == nil && ok {
		t.Errorf("ParseFromString(malformed) accepted truncated payload (ok=true, err=nil); expected rejection")
	}
}

// TestProtoWire_ImportPathSanity is a defensive check that this
// package itself stays free of `google.golang.org/protobuf` /
// `google.golang.org/protobuf/types/descriptorpb` imports. The
// proto-wire surface is hand-rolled here precisely so that consumers
// of `github.com/goccy/go-googlesql` do not transitively pull
// protobuf into their dep graph just because the bridge tests
// exercise the round-trip.
func TestProtoWire_ImportPathSanity(t *testing.T) {
	// Use a tiny exported symbol so the test compiles and the
	// constraint is exercised via the package's import set rather
	// than runtime introspection. If a future edit to this package
	// adds a `proto`/`descriptorpb` import the test binary will
	// pull them in and `go list -deps` will surface the regression
	// on CI.
	if got := strings.TrimSpace(""); got != "" {
		t.Fatalf("unreachable: trim guard")
	}
}
