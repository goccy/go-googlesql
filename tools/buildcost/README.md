# buildcost — wasm2go build/lint cost analysis

Reproduction harness for `docs/wasm2go-build-cost.md`. These are standalone
scripts (`//go:build ignore`); run them with `go run`. They are analysis tools —
the actual fix lives in the upstream **wasm2go** generator, not here.

## Tools

| File | Purpose |
|---|---|
| `runmax.go` | Run a command; print wall time + peak child RSS (via `wait4` rusage). `go run runmax.go <cmd...>` |
| `emitter_cleanups.go` | Drop redundant `_ = vN` blank-uses (keep only write-only temps) + lone `;` after labels. `go run emitter_cleanups.go p0.go > out.go` |
| `inline_singleuse.go` | Copy-propagate single-use, adjacent stack temps in one file (no-chain-safe). Run to a fixpoint by re-feeding output. |
| `memhelper_rejected.go` | **Rejected** approach: lower `*(*T)(unsafe.Add(m.M,a))` to `m.L*/m.S*` calls. Kept only to document the >12× `go build` / 14.5 GB lint regression it causes. |

## Downstream consumer cost — cold `go build ./internal/wasm2go/...`

What a dependent project's golangci pays to compile go-googlesql to export data
(idle 4-CPU/15 GB, std pre-warmed, `-p=4`, fresh GOCACHE each run):

| Variant | wall | peak RSS |
|---|---|---|
| baseline | 116.9 s | 1.58 GB |
| `//go:noinline` only | 116.7 s | 1.58 GB (no change) |
| emitter cleanups (`emitter_cleanups.go` + `inline_singleuse.go`) | 78.3 s | 0.86 GB |

Reproduce: apply `inline_singleuse.go` to a fixpoint, then `emitter_cleanups.go`,
to each `internal/wasm2go/p*/p*.go`; restore the originals afterwards (they are
sha256/attestation-verified and must not change in-tree).

## Measuring a cold build/lint

```sh
# warm std only, keep the target packages cold
tmpl=$(mktemp -d); GOCACHE=$tmpl go build std
run=$(mktemp -d); cp -a "$tmpl"/* "$run"/
GOCACHE=$run GOFLAGS=-p=4 \
  go run tools/buildcost/runmax.go \
  go build ./internal/wasm2go/...
```

For the in-module golangci type-check case (not the downstream scenario, kept
for completeness — the cleanups do **not** move it: 262 s vs 276 s baseline):

```sh
GOCACHE=$run GOLANGCI_LINT_CACHE=$(mktemp -d) \
  go run tools/buildcost/runmax.go \
  golangci-lint run --default=none --enable=govet ./...
```
