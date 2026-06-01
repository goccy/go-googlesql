# Size-reduction experiment on the upstream `googlesql.wasm`

Goal: see whether shrinking the WASM that wasm2go translates (the
`googlesql.wasm` release asset in `goccy/googlesql-wasm`, v0.2.1) reduces the
build/lint cost go-googlesql imposes on downstream consumers.

All numbers below are measured in this session and cross-checked by two
independent tools. `internal/wasm2go/data.bin` in *this* repo is **not** this
WASM — it is the linear-memory image the runtime copies into `m.Memory`
(`copy(m.Memory[33554432:], …)`); the WASM module lives upstream.

## Composition of `googlesql.wasm` (8,930,338 bytes)

| section | bytes | % | translated by wasm2go? |
|---|---|---|---|
| custom (DWARF debug + name/producers) | 4,283,963 | **48.0 %** | **no** |
| code (function bodies) | 3,625,052 | 40.6 % | yes → `p0..p10` |
| data | 270,228 | 3.0 % | yes → `data.bin` |
| function | 44,033 | 0.5 % | (signatures) |
| type | 15,838 | 0.2 % | (signatures) |
| other (import/table/memory/global/elem) | ~691,000 | ~7.7 % | metadata |
| **defined functions** | **32,194** | | → 32k+ `FnN` in the generated Go |

(`go run tools/buildcost/wasm_inspect.go googlesql.wasm` reproduces this.)

## What reduces, and what it buys

### Debug/custom strip — works, but build-cost-neutral
Stripping the custom sections yields **4,646,387 bytes (−48 %)**, verified
identically by `wasm-tools strip` (4,646,387 B) and a hand-rolled section
stripper (4,646,375 B). The module is valid (`wasm-tools validate` passes).

But this does **not** help go-googlesql's build/lint cost: wasm2go translates
only the **code + data** sections into `p0..p10`/`data.bin` and ignores debug
info. This repo doesn't even embed the `.wasm`. So debug-stripping shrinks the
upstream release asset only — the generated Go, and the cost downstream
consumers pay, are unchanged.

### Code/function reduction (`wasm-opt -Oz`) — the real lever, NOT measurable here
Downstream build cost is ~linear in the **function count (32,194)**. Reducing it
needs an optimizer pass (`wasm-opt -Oz`, `--converge`, function merging/DCE).
**This could not be run in this environment:** binaryen `wasm-opt` v119 and v123
both fail to parse the module —

```
[parse exception: Unexpected number of bytes after parsing (at 0:0 / byte 0)]
```

— even after `wasm-tools strip` and a parse/re-emit round-trip. `wasm-tools`
validates the module, so it is well-formed; this is a binaryen feature-support
limitation for this particular build. No `-Oz` numbers are reported because none
were measured.

## Honest conclusion

* Size reduction of the WASM **is** possible (−48 % via debug strip) but is
  **irrelevant to go-googlesql's build/lint cost**.
* The only WASM-side change that would cut that cost is reducing the **translated
  function count**, which requires either:
  1. `wasm-opt -Oz`/function-merging in the **googlesql-wasm build** (with a
     binaryen build that supports this module's features), or
  2. compiling zetasql with **fewer features / smaller footprint** so fewer
     functions exist in the first place.
* Both live upstream in **googlesql-wasm**; neither is reproducible from this
  repo, and I did not fabricate the `-Oz` outcome.

## Reproduce
```sh
curl -fsSLO https://github.com/goccy/googlesql-wasm/releases/download/v0.2.1/googlesql.wasm
go run tools/buildcost/wasm_inspect.go googlesql.wasm      # section map + func count
wasm-tools strip googlesql.wasm -o slim.wasm               # -> 4,646,387 bytes
wasm-tools validate --features all googlesql.wasm          # passes
# wasm-opt --all-features -Oz googlesql.wasm -o opt.wasm   # FAILS in binaryen 119/123
```
