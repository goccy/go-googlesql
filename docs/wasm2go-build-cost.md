# Cutting the build/lint cost go-googlesql imposes on downstream consumers

**Scenario.** A project that depends on `go-googlesql` runs golangci-lint (or
any build). To get `go-googlesql`'s type information, the toolchain must
**compile** the generated wasm2go runtime (`internal/wasm2go`, ~5.3M lines) to
export data. That compile is the slow, memory-heavy "full build" the consumer
pays — and it is paid on the first/cold run, so caches don't save us.

The fix belongs in the **wasm2go** generator (it lowers the WASM from
**googlesql-wasm** to these files). The checked-in `internal/wasm2go/*.go` are
byte-verified release artifacts (`make verify`) and are not hand-edited. This
records the generator changes and their measured effect. Reproduction harness +
prototypes: `tools/buildcost/`.

Method: cold GOCACHE (std pre-warmed), idle 4-CPU / 15 GB box, Go 1.25,
`-p=4`. Peak RSS = max single process (`wait4` rusage). Each number is a
back-to-back fresh-cache run.

## The cost, and what it is NOT

A downstream consumer's golangci uses **export data** for `go-googlesql` (it is
an external module to them), so the cost is the **compile**, not a source
type-check. Baseline cold `go build ./internal/wasm2go/...`: **116.9 s /
1.58 GB**.

Two tempting ideas that the measurements **kill**:

* **Dead-code elimination — not a lever.** 0 % of the 40,546 generated functions
  are dead: every one is referenced (11,602 are address-taken into the
  18,188-entry indirect-call table, and all have cross-package `//go:linkname`
  forward declarations). The WASM is fully live; there is nothing to drop.
* **`//go:noinline` on every function — no effect.** 116.7 s / 1.58 GB, i.e.
  identical to baseline. The compiler already isn't inlining these large,
  indirectly-called bodies, so suppressing it changes nothing.
* **Lower the 650k `*(*T)(unsafe.Add(m.M,a))` accesses to helper calls** — a
  large regression (`go build` >12× slower; whole-program lint 14.5 GB) because
  the compiler then inlines 650k tiny calls. Rejected.

## What works: trim what the emitter emits

Three semantics-preserving emitter changes the wasm2go translator can make while
lowering the WASM value stack (it already computes everything needed):

1. **Blank-use lines.** `_ = vN` is emitted after *every* temp to dodge
   "declared and not used". Emit it only for the rare **write-only** temp; every
   read temp doesn't need it.
2. **Empty statements after labels.** `Ln:` is followed by a lone `;`
   unconditionally; only needed when the label ends its block.
3. **Single-use stack temps.** A value pushed and consumed by the next op gets a
   named local; copy-propagate it into its single use and drop the local.

Combined effect on the artifacts (prototypes in `tools/buildcost/`):
lines 5.35M → 3.71M (−31 %), variable objects 837k → 562k (−33 %). Builds clean,
full test suite passes.

Cold `go build ./internal/wasm2go/...` (the downstream compile cost):

| Variant | wall | peak RSS |
|---|---|---|
| Baseline | 116.9 s | 1.58 GB |
| `//go:noinline` only | 116.7 s | 1.58 GB (no change) |
| **Emitter cleanups** | **78.3 s** | **0.86 GB** |

→ **−33 % wall, −46 % peak memory**, cold, no cache — directly reducing the
first/cold build every dependent project pays.

> Note on the lint-the-runtime-directly case: these same cleanups do **not**
> move golangci when it type-checks the runtime *as in-module source* (262 s vs
> 276 s — its cost is in the operations, which remain). That case isn't the
> target scenario; for downstream consumers the cost is the compile above, which
> the cleanups do cut.

### Reflecting it in the generator
Apply the three rules in the emitter:
* emit `_ = vN` only when the temp has zero read uses;
* emit the post-label `;` only when the next emitted line closes the block;
* fold a single-use temp whose value is consumed by the immediately following
  statement into that use.

No behavior, API, or attestation/sha256 change.

## Complementary lever (upstream, linear)
Compile cost is ~linear in the number of translated functions (40,546). The WASM
is already fully live (0 % dead), so further gains must come from **reducing the
WASM footprint in googlesql-wasm** — building zetasql with fewer features /
stronger size optimization (`wasm-opt -Oz`, `--gc-sections`) so fewer functions
are emitted. This scales the whole table down and stacks with the emitter
cleanups.

## Recommendation (priority order)
1. **Ship the emitter cleanups** → −33 % time / −46 % memory on the cold
   dependency build that downstream linters pay. Highest leverage available
   purely within the generator.
2. **Reduce the WASM footprint upstream** for linear, compounding gains.
3. Do **not** add `//go:noinline` (no effect) or lower memory access to helper
   calls (regresses).
