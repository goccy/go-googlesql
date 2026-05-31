//go:build ignore
// +build ignore

// Prototype of a generator-side change applied to already-generated wasm2go code.
// Two transformations, both expressible in the wasm2go translator:
//
//  (1) Drop the redundant "_ = vN" blank-use line for every temp that is READ
//      somewhere in its function. The translator currently emits this line for
//      EVERY temp defensively; it is only required for write-only temps (a call
//      result or value that is never consumed). We keep it only for those.
//
//  (2) Drop the lone ";" empty statement that follows a label when the label is
//      immediately followed by a real statement. The ";" is only needed when the
//      labeled point is the last thing in its block (next line is "}").
//
// Invariants verified on the corpus: locals are all \tvN, single "=" only,
// no op-assign, no multi-assign, no self-referential RHS.
package main

import (
	"bufio"
	"fmt"
	"os"
	"io"
	"regexp"
)

var (
	reFuncOpen = regexp.MustCompile(`^func .*\{$`)
	reDecl     = regexp.MustCompile(`^\t+var (v\d+) `)
	reBlank    = regexp.MustCompile(`^\t+_ = (v\d+)$`)
	reWrite    = regexp.MustCompile(`^\t+(v\d+) = (.*)$`)
	reVar      = regexp.MustCompile(`v\d+`)
	reSemi     = regexp.MustCompile(`^\t+;$`)
	reClose    = regexp.MustCompile(`^\t*\}`)
)

func main() {
	in := os.Args[1]
	data, err := os.ReadFile(in)
	if err != nil { panic(err) }
	var lines []string
	sc := bufio.NewScanner(data2reader(data))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024*16)
	for sc.Scan() { lines = append(lines, sc.Text()) }
	if err := sc.Err(); err != nil { panic(err) }

	keep := make([]bool, len(lines))
	for i := range keep { keep[i] = true }

	// Pass 1: per-function blank-use elimination.
	i := 0
	var droppedBlank, droppedSemi int
	for i < len(lines) {
		if reFuncOpen.MatchString(lines[i]) {
			// find function end (line "}")
			j := i + 1
			depth := 1
			for j < len(lines) && depth > 0 {
				l := lines[j]
				// crude brace depth on trailing/standalone braces is unreliable;
				// rely on top-level "}" at column 0 to end the func.
				if l == "}" { depth = 0; break }
				j++
			}
			// j is the closing "}" index.
			readSet := map[string]bool{}
			blankIdx := map[string][]int{}
			for k := i + 1; k < j; k++ {
				l := lines[k]
				if m := reDecl.FindStringSubmatch(l); m != nil {
					continue
				}
				if m := reBlank.FindStringSubmatch(l); m != nil {
					blankIdx[m[1]] = append(blankIdx[m[1]], k)
					continue
				}
				if m := reWrite.FindStringSubmatch(l); m != nil {
					// LHS m[1] is a write; reads are vars in RHS m[2].
					for _, v := range reVar.FindAllString(m[2], -1) {
						readSet[v] = true
					}
					continue
				}
				for _, v := range reVar.FindAllString(l, -1) {
					readSet[v] = true
				}
			}
			for v, idxs := range blankIdx {
				if readSet[v] {
					for _, k := range idxs { keep[k] = false; droppedBlank++ }
				}
			}
			i = j + 1
			continue
		}
		i++
	}

	// Pass 2: lone ";" after a label, when not last-in-block.
	for k := 0; k < len(lines); k++ {
		if !keep[k] || !reSemi.MatchString(lines[k]) { continue }
		// previous kept line must be a label "Lnn:"
		// find next kept line
		n := k + 1
		for n < len(lines) && !keep[n] { n++ }
		if n < len(lines) && reClose.MatchString(lines[n]) {
			continue // keep ";" (label is last statement in block)
		}
		keep[k] = false
		droppedSemi++
	}

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for k, l := range lines {
		if keep[k] { fmt.Fprintln(out, l) }
	}
	fmt.Fprintf(os.Stderr, "%s: droppedBlank=%d droppedSemi=%d\n", in, droppedBlank, droppedSemi)
}

func data2reader(b []byte) *bytesReader { return &bytesReader{b, 0} }
type bytesReader struct{ b []byte; i int }
func (r *bytesReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) { return 0, io.EOF }
	n := copy(p, r.b[r.i:]); r.i += n; return n, nil
}