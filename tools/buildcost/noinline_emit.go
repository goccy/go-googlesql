//go:build ignore
// +build ignore

// noinline_emit: prepend //go:noinline to every translated `func FnN(...) {`.
// REJECTED: measured to have no effect on the compile (the compiler already
// does not inline these large, indirectly-called bodies). Kept to document the
// negative result.
//
//   go run noinline_emit.go internal/wasm2go/p0/p0.go > out.go
package main

import (
	"bufio"
	"os"
	"regexp"
)

var reFunc = regexp.MustCompile(`^func Fn\d+\(.*\{$`)

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for sc.Scan() {
		l := sc.Text()
		if reFunc.MatchString(l) {
			out.WriteString("//go:noinline\n")
		}
		out.WriteString(l)
		out.WriteByte('\n')
	}
}
