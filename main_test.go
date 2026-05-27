package googlesql_test

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/goccy/go-googlesql"
)

// TestMain initialises the wasm2go runtime once before any test runs.
// The generated bindings ship as pure Go via the internal/wasm2go
// packages, so Init takes no arguments and there is no Close to call.
func TestMain(m *testing.M) {
	if err := googlesql.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "wasm init failed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// gc forces garbage collection so wasm-side handles released via
// runtime.SetFinalizer are freed between tests sharing the module.
func gc() {
	runtime.GC()
	runtime.GC()
}
