# buildcost — wasm2go build/lint cost analysis

Reproduction harness for `docs/wasm2go-build-cost.md`. These are standalone
scripts (`//go:build ignore`); run them with `go run`. They are analysis tools —
the actual fix lives in the upstream **wasm2go** generator, not here.

## Tools

| File | Purpose |
|---|---|
| `runmax.go` | Run a command; print wall time + peak child RSS (via `wait4` rusage). `go run runmax.go <cmd...>` |
| `emitter_cleanups.go` | Apply the safe emitter cleanups to one generated file (drop redundant `_ = vN` blank-uses + lone `;` after labels). `go run emitter_cleanups.go p0.go > out.go` |
| `inline_singleuse.go` | Copy-propagate single-use, adjacent stack temps in one file (no-chain-safe). Run to a fixpoint by re-feeding output. |
| `memhelper_rejected.go` | **Rejected** approach: lower `*(*T)(unsafe.Add(m.M,a))` to `m.L*/m.S*` calls. Kept only to document the >12× `go build` / 14.5 GB lint regression it causes. |

## Measuring golangci-lint cold (the headline number)

```sh
# warm std only, keep the target packages cold
tmpl=$(mktemp -d); GOCACHE=$tmpl go build std
run=$(mktemp -d); cp -a "$tmpl"/* "$run"/
GOCACHE=$run GOLANGCI_LINT_CACHE=$(mktemp -d) \
  go run tools/buildcost/runmax.go \
  golangci-lint run --default=none --enable=govet ./...
```

## Reproducing the "runtime as external module" win

1. Copy `internal/wasm2go/{base,p0..p10,wasm2go.go,data.bin}` into a new module
   root with `module github.com/goccy/go-googlesql/internal/wasm2go` in its
   `go.mod`.
2. Serve it from a file proxy (`GOPROXY=file://…`, `GOSUMDB=off`) so it lands in
   the module cache; do **not** use a `replace => ./dir` (that stays source).
3. Drop `internal/` from the main module and add
   `require github.com/goccy/go-googlesql/internal/wasm2go vX`.
4. `go build ./...` once (compiles the dep into cache), then measure
   `golangci-lint run` — it now uses the dep's export data.

Result: govet warm 120 s / 5.5 GB (in-module) → 55 s / 3.0 GB (external module).
