# Build/lint cost that go-googlesql imposes on downstream consumers

**Scenario.** A project that depends on `go-googlesql` runs golangci-lint (or any
build). To obtain `go-googlesql`'s type information the toolchain must **compile**
the generated wasm2go runtime (`internal/wasm2go`, ~5.3M lines, 40,546 functions)
to export data. That compile is the slow, memory-heavy "full build" the consumer
pays, on the first/cold run, so caches don't save us.

The fix belongs in the **wasm2go** generator (it lowers the WASM from
**googlesql-wasm** to these files). The checked-in `internal/wasm2go/*.go` are
byte-verified release artifacts (`make verify`) and are not hand-edited. This
records what was measured. Reproduction harness + prototypes: `tools/buildcost/`.

## Honest summary

There is **no dramatic source-level lever**. Cosmetic shrinking of the generated
Go (fewer lines/variables/statements) yields only a modest, reproducible
compile-time win and does **not** move whole-program linters. The only lever with
*dramatic* potential is reducing the number of translated functions, which lives
upstream in **googlesql-wasm** (the compile cost is ~linear in function count and
0 % of functions are dead).

## Measured levers

### Deterministic per-package compile (cache-free `go tool compile`, p0 = largest package, 4 reps each, back-to-back)

| Variant | wall | peak RSS |
|---|---|---|
| baseline | ~7.0 s | ~707 MB |
| emitter cleanups (B1) | ~7.2 s | ~644 MB | **RSS −9 %, wall flat** |
| `//go:noinline` only | ~7.0 s | ~698 MB | no change |
| B1 + `//go:noinline` | ~6.95 s | ~644 MB | RSS −9 %, wall flat |

(Per-rep wall spread is ±3 %, so the wall differences are within noise; only the
~9 % peak-memory reduction from B1 is consistent across all reps.)

* **Emitter cleanups (B1)** = drop redundant `_ = vN` blank-uses (keep only
  write-only temps), drop lone `;` after labels (keep only end-of-block),
  copy-propagate single-use adjacent stack temps. Reduces the artifacts 5.35M →
  3.71M lines (−31 %) and 837k → 562k variable objects (−33 %), builds clean,
  full test suite passes — but the **compile** only drops ~9 % in peak memory and
  nothing measurable in wall time, because the type-checker/SSA cost is dominated
  by the operations, which remain.
* **`//go:noinline`** does nothing (the compiler already doesn't inline these
  large, indirectly-called bodies).

### Whole-program full build is noise-dominated
Cold `go build ./internal/wasm2go/...` varied 114–146 s across runs on this
shared box; B1's modest per-package gain washes out under that variance and the
cross-package link/inline cost. So the per-package deterministic number above is
the trustworthy figure: a real but **modest** improvement (memory only).

### Rejected, with measurements
* **Dead-code elimination — not a lever.** 0 % of 40,546 functions are dead:
  11,602 are address-taken into the 18,188-entry indirect-call table and every
  function has cross-package `//go:linkname` declarations. Nothing to drop.
* **Lower the 650k `*(*T)(unsafe.Add(m.M,a))` accesses to helper calls** —
  regresses badly (`go build` >12× slower; whole-program lint 14.5 GB) because
  the compiler then inlines 650k tiny calls.
* **Ship the runtime as a separate module** — doesn't help the cold/first run
  (the compile still happens) and whole-program linters recompile dependency
  source anyway.

## The only dramatic lever: shrink the WASM upstream
Compile cost is ~linear in translated function count (40,546, all live). Reducing
the zetasql/WASM footprint in **googlesql-wasm** — fewer compiled-in features,
stronger size optimization (`wasm-opt -Oz`, `--gc-sections`) — removes whole
functions and scales the entire cost down proportionally. This is the only change
with the potential to *dramatically* cut what downstream consumers pay, and it
lives in the repo the WASM is built from.

## Recommendation (priority order)
1. **Reduce the WASM footprint in googlesql-wasm** — the only lever with
   dramatic, linear payoff on the cold compile downstream consumers pay.
2. **Ship the emitter cleanups (B1)** in the generator — a modest but free
   ~9 % compile peak-memory reduction and 31 % smaller artifacts (no wall-time
   change); no behavior/API change.
3. Do **not** add `//go:noinline` (no effect) or lower memory access to helper
   calls (large regression).
