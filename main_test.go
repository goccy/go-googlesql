package googlesql_test

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/goccy/go-googlesql"
)

// TestMain initialises the wasm runtime once and tears it down after
// every test. The wasm binary is embedded into the googlesql package
// via //go:embed googlesql.wasm, so Init takes no arguments. The
// wazero compiler backend cuts per-test analysis down from minutes
// (interpreter) to milliseconds for googlesql's register-all-builtins
// code path.
func TestMain(m *testing.M) {
	if err := googlesql.Init(googlesql.WithCompilationMode(googlesql.CompilationModeCompiler)); err != nil {
		fmt.Fprintf(os.Stderr, "wasm init failed: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	googlesql.Close()
	os.Exit(code)
}

// gc forces garbage collection so wasm-side handles released via
// runtime.SetFinalizer are freed between tests sharing the module.
func gc() {
	runtime.GC()
	runtime.GC()
}
