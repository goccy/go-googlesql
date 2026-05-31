# Shrinking the embedded WASM (`internal/wasm2go/data.bin`)

The wasm2go runtime embeds the **full** googlesql WASM module via `//go:embed
data.bin` (2,906,633 bytes). But wasm2go has *already translated the WASM code
section into Go* (`p0..p10`), so most of the embedded module is dead weight at
runtime. Measured section layout:

| section | bytes | % | needed at runtime? |
|---|---|---|---|
| **code** (id 10) | 2,280,001 | **78.4 %** | **No** — already lowered to Go `FnN` |
| export (id 7) | 266,902 | 9.2 % | No — dispatch is by table index |
| **data** (id 11) | 270,214 | 9.3 % | **Yes** — linear-memory initialisation |
| function (id 3) | 43,342 | 1.5 % | No |
| type/import/table/global/elem/memory/datacount | ~28,000 | ~1 % | metadata only |
| **total** | 2,906,633 | 100 % | |

## Measured reduction (build + full `go test ./...` pass in every case)

Replacing `data.bin` with a section-stripped copy (`tools/buildcost/wasm_strip.go`):

| variant | bytes | % of original | build+tests |
|---|---|---|---|
| full (current) | 2,906,633 | 100 % | pass |
| **strip `code`** (conservative) | 626,632 | **22 % (−78 %)** | pass |
| data section only (floor) | 290,711 | 10 % (−90 %) | pass |

The runtime reads only the **data** segment payloads from the embedded bytes;
the code section (which it would need to *interpret* the WASM) is unused because
the code already exists as compiled Go. Stripping it is safe and verified.

## What it does and doesn't buy

`//go:embed` of a `[]byte` is just bytes — it is **not** type-checked or
compiled. So this is a pure **size** win, not a compile-CPU win:

* `wasm2go` package recompiles in **1.36 s regardless** of `data.bin` size
  (measured, cache-cold, full vs stripped).
* It **does** shrink: the in-repo artifact (−78 % to −90 %), every downstream
  `.a` archive that embeds these bytes, and the final linked binary, by ~2.3 MB.

So it reduces the **footprint** go-googlesql imposes on consumers (repo,
module cache, archives, binaries), not their lint/compile CPU time. (For
compile/lint *time*, the only lever is reducing the translated function count —
i.e. a smaller WASM built upstream in googlesql-wasm; see the build-cost notes.)

## How to reflect it in the generator

When emitting `data.bin`, write **only the sections the runtime consumes** —
minimally the `data` section (and, to keep the file a valid standalone WASM, the
small structural sections), dropping the `code` section (and `export`). The
translator already has the parsed module, so it can re-serialise a slim module
trivially. No change to behaviour, API, or the Go in `p0..p10`.

> Note: this changes `data.bin`, which is covered by `googlesql_wasm2go.sha256`
> and the upstream attestation, so it must be produced by the generator/build,
> not hand-edited in this repo.

## Reproduce
```sh
go run tools/buildcost/wasm_sections.go internal/wasm2go/data.bin     # section map
go run tools/buildcost/wasm_strip.go internal/wasm2go/data.bin /tmp/slim.bin 10   # drop code
# swap /tmp/slim.bin in over a COPY of the repo, then: go build ./... && go test ./...
```
