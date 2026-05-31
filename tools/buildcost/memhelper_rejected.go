//go:build ignore
// +build ignore

// Generator-emit change prototype (full-file, balance-aware, two-pass):
// lower wasm memory load/store from "*(*T)(unsafe.Add(m.M, ARG))" to helper
// method calls m.L<t>(ARG) / m.S<t>(ARG, VAL). Pass 1 rewrites stores (value
// scanned to end-of-statement, may span lines), pass 2 rewrites all remaining
// loads (rvalues, incl. those nested in multi-line store values).
package main

import (
	"os"
	"strings"
)

var sfx = map[string]string{
	"int32": "I32", "int64": "I64", "float32": "F32", "float64": "F64",
	"int8": "I8", "uint8": "U8", "int16": "I16", "uint16": "U16", "uint32": "U32",
}

func wrapU32(a string) string {
	if strings.HasPrefix(a, "uint32(") { return a }
	return "uint32(" + a + ")"
}

// at position idx (start of "*(*"), parse the memory access; return type
// suffix, ARG, and index just past the closing ')' of the conversion, or ok=false.
func parseAccess(s string, idx int) (suf, arg string, end int, ok bool) {
	rest := s[idx:]
	tEnd := strings.IndexByte(rest, ')')
	if tEnd < 3 { return }
	typ := rest[3:tEnd]
	suf, ok = sfx[typ]
	if !ok { return }
	marker := "*(*" + typ + ")(unsafe.Add(m.M, "
	if !strings.HasPrefix(rest, marker) { ok = false; return }
	addOpen := idx + len("*(*"+typ+")(") + len("unsafe.Add")
	// matchParen
	depth := 0; addClose := -1
	for i := addOpen; i < len(s); i++ {
		if s[i] == '(' { depth++ } else if s[i] == ')' { depth--; if depth == 0 { addClose = i; break } }
	}
	if addClose < 0 || addClose+1 >= len(s) || s[addClose+1] != ')' { ok = false; return }
	arg = s[idx+len(marker) : addClose]
	end = addClose + 2
	return
}

// scan VAL from valStart to end-of-statement (newline at bracket depth 0).
func valEnd(s string, valStart int) int {
	depth := 0
	for i := valStart; i < len(s); i++ {
		switch s[i] {
		case '(', '[', '{': depth++
		case ')', ']', '}': depth--
		case '\n':
			if depth <= 0 { return i }
		}
	}
	return len(s)
}

func lineStartIsTabsOnly(s string, idx int) bool {
	j := strings.LastIndexByte(s[:idx], '\n')
	return strings.Trim(s[j+1:idx], "\t") == ""
}

func main() {
	data, _ := os.ReadFile(os.Args[1])
	s := string(data)

	// Pass 1: stores.
	var b strings.Builder
	i := 0
	for {
		k := strings.Index(s[i:], "*(*")
		if k < 0 { b.WriteString(s[i:]); break }
		idx := i + k
		suf, arg, end, ok := parseAccess(s, idx)
		if ok && lineStartIsTabsOnly(s, idx) && strings.HasPrefix(s[end:], " = ") {
			valStart := end + len(" = ")
			ve := valEnd(s, valStart)
			val := s[valStart:ve]
			b.WriteString(s[i:idx])
			b.WriteString("m.S" + suf + "(" + wrapU32(arg) + ", " + val + ")")
			i = ve
			continue
		}
		// not a store here; emit up to idx+3 and continue
		b.WriteString(s[i : idx+3])
		i = idx + 3
	}
	s = b.String()

	// Pass 2: loads (all remaining accesses).
	var c strings.Builder
	i = 0
	for {
		k := strings.Index(s[i:], "*(*")
		if k < 0 { c.WriteString(s[i:]); break }
		idx := i + k
		suf, arg, end, ok := parseAccess(s, idx)
		if ok {
			c.WriteString(s[i:idx])
			c.WriteString("m.L" + suf + "(" + wrapU32(arg) + ")")
			i = end
			continue
		}
		c.WriteString(s[i : idx+3])
		i = idx + 3
	}
	os.Stdout.WriteString(c.String())
}