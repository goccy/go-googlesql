# go-googlesql

[![CI](https://github.com/goccy/go-googlesql/actions/workflows/ci.yml/badge.svg)](https://github.com/goccy/go-googlesql/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/goccy/go-googlesql.svg)](https://pkg.go.dev/github.com/goccy/go-googlesql)

Pure-Go bindings for [GoogleSQL](https://github.com/google/googlesql),
the SQL dialect that powers BigQuery, Spanner, and other Google Cloud
databases.

## Why this library

I previously maintained
[`goccy/go-zetasql`](https://github.com/goccy/go-zetasql), which
exposed the same engine through cgo. The cgo dependency made
cross-compilation, static linking, and overall portability painful.
`go-googlesql` solves that by running GoogleSQL inside a WebAssembly
module — no cgo, no native toolchain, just a pure Go library.

## Features

- **Pure Go, no cgo.** GoogleSQL runs inside a WebAssembly module
  loaded with [`tetratelabs/wazero`](https://github.com/tetratelabs/wazero).
  `CGO_ENABLED=0` builds, static linking, and cross-compilation all
  work without extra setup.
- **Auto-generated end-to-end.** `googlesql.wasm` and the Go bridge
  `googlesql.go` are produced upstream by
  [`goccy/wasmify`](https://github.com/goccy/wasmify) +
  [`goccy/googlesql-wasm`](https://github.com/goccy/googlesql-wasm).
  When upstream GoogleSQL ships a new revision, the artifacts here
  follow without manual intervention.
- **End-to-end provenance.** Every released artifact is signed with
  [GitHub artifact attestations](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations).
  CI re-verifies the in-tree binary against the signed release on
  every PR (see [Verifying provenance](#verifying-provenance)).
- **Both wazero execution modes.** Compiler and interpreter modes are
  selectable through `WithCompilationMode`. See
  [Resource footprint](#resource-footprint) for the trade-offs.

## Status

Tracks GoogleSQL revision
[`36dd14aa0657ea299725504bc0f938732f58f380`](https://github.com/google/googlesql/commit/36dd14aa0657ea299725504bc0f938732f58f380)
(2026-01-31). New upstream revisions are picked up here as they land.

The library passes the full test suite of
[`goccy/go-zetasqlite`](https://github.com/goccy/go-zetasqlite), which
gives a baseline that it is usable for real workloads. It has not yet
seen broad production traffic, so treat it as **early-access**. Planned
next steps: migrate `goccy/go-zetasqlite` and
[`goccy/bigquery-emulator`](https://github.com/goccy/bigquery-emulator)
onto `go-googlesql`.

## Installation

```sh
go get github.com/goccy/go-googlesql
```

The first build is heavy: the embedded `googlesql.wasm` is ~12 MB and
the generated `googlesql.go` bridge is ~7 MB. Expect the Go compiler
to need several gigabytes of RAM and tens of seconds for a cold
build. See [Resource footprint](#resource-footprint) for measured
numbers and runtime cost.

## Synopsis

### Initialise the wasm runtime

`Init` loads the embedded wasm module. Call it once per process before
using any other API; it is `sync.Once`-guarded so calling more than
once is a no-op.

```go
package main

import "github.com/goccy/go-googlesql"

func main() {
    if err := googlesql.Init(
        googlesql.WithCompilationMode(googlesql.CompilationModeCompiler),
        // Optional: re-use a persistent wazero compilation cache so
        // subsequent processes skip the ~3 s wasm-to-native compile.
        googlesql.WithCompilationCache("/var/cache/go-googlesql"),
    ); err != nil {
        panic(err)
    }
    defer googlesql.Close()

    // ...use the parser / analyzer APIs here...
}
```

### Parse a SQL statement

```go
opts, err := googlesql.NewParserOptions()
if err != nil {
    panic(err)
}
out, err := googlesql.ParseStatement("SELECT * FROM Samples WHERE id = 1", opts)
if err != nil {
    panic(err)
}
stmt, err := out.Statement()
if err != nil {
    panic(err)
}

// Use a type assertion to reach concrete AST node types.
queryStmt, ok := stmt.(*googlesql.ASTQueryStatement)
_ = queryStmt
_ = ok
```

### Analyze a SQL statement against a catalog

```go
catalog, err := googlesql.NewSimpleCatalog("catalog", nil)
if err != nil {
    panic(err)
}

langOpts, err := googlesql.NewLanguageOptions()
if err != nil {
    panic(err)
}
_ = langOpts.EnableMaximumLanguageFeaturesForDevelopment()
_ = langOpts.SetSupportsAllStatementKinds()

if err := catalog.AddBuiltinFunctionsAndTypes(
    &googlesql.BuiltinFunctionOptions{LanguageOptions: langOpts},
); err != nil {
    panic(err)
}

opts, err := googlesql.NewAnalyzerOptions2()
if err != nil {
    panic(err)
}
_ = opts.SetLanguage(langOpts)

tf, err := googlesql.NewTypeFactory()
if err != nil {
    panic(err)
}

out, err := googlesql.AnalyzeStatement(
    "SELECT 1 AS col1, 'hi' AS col2",
    opts, catalog, tf,
)
if err != nil {
    panic(err)
}
resolved, err := out.ResolvedStatement()
if err != nil {
    panic(err)
}
_ = resolved
```

## Verifying provenance

You can re-run the upstream attestation check yourself, locally and
**without a GitHub access token**:

```sh
make verify
```

The Makefile target does two things:

1. `verify-release` downloads the v0.1.2 release artifacts from
   `goccy/googlesql-wasm` and byte-for-byte compares them to the
   in-tree files. Any divergence aborts the build.
2. `verify-attestation` fetches each artifact's GitHub attestation
   bundle from the public `/attestations/` API, hands it to
   `gh attestation verify --bundle`, and pins the signer to
   `goccy/googlesql-wasm/.github/workflows/build.yml`.

Both checks run unauthenticated — no `gh auth login`, no `GH_TOKEN` /
`GITHUB_TOKEN` (technique from
<https://zenn.dev/shunsuke_suzuki/articles/gh-at-verify-without-access-token>).
CI runs the same target on every push to `main` and every pull request
before running the test suite.

## Resource footprint

GoogleSQL is a large engine, and its compiled form is correspondingly
large: the embedded WebAssembly module is ~12 MB and the Go bridge is
~7 MB. Build cost and runtime cost are higher than for a typical Go
dependency.

### Build

`go test -c` against this package on a cold Go build cache.

| Phase            | Wall time | Peak RSS  |
|------------------|-----------|-----------|
| `go test -c .`   | ~22 s     | ~3.7 GB   |

### Runtime — `googlesql.Init`

Three runtime configurations are exposed via `WithCompilationMode`
and `WithCompilationCache(dir)`:

| Mode                              | Init wall  | Peak RSS  | Steady-state in-use |
|-----------------------------------|------------|-----------|----------------------|
| Compiler, no cache                | ~3.4 s     | ~500 MB   | ~87 MB               |
| Compiler, **warm wazero cache**   | **~0.57 s**| **~205 MB** | **~87 MB**         |
| Interpreter                       | ~0.85 s    | ~500 MB   | ~383 MB              |

How to read this:

- **Init wall** is the time `googlesql.Init(...)` blocks. Compiler
  mode without cache compiles ~12 MB of wasm to native machine code
  in-process every call; that is where the ~3.4 s comes from. When
  `WithCompilationCache(dir)` points at a populated directory, the
  compile phase is skipped and Init drops to ~0.6 s.
- **Peak RSS** is the OS-level peak resident set size during the
  whole process (`/usr/bin/time -l`). It captures the transient spike
  during compile.
- **Steady-state in-use** is the resident Go heap _after_ Init,
  measured via `runtime/pprof.WriteHeapProfile` (top of `inuse_space`,
  post-`runtime.GC`). This is what your service pays long-term.
  Compiler mode keeps ~87 MB of wasm-runtime metadata; the interpreter
  retains the full decoded operation stream (~290 MB) on top of that,
  which is the gap.

The "warm wazero cache" row is the long-running mode you almost
certainly want in production: low Init latency *and* small steady-state
heap. Pass `WithCompilationCache("/some/persistent/dir")` to enable
it.

Measured on Apple M5 (32 GB RAM), macOS 26.2, Go 1.26.2 amd64
toolchain via Rosetta. Native arm64 toolchains should be somewhat
faster.

## License

[MIT](LICENSE).
