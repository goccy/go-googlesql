# Cutting the golangci-lint / build cost of the generated wasm2go runtime

The generated runtime in `internal/wasm2go/{base,p0..p10,wasm2go.go}` is ~5.3M
lines / ~100 MB. Running any type-aware golangci-lint linter forces a full
type-check of it and is slow + memory-hungry.

The fix belongs in the **wasm2go** generator (it lowers the WASM in
**googlesql-wasm** to these files). The files checked in here are byte-verified
release artifacts (`make verify`) and are not hand-edited. This document records
what was measured and what the generator should change.

All numbers are **cold GOCACHE unless stated**, std pre-warmed, on a 4-CPU /
15 GB box, `golangci-lint v2.5.0`, `--default=none --enable=govet ./...`.
Peak RSS = max single process (`wait4` rusage). Reproduction harness +
prototypes: `tools/buildcost/`.

## Where the cost is

| Workload | wall | peak RSS |
|---|---|---|
| golangci-lint govet `./...` (in-module, cold) | 276 s | 5.6 GB |
| golangci-lint govet `.` root pkg only (in-module, cold) | 240 s | 5.8 GB |
| `go build ./internal/wasm2go/...` (compile incl. codegen, cold) | 114 s | 1.6 GB |

* golangci-lint **type-checks the whole in-module source graph** — linting only
  the root package costs the same, so narrowing the lint scope does nothing.
* The cost is `go/types` over the source, not codegen.

## What does NOT work (measured, so we don't repeat it)

| Approach | result vs baseline 276 s / 5.6 GB |
|---|---|
| **Emitter cleanups** — `_ = vN` only for write-only temps; drop `;` after labels except end-of-block; copy-propagate single-use stack temps. −31% lines (5.35M→3.71M), −33% variable objects (837k→562k), builds + tests pass. | **262 s / 5.9 GB — within noise.** Helps `go build` ~8%/7%, but `go/types`' cost is in the *operations*, which remain. |
| **Memory-helper calls** — lower `*(*int32)(unsafe.Add(m.M, a))` (650,194 sites) to `m.LI32(a)` / `m.SI32(a,v)`. | **265 s / 14.5 GB ✗** and `go build` >12× slower — the compiler inlines 650k tiny calls. Rejected. |
| **Narrow lint scope** to root package only. | 240 s / 5.8 GB — no real change (deps still type-checked from source). |

Source-level shrinking of the generated code does **not** move golangci-lint,
and the obvious "fewer expressions" idea backfires.

## What works: ship the runtime as its own module

golangci-lint re-type-checks **in-module** source on every run, but consumes an
**external module dependency** through its compiled **export data** — it never
type-checks that dependency's source. So move the runtime out of the main module.

Measured with the runtime relocated to a separate module
`github.com/goccy/go-googlesql/internal/wasm2go` (served via a file proxy so it
lands in the module cache as a normal dependency), main module unchanged:

| Configuration (warm build cache — normal dev) | wall | peak RSS |
|---|---|---|
| Runtime **in-module** (current) | 120 s | 5.5 GB |
| Runtime **external module** (export data) | **55 s** | **3.0 GB** |

→ **2.2× faster, 1.8× less memory**, every lint. The expensive compile of the
runtime happens **once per version** (cacheable, 114 s / 1.6 GB via parallel
`go build`) instead of an in-process 5.5 GB type-check on every lint. Cold, the
two are comparable (236 s vs 276 s); the win is the steady state, because the
runtime's export data is a legitimate immutable artifact (this is exactly why
linting a normal project doesn't re-type-check stdlib / large deps from source).

The remaining 3.0 GB is type-checking the in-module public binding
(`googlesql.go`, ~340k lines); externalizing the runtime removes the 5.3M-line
part entirely.

### How to reflect it in the generator
The wasm2go generator already emits a self-contained package tree
(`base`, `p0..p10`, `wasm2go.go`). Have it additionally emit a `go.mod` and
publish that tree as a **versioned module** (e.g. from the googlesql-wasm
release, or a dedicated `go-googlesql-runtime` module). `go-googlesql` then
`require`s it instead of vendoring the source in-tree. No change to the
generated Go itself; the attestation/sha256 flow is unaffected.

> A local `replace => ./dir` does **not** achieve this — go/packages treats a
> replaced directory as editable workspace source and golangci-lint type-checks
> it. It must be a real cached module (require + proxy/VCS, no path replace).

## Complementary, independent levers
* **Shrink the WASM upstream (googlesql-wasm).** golangci cost is ~linear in the
  number of translated functions (32,558 today). Building zetasql-wasm with dead
  -code elimination (`--gc-sections`, `wasm-opt -Oz`, only required features)
  removes whole functions → proportionally less Go to type-check. Root-cause and
  linear; stacks with the module split.
* **Emitter cleanups** (the −31%/−33% above): worth shipping for smaller
  artifacts and faster `go build`/CI even though they don't move golangci.

## Recommendation (priority order)
1. **Publish the wasm2go runtime as a separate versioned module** → 2.2× faster,
   1.8× less golangci memory in steady state; compile once per version.
2. **Reduce the WASM binary's function count** in googlesql-wasm (dead-code
   elimination) → linear reduction that compounds with (1).
3. Land the **emitter cleanups** as a free size/compile win.
4. Do **not** lower memory access to helper calls (B2) — it regresses badly.
