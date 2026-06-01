# Size-reduction experiment on the upstream `googlesql.wasm`

Goal: see whether shrinking the WASM that wasm2go translates (the
`googlesql.wasm` release asset in `goccy/googlesql-wasm`, v0.2.1) reduces the
build/lint cost go-googlesql imposes on downstream consumers.

All numbers below were measured in this session and cross-checked with
`wasm-tools` (1.220.0) and `binaryen wasm-opt` (v123). Note:
`internal/wasm2go/data.bin` in *this* repo is **not** this WASM — it is the
linear-memory image the runtime copies into `m.Memory`
(`copy(m.Memory[33554432:], …)`). The WASM module itself lives upstream.

## Composition of `googlesql.wasm`

* size: **13,859,545 bytes**
* sha256: `edf20a34cd2b5a3c4c831181dc7fcd75c5f0c0cde531ce2cbb132e4fb4d57aae`
* `wasm-tools validate --features all`: **PASS**

| section | bytes | % | translated by wasm2go? |
|---|---|---|---|
| **code** (function bodies) | 10,840,400 | **78.2 %** | yes → `p0..p10` |
| **data** | 2,605,866 | 18.8 % | yes → `data.bin` memory image |
| export | 317,297 | 2.3 % | no (dispatch is by table index) |
| elem | 52,079 | 0.4 % | → function table |
| function (signatures) | 40,574 | 0.3 % | (signatures) |
| type / import / table / memory / global / datacount | ~3,300 | <0.1 % | metadata |

* **defined functions: 40,546** (authoritative: `wasm-tools objdump`,
  "40546 count" for both the function and code sections). This matches the
  40,546 `FnN` bodies in the generated Go exactly — the WASM functions map 1:1
  to `p0..p10`, so the function count is the lever for the generated-Go size.
* There is **no custom/DWARF/debug section**: `wasm-tools strip` returns a
  byte-identical 13,859,545-byte file. So there is nothing to gain from
  debug-stripping — this build is already debug-free.

## Optimizer results (measured, with function counts)

Function count via `wasm-tools objdump … | grep '^ code'` ("<N> count");
code-section bytes likewise.

| variant | total size | Δ size | code-section bytes | **function count** |
|---|---|---|---|---|
| original | 13,859,545 | — | 10,840,400 | **40,546** |
| `wasm-tools strip` | 13,859,545 | 0 (no debug) | 10,840,400 | 40,546 |
| `wasm-opt -O2` | 13,917,650 | +0.4 % | — | 40,546 |
| `wasm-opt -Oz` | 11,931,847 | **−13.9 %** | 8,893,983 (**−18.0 %**) | **41,531 (+2.4 %)** |
| `wasm-opt -Oz --converge` | 11,929,964 | −13.9 % | — | 41,487 |

## The decisive finding

`-Oz` shrinks the module by 13.9 % (code section −18.0 %), but it **does not
reduce the function count — it slightly *increases* it** (40,546 → 41,531).
binaryen at `-Oz` shrinks code by splitting/outlining shared sequences into
*new* helper functions, trading more functions for fewer total bytes.

For go-googlesql that is the wrong direction:

* wasm2go emits **one Go `FnN` per WASM function**, so 41,531 functions would
  generate *more* `FnN` definitions, not fewer.
* This repo's build/lint cost is ~linear in **function count**, not in WASM byte
  size (the `.wasm` isn't even shipped here — only the translated Go and the
  `data.bin` memory image are).

So **`wasm-opt -Oz` would not reduce — and would likely slightly increase — the
downstream build/lint cost**, despite making the WASM smaller. It is not a useful
lever for this goal.

## Honest conclusion

* The WASM has **no debug bloat** (`wasm-tools strip` is a no-op) and is already
  validated well-formed.
* `wasm-opt -Oz` reduces WASM **size** (−13.9 %) but **raises function count**
  (+2.4 %); `-O2` helps neither. Because the generated-Go cost tracks function
  count, **no `wasm-opt` pass here reduces go-googlesql's build/lint cost** —
  measured, not assumed.
* The only thing that would cut the cost is **fewer WASM functions**, which
  binaryen does not give. That requires building zetasql with a smaller
  feature/footprint upstream in **googlesql-wasm** so fewer functions exist in
  the first place — an optimizer pass cannot manufacture that.

## Reproduce
```sh
curl -fsSLO https://github.com/goccy/googlesql-wasm/releases/download/v0.2.1/googlesql.wasm
go run tools/buildcost/wasm_inspect.go googlesql.wasm        # section map + func count
wasm-tools validate --features all googlesql.wasm            # PASS
wasm-tools strip googlesql.wasm -o /tmp/s.wasm               # no-op (no debug)
wasm-opt --all-features -Oz googlesql.wasm -o /tmp/oz.wasm   # 11,931,847 bytes (-13.9%)
wasm-tools objdump /tmp/oz.wasm | grep -E '^\s*code '        # 41531 count (+2.4%, WRONG direction)
```
