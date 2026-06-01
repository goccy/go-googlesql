# buildcost — wasm2go build/lint cost analysis

Reproduction harness for `docs/wasm2go-build-cost.md`. Standalone scripts
(`//go:build ignore`); run them with `go run`. Analysis tools only — the actual
fix lives in the upstream **wasm2go** generator (and **googlesql-wasm**).

## Tools

| File | Purpose |
|---|---|
| `runmax.go` | Run a command; print wall time + peak child RSS (via `wait4` rusage). `go run runmax.go <cmd...>` |
| `emitter_cleanups.go` | Drop redundant `_ = vN` blank-uses (keep only write-only temps) + lone `;` after labels. `go run emitter_cleanups.go p0.go > out.go` |
| `inline_singleuse.go` | Copy-propagate single-use, adjacent stack temps (no-chain-safe). Run to a fixpoint by re-feeding output. |
| `noinline_emit.go` | **Rejected (no effect):** prepend `//go:noinline` to every translated `func FnN`. |
| `memhelper_rejected.go` | **Rejected (regresses):** lower `*(*T)(unsafe.Add(m.M,a))` to `m.L*/m.S*` calls — `go build` >12× slower, 14.5 GB lint. |
| `wasm_inspect.go` | Dump a WASM module's section map + defined-function/export counts. `go run wasm_inspect.go googlesql.wasm`. See `docs/wasm-size-experiment.md`. |

## Trustworthy measurement: deterministic per-package compile

`go build`-level timing on a shared box is noise-dominated (cold baseline varied
114–146 s). Use cache-free `go tool compile` on the largest package instead:

```sh
GOROOT=$(go env GOROOT)
go build ./internal/wasm2go/base/                       # produce base export data
BASE=$(go list -export -f '{{.Export}}' ./internal/wasm2go/base/)
echo "packagefile github.com/goccy/go-googlesql/internal/wasm2go/base=$BASE" > /tmp/p0.importcfg
go run runmax.go "$GOROOT/pkg/tool/linux_amd64/compile" \
  -p github.com/goccy/go-googlesql/internal/wasm2go/p0 \
  -importcfg /tmp/p0.importcfg -o /tmp/o.a -pack internal/wasm2go/p0/p0.go
```

Measured p0 (cache-free, 4 reps back-to-back; wall spread ±3 % = noise):

| Variant | wall | peak RSS |
|---|---|---|
| baseline | ~7.0 s | ~707 MB |
| emitter cleanups (`emitter_cleanups.go` + `inline_singleuse.go`) | ~7.2 s | ~644 MB (RSS −9 %, wall flat) |
| `//go:noinline` only | ~7.0 s | ~698 MB (no change) |
| both | ~6.95 s | ~644 MB |

Apply transforms to a copy; the in-tree `internal/wasm2go/*.go` are
sha256/attestation-verified and must not change.
