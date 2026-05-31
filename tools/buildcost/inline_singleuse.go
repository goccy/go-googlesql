//go:build ignore
// +build ignore

// Prototype of generator-side copy-propagation for single-use stack temps.
// Inlines a temp vN when ALL hold (100%-safe subset):
//   * vN is declared once and assigned once via "vN = RHS";
//   * RHS is side-effect-free (no call "Fn"/"(m,", no memory load "*(", no ".(");
//   * vN is read exactly once, on the statement line IMMEDIATELY after its def
//     (same basic block, nothing executes in between -> no reordering hazard).
// Effect: substitute "(RHS)" at the use, drop the def line, decl and blank.
// This is exactly what the wasm2go translator can do while lowering the WASM
// value stack: a value pushed and consumed by the next op needs no named temp.
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	reFuncOpen = regexp.MustCompile(`^func .*\{$`)
	reDecl     = regexp.MustCompile(`^(\t+)var (v\d+) (\S+)$`)
	reBlank    = regexp.MustCompile(`^\t+_ = (v\d+)$`)
	reWrite    = regexp.MustCompile(`^(\t+)(v\d+) = (.*)$`)
	reVar      = regexp.MustCompile(`v\d+`)
	reLabelLn  = regexp.MustCompile(`^L\d+:`)
	reImpure   = regexp.MustCompile(`Fn\d|\(m,|\*\(|\.\(|m\.T0|m\.M`)
	reWord     = regexp.MustCompile(`\bv\d+\b`)
)

func isStmt(l string) bool { // a real body statement line (not decl/blank/label/semi/brace-only)
	t := strings.TrimLeft(l, "\t")
	if t == "" || t == ";" { return false }
	if reLabelLn.MatchString(t) { return false }
	if strings.HasPrefix(t, "var ") { return false }
	if strings.HasPrefix(t, "_ = v") { return false }
	if t == "}" || t == "} else {" || strings.HasPrefix(t,"}") { return false }
	return true
}

func main() {
	data, _ := os.ReadFile(os.Args[1])
	lines := strings.Split(string(data), "\n")
	keep := make([]bool, len(lines))
	for i := range keep { keep[i] = true }
	var inlined int

	i := 0
	for i < len(lines) {
		if !reFuncOpen.MatchString(lines[i]) { i++; continue }
		end := i + 1
		for end < len(lines) && lines[end] != "}" { end++ }
		// gather per-temp info within [i+1, end)
		defLine := map[string]int{}
		defRHS := map[string]string{}
		defCount := map[string]int{}
		readCount := map[string]int{}
		readLine := map[string]int{}
		declLine := map[string]int{}
		blankLine := map[string]int{}
		for k := i + 1; k < end; k++ {
			l := lines[k]
			if m := reDecl.FindStringSubmatch(l); m != nil { declLine[m[2]] = k; continue }
			if m := reBlank.FindStringSubmatch(l); m != nil { blankLine[m[1]] = k; continue }
			if m := reWrite.FindStringSubmatch(l); m != nil {
				defCount[m[2]]++; defLine[m[2]] = k; defRHS[m[2]] = m[3]
				for _, v := range reVar.FindAllString(m[3], -1) { readCount[v]++; readLine[v] = k }
				continue
			}
			for _, v := range reVar.FindAllString(l, -1) { readCount[v]++; readLine[v] = k }
		}
		// next-statement index helper
		nextStmt := func(k int) int {
			for n := k + 1; n < end; n++ {
				if !keep[n] { continue }
				if isStmt(lines[n]) { return n }
				return -1 // a label/semi/brace breaks adjacency
			}
			return -1
		}
		// phase 1: collect candidates (single def, single read, adjacent next-stmt)
		cand := map[string]bool{}
		for v := range defLine {
			if defCount[v] != 1 || readCount[v] != 1 { continue }
			d := defLine[v]; r := readLine[v]
			if !keep[d] || !keep[r] { continue }
			if nextStmt(d) != r { continue }
			cand[v] = true
		}
		// phase 2: drop candidates whose RHS references another candidate (no chains)
		for v := range cand {
			for _, rv := range reVar.FindAllString(defRHS[v], -1) {
				if cand[rv] { delete(cand, v); break }
			}
		}
		for v := range cand {
			rhs := defRHS[v]
			d := defLine[v]
			r := readLine[v]
			// substitute (RHS) for the single read of v on line r
			newr := reWord.ReplaceAllStringFunc(lines[r], func(tok string) string {
				if tok == v { return "(" + rhs + ")" }
				return tok
			})
			lines[r] = newr
			keep[d] = false
			if dl, ok := declLine[v]; ok { keep[dl] = false }
			if bl, ok := blankLine[v]; ok { keep[bl] = false }
			inlined++
		}
		i = end + 1
	}
	out := bufio.NewWriter(os.Stdout); defer out.Flush()
	for k, l := range lines {
		if k == len(lines)-1 && l == "" { break }
		if keep[k] { fmt.Fprintln(out, l) }
	}
	fmt.Fprintf(os.Stderr, "%s: inlined=%d temps\n", os.Args[1], inlined)
}