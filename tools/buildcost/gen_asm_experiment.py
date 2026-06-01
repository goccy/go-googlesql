# gen_asm_experiment.py — generate two equivalent packages (Go source vs Go
# stubs + Plan 9 .s) of N functions x 30 wasm-like ops, to measure the
# golangci-lint / build cost of moving function bodies out of type-checked Go.
# See docs/wasm-plan9asm-experiment.md.
#   python3 gen_asm_experiment.py 4000
import os, sys
N = int(sys.argv[1]) if len(sys.argv) > 1 else 4000
STMTS = 30

root = "/tmp/asmexp"
os.makedirs(root, exist_ok=True)
open(f"{root}/go.mod","w").write("module asmexp\n\ngo 1.25.0\n")

# Go-source variant (realistic wasm-like bodies: loads/stores/branches/arith)
os.makedirs(f"{root}/gopkg", exist_ok=True)
with open(f"{root}/gopkg/gopkg.go","w") as f:
    f.write("package gopkg\n\nimport \"unsafe\"\n\nvar Mem = make([]byte, 1<<20)\nvar M = unsafe.Pointer(&Mem[0])\n\n")
    for i in range(N):
        f.write(f"func Fn{i}(l0 int32) int32 {{\n")
        for v in range(STMTS): f.write(f"\tvar v{v} int32\n")
        f.write("\tv0 = l0 & int32(0xffff)\n")
        for v in range(1, STMTS):
            op = v % 5
            if op == 0:   f.write(f"\tv{v} = *(*int32)(unsafe.Add(M, uint32(v{v-1})&0xffff))\n")
            elif op == 1: f.write(f"\t*(*int32)(unsafe.Add(M, uint32(v{v-1})&0xffff)) = v{v-1}\n\tv{v} = v{v-1} + {v}\n")
            elif op == 2: f.write(f"\tv{v} = v{v-1} + int32({v})\n")
            elif op == 3: f.write(f"\tif v{v-1} != 0 {{\n\t\tv{v} = v{v-1} - 1\n\t}} else {{\n\t\tv{v} = {v}\n\t}}\n")
            else:         f.write(f"\tv{v} = v{v-1} | int32({v})\n")
        f.write(f"\treturn v{STMTS-1}\n}}\n\n")

# ASM variant: Go stubs + trivially-correct amd64 .s (pure register arithmetic)
os.makedirs(f"{root}/asmpkg", exist_ok=True)
with open(f"{root}/asmpkg/stubs.go","w") as f:
    f.write("package asmpkg\n\n")
    for i in range(N): f.write(f"func Fn{i}(l0 int32) int32\n")
with open(f"{root}/asmpkg/body_amd64.s","w") as f:
    f.write("#include \"textflag.h\"\n\n")
    for i in range(N):
        f.write(f"TEXT ·Fn{i}(SB), NOSPLIT, $0-16\n")
        f.write("\tMOVL l0+0(FP), AX\n")
        for v in range(1, STMTS):  # same instruction count, all safe reg ops
            op = v % 3
            if op == 0:   f.write(f"\tADDL ${v}, AX\n")
            elif op == 1: f.write(f"\tORL ${v}, AX\n")
            else:         f.write("\tSHLL $1, AX\n")
        f.write("\tMOVL AX, ret+8(FP)\n\tRET\n\n")

gl = sum(1 for _ in open(f"{root}/gopkg/gopkg.go"))
sl = sum(1 for _ in open(f"{root}/asmpkg/body_amd64.s"))
print(f"N={N} gopkg.go_lines={gl} body.s_lines={sl}")
