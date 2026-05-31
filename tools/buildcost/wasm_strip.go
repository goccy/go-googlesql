//go:build ignore
// +build ignore

// Strip selected top-level sections from a WASM module.
// Usage: wasmstrip in.wasm out.wasm <drop-id>[,<drop-id>...]
package main

import (
	"encoding/binary"
	"os"
	"strconv"
	"strings"
)

func main() {
	b, _ := os.ReadFile(os.Args[1])
	drop := map[byte]bool{}
	for _, s := range strings.Split(os.Args[3], ",") {
		if s == "" { continue }
		n, _ := strconv.Atoi(s)
		drop[byte(n)] = true
	}
	out := make([]byte, 0, len(b))
	out = append(out, b[:8]...) // magic+version
	p := 8
	for p < len(b) {
		start := p
		id := b[p]; p++
		sz, n := binary.Uvarint(b[p:]); p += n
		end := p + int(sz)
		if !drop[id] {
			out = append(out, b[start:end]...)
		}
		p = end
	}
	os.WriteFile(os.Args[2], out, 0644)
}