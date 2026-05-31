//go:build ignore
// +build ignore

// Minimal WASM section dumper: prints each top-level section id + size.
package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

var names = map[byte]string{
	0: "custom", 1: "type", 2: "import", 3: "function", 4: "table",
	5: "memory", 6: "global", 7: "export", 8: "start", 9: "elem",
	10: "code", 11: "data", 12: "datacount",
}

func uvarint(b []byte, p int) (uint64, int) {
	v, n := binary.Uvarint(b[p:])
	return v, p + n
}

func main() {
	b, _ := os.ReadFile(os.Args[1])
	if len(b) < 8 || string(b[:4]) != "\x00asm" {
		fmt.Println("not wasm"); return
	}
	p := 8
	total := len(b)
	for p < total {
		id := b[p]; p++
		sz, np := uvarint(b, p); p = np
		extra := ""
		if id == 0 { // custom: read name
			nl, q := uvarint(b, p)
			nm := string(b[q : q+int(nl)])
			extra = " name=" + nm
		}
		fmt.Printf("section %-9s id=%2d size=%10d (%.1f%%)%s\n", names[id], id, sz, 100*float64(sz)/float64(total), extra)
		p += int(sz)
	}
	fmt.Printf("TOTAL %d bytes\n", total)
}