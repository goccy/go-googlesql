package base

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"math"
	"math/bits"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type EnvImports interface {
	X_ZNK9googlesql12ASTIntoAlias13GetAsIdStringEv(m *Module, l0 int32) int32
	X__cxa_allocate_exception(m *Module, l0 int32) int32
	X__cxa_throw(m *Module, l0 int32, l1 int32, l2 int32)
	X_ZN4absl12lts_2024072216raw_log_internal6RawLogENS0_11LogSeverityEPKciS4_z(m *Module, l0 int32, l1 int32, l2 int32, l3 int32, l4 int32)
	X__cxa_thread_atexit(m *Module, l0 int32, l1 int32, l2 int32) int32
	U_isUWhiteSpace_76(m *Module, l0 int32) int32
	X_ZN4absl12lts_2024072213GetStackTraceEPPvii(m *Module, l0 int32, l1 int32, l2 int32) int32
	X_ZN4absl12lts_2024072212log_internal8TimeZoneEv(m *Module) int32
	X_ZN4absl12lts_2024072212log_internal13IsInitializedEv(m *Module) int32
	X_ZN4absl12lts_2024072212log_internal24MaxFramesInLogStackTraceEv(m *Module) int32
	X_ZN4absl12lts_2024072212log_internal28ShouldSymbolizeLogStackTraceEv(m *Module) int32
	X_ZN4absl12lts_202407229SymbolizeEPKvPci(m *Module, l0 int32, l1 int32, l2 int32) int32
	X_ZN4absl12lts_2024072212log_internal12ExitOnDFatalEv(m *Module) int32
	X_ZN4absl12lts_2024072212log_internal24SetSuppressSigabortTraceEb(m *Module, l0 int32) int32
	X_ZN4absl12lts_2024072212log_internal13WriteToStderrENSt3__217basic_string_viewIcNS2_11char_traitsIcEEEENS0_11LogSeverityE(m *Module, l0 int32, l1 int32)
	X_ZN6icu_769ErrorCodeD1Ev(m *Module, l0 int32) int32
	X_ZNK6icu_769ErrorCode9errorNameEv(m *Module, l0 int32) int32
	X_ZN6icu_768ByteSinkD2Ev(m *Module, l0 int32) int32
	Utf8_prevCharSafeBody_76(m *Module, l0 int32, l1 int32, l2 int32, l3 int32, l4 int32) int32
	X_ZN6icu_767UMemorydlEPv(m *Module, l0 int32)
	X_ZN6icu_768ByteSink15GetAppendBufferEiiPciPi(m *Module, l0 int32, l1 int32, l2 int32, l3 int32, l4 int32, l5 int32) int32
	X_ZN6icu_768ByteSink5FlushEv(m *Module, l0 int32)
	X_ZN4absl12lts_2024072213time_internal4cctz12TimeZoneLibC4MakeERKNSt3__212basic_stringIcNS4_11char_traitsIcEENS4_9allocatorIcEEEE(m *Module, l0 int32) int32
	X_ZN6icu_7611Normalizer223getNFKCCasefoldInstanceER10UErrorCode(m *Module, l0 int32) int32
	Utf8_back1SafeBody_76(m *Module, l0 int32, l1 int32, l2 int32) int32
}
type WasmifyImports interface {
	Callback_invoke(m *Module, l0 int32, l1 int32, l2 int32, l3 int32) int64
}
type Wasi_snapshot_preview1Imports interface {
	Environ_get(m *Module, l0 int32, l1 int32) int32
	Environ_sizes_get(m *Module, l0 int32, l1 int32) int32
	Clock_time_get(m *Module, l0 int32, l1 int64, l2 int32) int32
	Fd_close(m *Module, l0 int32) int32
	Fd_fdstat_get(m *Module, l0 int32, l1 int32) int32
	Fd_fdstat_set_flags(m *Module, l0 int32, l1 int32) int32
	Fd_prestat_get(m *Module, l0 int32, l1 int32) int32
	Fd_prestat_dir_name(m *Module, l0 int32, l1 int32, l2 int32) int32
	Fd_read(m *Module, l0 int32, l1 int32, l2 int32, l3 int32) int32
	Fd_seek(m *Module, l0 int32, l1 int64, l2 int32, l3 int32) int32
	Fd_write(m *Module, l0 int32, l1 int32, l2 int32, l3 int32) int32
	Path_open(m *Module, l0 int32, l1 int32, l2 int32, l3 int32, l4 int32, l5 int64, l6 int64, l7 int32, l8 int32) int32
	Poll_oneoff(m *Module, l0 int32, l1 int32, l2 int32, l3 int32) int32
	Proc_exit(m *Module, l0 int32)
	Sched_yield(m *Module) int32
}
type Module struct {
	Memory                 []byte
	MaxMem                 uint64
	M                      unsafe.Pointer
	T0                     []any
	G0                     int32
	Env                    EnvImports
	Wasmify                WasmifyImports
	Wasi_snapshot_preview1 Wasi_snapshot_preview1Imports
}

func I32(x int32) int32 { return x }

func I64(x int64) int64 { return x }

// ui32 / ui64 reinterpret a signed integer as its unsigned bit
// equivalent at runtime. Used for the operands of wasm unsigned
// comparisons (i32.lt_u etc.) — emitting `uint32(int32(-N))` directly
// fails Go's compile-time constant rule because the negative typed
// constant isn't representable in uint32; routing through these
// function-call boundaries forces runtime conversion.
func Ui32(x int32) uint32 { return uint32(x) }

func Ui64(x int64) uint64 { return uint64(x) }

func F32(x float32) float32 { runtime.KeepAlive(&x); return x }

func F64(x float64) float64 { runtime.KeepAlive(&x); return x }

func I32_div_s(x, y int32) int32 {
	if y == -1 && x == math.MinInt32 {
		panic("wasm: integer overflow")
	}
	if y == 0 {
		panic("wasm: integer divide by zero")
	}
	return x / y
}

func I64_div_s(x, y int64) int64 {
	if y == -1 && x == math.MinInt64 {
		panic("wasm: integer overflow")
	}
	if y == 0 {
		panic("wasm: integer divide by zero")
	}
	return x / y
}

func I32_div_u(x, y uint32) uint32 {
	if y == 0 {
		panic("wasm: integer divide by zero")
	}
	return x / y
}

func I64_div_u(x, y uint64) uint64 {
	if y == 0 {
		panic("wasm: integer divide by zero")
	}
	return x / y
}

func I32_rem_s(x, y int32) int32 {
	if y == 0 {
		panic("wasm: integer divide by zero")
	}
	if y == -1 {
		return 0
	}
	return x % y
}

func I64_rem_s(x, y int64) int64 {
	if y == 0 {
		panic("wasm: integer divide by zero")
	}
	if y == -1 {
		return 0
	}
	return x % y
}

func I32_rem_u(x, y uint32) uint32 {
	if y == 0 {
		panic("wasm: integer divide by zero")
	}
	return x % y
}

func I64_rem_u(x, y uint64) uint64 {
	if y == 0 {
		panic("wasm: integer divide by zero")
	}
	return x % y
}

func I32_rotl(x, y int32) int32 { return int32(bits.RotateLeft32(uint32(x), int(y&31))) }

func I32_rotr(x, y int32) int32 { return int32(bits.RotateLeft32(uint32(x), -int(y&31))) }

func I64_rotl(x, y int64) int64 { return int64(bits.RotateLeft64(uint64(x), int(y&63))) }

func F32_abs(x float32) float32 { return math.Float32frombits(math.Float32bits(x) &^ (1 << 31)) }

func F64_abs(x float64) float64 { return math.Float64frombits(math.Float64bits(x) &^ (1 << 63)) }

func F32_neg(x float32) float32 { return math.Float32frombits(math.Float32bits(x) ^ (1 << 31)) }

func F64_neg(x float64) float64 { return math.Float64frombits(math.Float64bits(x) ^ (1 << 63)) }

func F64_copysign(x, y float64) float64 { return math.Copysign(x, y) }

func I32_trunc_sat_f32_s(x float32) int32 {
	if x != x {
		return 0
	}
	if x <= -2147483648.0 {
		return math.MinInt32
	}
	if x >= 2147483648.0 {
		return math.MaxInt32
	}
	return int32(x)
}

func I32_trunc_sat_f32_u(x float32) int32 {
	if x != x || x <= 0 {
		return 0
	}
	if x >= 4294967296.0 {
		return -1
	}
	return int32(uint32(x))
}

func I32_trunc_sat_f64_s(x float64) int32 {
	if x != x {
		return 0
	}
	if x <= -2147483648.0 {
		return math.MinInt32
	}
	if x >= 2147483648.0 {
		return math.MaxInt32
	}
	return int32(x)
}

func I32_trunc_sat_f64_u(x float64) int32 {
	if x != x || x <= 0 {
		return 0
	}
	if x >= 4294967296.0 {
		return -1
	}
	return int32(uint32(x))
}

func I64_trunc_sat_f32_s(x float32) int64 {
	if x != x {
		return 0
	}
	if float64(x) <= -9223372036854775808.0 {
		return math.MinInt64
	}
	if float64(x) >= 9223372036854775808.0 {
		return math.MaxInt64
	}
	return int64(x)
}

func I64_trunc_sat_f32_u(x float32) int64 {
	if x != x || x <= 0 {
		return 0
	}
	if float64(x) >= 18446744073709551616.0 {
		return -1
	}
	return int64(uint64(x))
}

func I64_trunc_sat_f64_s(x float64) int64 {
	if x != x {
		return 0
	}
	if x <= -9223372036854775808.0 {
		return math.MinInt64
	}
	if x >= 9223372036854775808.0 {
		return math.MaxInt64
	}
	return int64(x)
}

func I64_trunc_sat_f64_u(x float64) int64 {
	if x != x || x <= 0 {
		return 0
	}
	if x >= 18446744073709551616.0 {
		return -1
	}
	return int64(uint64(x))
}

// memorySize returns the current size of m.memory in wasm pages (each
// page is 64 KiB).
func MemorySize(m *Module) int32 { return int32(len(m.Memory) >> 16) }

// memoryGrow grows m.memory by n wasm pages (64 KiB each). Returns the
// previous page count, or -1 if the new size would exceed maxMem. n may be 0,
// which simply returns the current size.
//
// len(m.memory) must always equal the exact wasm memory size (memory.size
// and every bounds check depend on it), but the backing array is grown
// GEOMETRICALLY: a sequence of small memory.grow calls — which a C++ heap
// does constantly during start-up — would otherwise reallocate and recopy
// the whole linear memory on every page, i.e. O(n^2) total copying. Spare
// capacity makes the common grow a zero-copy reslice and amortizes the
// reallocations to O(n).
func MemoryGrow(m *Module, n int32) int32 {
	prev := int32(len(m.Memory) >> 16)
	if n == 0 {
		return prev
	}
	if n < 0 {
		return -1
	}

	want := uint64(len(m.Memory)) + uint64(n)*65536
	if m.MaxMem != 0 && want > m.MaxMem {
		return -1
	}
	if want > 1<<32 {
		return -1
	}
	if want <= uint64(cap(m.Memory)) {

		m.Memory = m.Memory[:want]
		return prev
	}

	newCap := uint64(cap(m.Memory)) * 2
	if newCap < want {
		newCap = want
	}
	if m.MaxMem != 0 && newCap > m.MaxMem {
		newCap = m.MaxMem
	}
	if newCap > 1<<32 {
		newCap = 1 << 32
	}

	grown := make([]byte, want, newCap)
	copy(grown, m.Memory)
	m.Memory = grown

	m.M = unsafe.Pointer(unsafe.SliceData(m.Memory))
	return prev
}

func I32_div_u_s(x, y int32) int32 { return int32(I32_div_u(uint32(x), uint32(y))) }
func I32_rem_u_s(x, y int32) int32 { return int32(I32_rem_u(uint32(x), uint32(y))) }
func I64_div_u_s(x, y int64) int64 { return int64(I64_div_u(uint64(x), uint64(y))) }
func I64_rem_u_s(x, y int64) int64 { return int64(I64_rem_u(uint64(x), uint64(y))) }

func F32_add(x, y float32) float32 { return x + y }
func F32_sub(x, y float32) float32 { return x - y }
func F32_mul(x, y float32) float32 { return x * y }
func F32_div(x, y float32) float32 { return x / y }
func F64_add(x, y float64) float64 { return x + y }
func F64_sub(x, y float64) float64 { return x - y }
func F64_mul(x, y float64) float64 { return x * y }
func F64_div(x, y float64) float64 { return x / y }

func I32_eqz(x int32) int32 {
	if x == 0 {
		return 1
	}
	return 0
}

func I64_eqz(x int64) int32 {
	if x == 0 {
		return 1
	}
	return 0
}

func I32_clz(x int32) int32    { return int32(bits.LeadingZeros32(uint32(x))) }
func I32_ctz(x int32) int32    { return int32(bits.TrailingZeros32(uint32(x))) }
func I32_popcnt(x int32) int32 { return int32(bits.OnesCount32(uint32(x))) }

func I64_clz(x int64) int64 { return int64(bits.LeadingZeros64(uint64(x))) }
func I64_ctz(x int64) int64 { return int64(bits.TrailingZeros64(uint64(x))) }

func F32_ceil(x float32) float32 { return float32(math.Ceil(float64(x))) }
func F64_ceil(x float64) float64 { return math.Ceil(x) }

func F64_trunc(x float64) float64 { return math.Trunc(x) }

func F32_eq(x, y float32) int32 {
	if x == y {
		return 1
	}
	return 0
}

func F32_ne(x, y float32) int32 {
	if x != y {
		return 1
	}
	return 0
}

func F32_lt(x, y float32) int32 {
	if x < y {
		return 1
	}
	return 0
}

func F32_gt(x, y float32) int32 {
	if x > y {
		return 1
	}
	return 0
}

func F32_le(x, y float32) int32 {
	if x <= y {
		return 1
	}
	return 0
}

func F32_ge(x, y float32) int32 {
	if x >= y {
		return 1
	}
	return 0
}

func F64_eq(x, y float64) int32 {
	if x == y {
		return 1
	}
	return 0
}

func F64_ne(x, y float64) int32 {
	if x != y {
		return 1
	}
	return 0
}

func F64_lt(x, y float64) int32 {
	if x < y {
		return 1
	}
	return 0
}

func F64_gt(x, y float64) int32 {
	if x > y {
		return 1
	}
	return 0
}

func F64_le(x, y float64) int32 {
	if x <= y {
		return 1
	}
	return 0
}

func F64_ge(x, y float64) int32 {
	if x >= y {
		return 1
	}
	return 0
}

func I32_wrap_i64(x int64) int32       { return int32(x) }
func I64_extend_i32_s(x int32) int64   { return int64(x) }
func I64_extend_i32_u(x int32) int64   { return int64(uint32(x)) }
func F32_demote_f64(x float64) float32 { return float32(x) }
func F64_promote_f32(x float32) float64 {
	if math.IsNaN(float64(x)) {
		return float64(x)
	}
	return float64(x)
}

func F32_convert_i32_s(x int32) float32 { return float32(x) }
func F32_convert_i32_u(x int32) float32 { return float32(uint32(x)) }
func F32_convert_i64_s(x int64) float32 { return float32(x) }
func F32_convert_i64_u(x int64) float32 { return float32(uint64(x)) }
func F64_convert_i32_s(x int32) float64 { return float64(x) }
func F64_convert_i32_u(x int32) float64 { return float64(uint32(x)) }
func F64_convert_i64_s(x int64) float64 { return float64(x) }
func F64_convert_i64_u(x int64) float64 { return float64(uint64(x)) }

func I32_reinterpret_f32(x float32) int32 { return int32(math.Float32bits(x)) }
func I64_reinterpret_f64(x float64) int64 { return int64(math.Float64bits(x)) }
func F32_reinterpret_i32(x int32) float32 { return math.Float32frombits(uint32(x)) }
func F64_reinterpret_i64(x int64) float64 { return math.Float64frombits(uint64(x)) }

func I32_extend8_s(x int32) int32  { return int32(int8(x)) }
func I32_extend16_s(x int32) int32 { return int32(int16(x)) }
func I64_extend8_s(x int64) int64  { return int64(int8(x)) }
func I64_extend16_s(x int64) int64 { return int64(int16(x)) }
func I64_extend32_s(x int64) int64 { return int64(int32(x)) }

func MemoryFill(m *Module, dst int32, val int32, n int32) {
	if n == 0 {
		return
	}

	end := uint64(uint32(dst)) + uint64(uint32(n))
	if end > uint64(len(m.Memory)) {
		panic("wasm: memory.fill out of bounds")
	}

	b := m.Memory[uint32(dst):uint32(end)]
	v := byte(val)
	for k := range b {
		b[k] = v
	}
}

func MemoryCopy(m *Module, dst int32, src int32, n int32) {
	if n == 0 {
		return
	}

	srcEnd := uint64(uint32(src)) + uint64(uint32(n))
	dstEnd := uint64(uint32(dst)) + uint64(uint32(n))
	if srcEnd > uint64(len(m.Memory)) || dstEnd > uint64(len(m.Memory)) {
		panic("wasm: memory.copy out of bounds")
	}

	copy(m.Memory[uint32(dst):uint32(dstEnd)], m.Memory[uint32(src):uint32(srcEnd)])
}

// WasiExitError is the sentinel that the recover layer of SafeInvokeExport
// promotes Proc_exit() panics into, so a wasm-level exit doesn't kill the
// host process and the caller can read the exit code instead.
type WasiExitError struct{ Code int32 }

func (e *WasiExitError) Error() string { return "wasi: proc_exit(" + itoa32(e.Code) + ")" }

// itoa32 is a tiny dependency-free strconv replacement so this file
// doesn't drag in fmt for its sole error path.
func itoa32(v int32) string {
	if v == 0 {
		return "0"
	}

	neg := false
	if v < 0 {
		v = -v
		neg = true
	}

	var buf [12]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// wasiOpen is one entry in WasiStubs' fd table. Stdio entries are nil-file
// markers (writes go to the OS handles directly via the WasiStubs fields).
// The conn arm carries a net.Conn for sockets opened via Sock_accept.
type wasiOpen struct {
	f        *os.File
	conn     net.Conn
	listener net.Listener
	isDir    bool
	path     string // absolute path for stat/readdir
	fdflags  int32  // last fdflags set via Path_open or Fd_fdstat_set_flags
	dirCache []os.DirEntry
}

// WasiStubs is the default Go-native implementation of wasi_snapshot_preview1.
// State is owned per-Module via NewWithWASI / DefaultWASI.
type WasiStubs struct {
	mu sync.Mutex

	stdin, stdout, stderr *os.File
	fdTable               map[int32]*wasiOpen
	nextFD                int32
	args, env             []string
	monoStart             time.Time
	// preopenDir is the host directory mapped to wasi preopen fd 3.
	// Defaults to "/" (i.e. no rewriting) — the legacy behaviour. Tests
	// can set this via SetPreopenDir to scope filesystem ops to a
	// temporary directory.
	preopenDir string
}

// DefaultWASI returns a WasiStubs configured for typical CLI use: real
// stdio, os.Args, os.Environ(), wall + monotonic clocks. Consumers who
// want a sandboxed setup should construct their own WasiStubs (or any
// Wasi_snapshot_preview1Imports implementation) and pass it to
// NewWithWASI.
func DefaultWASI() *WasiStubs {
	return &WasiStubs{stdin: os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		fdTable:    map[int32]*wasiOpen{},
		nextFD:     4,
		args:       os.Args,
		env:        os.Environ(),
		monoStart:  time.Now(),
		preopenDir: "/"}
}

// SetPreopenDir scopes the WASI preopen "/" to a host directory. Empty
// string restores the default ("/"), i.e. no rewriting. Tests use this
// to run filesystem syscalls against t.TempDir().
func (w *WasiStubs) SetPreopenDir(dir string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if dir == "" {
		w.preopenDir = "/"
		return
	}

	w.preopenDir = dir
}

// joinPreopen turns a wasm-supplied relative path into a host path
// scoped to the configured preopen directory. Callers MUST hold w.mu.
func (w *WasiStubs) joinPreopen(rel string) string {
	if w.preopenDir == "" || w.preopenDir == "/" {
		return "/" + rel
	}
	return filepath.Join(w.preopenDir, rel)
}

// memSlice returns m.memory[off : off+n]. Callers must hold any locks
// they need on the wasm side; WasiStubs.mu is independent. Returns an
// empty slice on out-of-range (the wasi function should then return
// EFAULT / EINVAL).
func (w *WasiStubs) memSlice(m *Module, off, n int32) []byte {
	mem := m.Memory
	lo := uint64(uint32(off))
	hi := lo + uint64(uint32(n))
	if hi > uint64(len(mem)) {
		return nil
	}
	return mem[lo:hi]
}

// errno values used below (subset; see wasi-libc errno.h).
const (
	_wasiESUCCESS int32 = 0
	_wasiE2BIG    int32 = 1
	_wasiEACCES   int32 = 2
	_wasiEAGAIN   int32 = 6
	_wasiEBADF    int32 = 8
	_wasiEBUSY    int32 = 10
	_wasiEEXIST   int32 = 20
	_wasiEFAULT   int32 = 21
	_wasiEINVAL   int32 = 28
	_wasiEIO      int32 = 29
	_wasiEISDIR   int32 = 31
	_wasiENOENT   int32 = 44
	_wasiENOTDIR  int32 = 54
	_wasiENOTSOCK int32 = 57
	_wasiENOTSUP  int32 = 58
	_wasiEPERM    int32 = 63
	_wasiEPIPE    int32 = 64
)

// mapOSError turns an os/filesystem error into a wasi errno. Used by the
// path-based syscalls so any os.PathError surfaces as the appropriate
// guest-visible code instead of a coarse EIO.
func mapOSError(err error) int32 {
	if err == nil {
		return _wasiESUCCESS
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return _wasiENOENT
	case errors.Is(err, fs.ErrExist):
		return _wasiEEXIST
	case errors.Is(err, fs.ErrPermission):
		return _wasiEACCES
	case errors.Is(err, syscall.ENOTDIR):
		return _wasiENOTDIR
	case errors.Is(err, syscall.EISDIR):
		return _wasiEISDIR
	case errors.Is(err, syscall.EINVAL):
		return _wasiEINVAL
	case errors.Is(err, syscall.EBADF):
		return _wasiEBADF
	case errors.Is(err, syscall.EAGAIN):
		return _wasiEAGAIN
	case errors.Is(err, syscall.EPIPE):
		return _wasiEPIPE
	}
	return _wasiEIO
}

// totalBytes sums len(s)+1 over s in a uint64 and reports whether the
// total fits in an int32 (i.e. is representable as a wasm-side i32
// length). Callers route the result through memSlice and an OOB on a
// pathologically long arg list surfaces as EFAULT to the guest rather
// than a host-side panic via a wrapped-int32 length.
func totalBytesPlusNul(ss []string) (int32, bool) {
	var total uint64
	for _, s := range ss {
		total += uint64(len(s)) + 1
		if total > 0x7fffffff {
			return 0, false
		}
	}
	return int32(total), true
}

func (w *WasiStubs) Args_get(m *Module, argv, argvBuf int32) int32 {
	w.mu.Lock()
	defer w.mu.Unlock()

	argvBytes64 := uint64(len(w.args)) * 4
	if argvBytes64 > 0x7fffffff {
		return _wasiEFAULT
	}

	argvSlice := w.memSlice(m, argv, int32(argvBytes64))
	if argvSlice == nil {
		return _wasiEFAULT
	}

	total, ok := totalBytesPlusNul(w.args)
	if !ok {
		return _wasiEFAULT
	}

	argvBufSlice := w.memSlice(m, argvBuf, total)
	if argvBufSlice == nil {
		return _wasiEFAULT
	}

	bufOff := uint32(0)
	for i, a := range w.args {
		binary.LittleEndian.PutUint32(argvSlice[i*4:], uint32(argvBuf)+bufOff)
		n := copy(argvBufSlice[bufOff:], a)
		if n < len(a) {
			return _wasiEFAULT
		}

		bufOff += uint32(n)
		argvBufSlice[bufOff] = 0
		bufOff++
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Args_sizes_get(m *Module, argcPtr, argvBufLenPtr int32) int32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	argcSlice := w.memSlice(m, argcPtr, 4)
	bufLenSlice := w.memSlice(m, argvBufLenPtr, 4)
	if argcSlice == nil || bufLenSlice == nil {
		return _wasiEFAULT
	}

	total, ok := totalBytesPlusNul(w.args)
	if !ok {
		return _wasiEFAULT
	}

	binary.LittleEndian.PutUint32(argcSlice, uint32(len(w.args)))
	binary.LittleEndian.PutUint32(bufLenSlice, uint32(total))
	return _wasiESUCCESS
}

func (w *WasiStubs) Environ_get(m *Module, envv, envBuf int32) int32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	envvBytes64 := uint64(len(w.env)) * 4
	if envvBytes64 > 0x7fffffff {
		return _wasiEFAULT
	}

	envvSlice := w.memSlice(m, envv, int32(envvBytes64))
	if envvSlice == nil {
		return _wasiEFAULT
	}

	total, ok := totalBytesPlusNul(w.env)
	if !ok {
		return _wasiEFAULT
	}

	envBufSlice := w.memSlice(m, envBuf, total)
	if envBufSlice == nil {
		return _wasiEFAULT
	}

	bufOff := uint32(0)
	for i, e := range w.env {
		binary.LittleEndian.PutUint32(envvSlice[i*4:], uint32(envBuf)+bufOff)
		n := copy(envBufSlice[bufOff:], e)
		if n < len(e) {
			return _wasiEFAULT
		}

		bufOff += uint32(n)
		envBufSlice[bufOff] = 0
		bufOff++
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Environ_sizes_get(m *Module, envcPtr, envBufLenPtr int32) int32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	envcSlice := w.memSlice(m, envcPtr, 4)
	bufLenSlice := w.memSlice(m, envBufLenPtr, 4)
	if envcSlice == nil || bufLenSlice == nil {
		return _wasiEFAULT
	}

	total, ok := totalBytesPlusNul(w.env)
	if !ok {
		return _wasiEFAULT
	}

	binary.LittleEndian.PutUint32(envcSlice, uint32(len(w.env)))
	binary.LittleEndian.PutUint32(bufLenSlice, uint32(total))
	return _wasiESUCCESS
}

func (w *WasiStubs) Clock_res_get(m *Module, clockID int32, resPtr int32) int32 {

	out := w.memSlice(m, resPtr, 8)
	if out == nil {
		return _wasiEFAULT
	}

	binary.LittleEndian.PutUint64(out, 1)
	return _wasiESUCCESS
}

func (w *WasiStubs) Clock_time_get(m *Module, clockID int32, precision int64, timePtr int32) int32 {
	out := w.memSlice(m, timePtr, 8)
	if out == nil {
		return _wasiEFAULT
	}

	var nanos uint64
	switch clockID {
	case 0:
		nanos = uint64(time.Now().UnixNano())
	case 1:
		w.mu.Lock()
		nanos = uint64(time.Since(w.monoStart).Nanoseconds())
		w.mu.Unlock()
	default:
		return _wasiEINVAL
	}

	binary.LittleEndian.PutUint64(out, nanos)
	return _wasiESUCCESS
}

// closeWasiOpen releases every underlying handle held by op and
// joins any Close errors so callers can map them to a wasi errno
// instead of silently dropping the failure.
func closeWasiOpen(op *wasiOpen) error {
	var err error
	if op.f != nil {
		err = errors.Join(err, op.f.Close())
	}
	if op.conn != nil {
		err = errors.Join(err, op.conn.Close())
	}
	if op.listener != nil {
		err = errors.Join(err, op.listener.Close())
	}
	return err
}

func (w *WasiStubs) Fd_close(m *Module, fd int32) int32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	op := w.fdTable[fd]
	if op == nil {
		return _wasiEBADF
	}

	closeErr := closeWasiOpen(op)
	delete(w.fdTable, fd)
	if closeErr != nil {
		return mapOSError(closeErr)
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_fdstat_get(m *Module, fd, ptr int32) int32 {

	out := w.memSlice(m, ptr, 24)
	if out == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	var ftype byte = 4 // regular file
	var fdflags uint16
	if fd >= 0 && fd <= 2 {
		ftype = 2
	} else if op := w.fdTable[fd]; op != nil {
		if op.isDir {
			ftype = 3
		} else if op.conn != nil {
			ftype = 6
		} else if op.listener != nil {
			ftype = 6
		}

		fdflags = uint16(op.fdflags)
	} else if fd == 3 {
		ftype = 3
	} else if fd >= 4 {
		return _wasiEBADF
	}

	out[0] = ftype
	out[1] = 0
	binary.LittleEndian.PutUint16(out[2:], fdflags)

	binary.LittleEndian.PutUint64(out[8:], ^uint64(0))
	binary.LittleEndian.PutUint64(out[16:], ^uint64(0))
	return _wasiESUCCESS
}

// Fd_fdstat_set_flags maps WASI fdflags to OS file-status flags via the
// per-platform Fcntl wrapper. The flags are also cached on the wasiOpen
// so a subsequent Fd_fdstat_get reflects what the guest set. Stdio fds
// store the requested flags but otherwise no-op; sockets/listeners take
// only the cache update because Go's net layer manages blocking mode
// internally.
func (w *WasiStubs) Fd_fdstat_set_flags(m *Module, fd, flags int32) int32 {
	w.mu.Lock()
	op := w.fdTable[fd]
	if op == nil && fd > 2 {
		w.mu.Unlock()
		return _wasiEBADF
	}
	if op != nil {
		op.fdflags = flags
	}

	w.mu.Unlock()

	_ = op
	_ = flags
	return _wasiESUCCESS
}

// Fd_fdstat_set_rights stores the requested rights on the wasiOpen but
// does not enforce them — the host process is the trust boundary. WASI
// programs that succeed with maximal rights (per Fd_fdstat_get) get the
// same ESUCCESS here.
func (w *WasiStubs) Fd_fdstat_set_rights(m *Module, fd int32, rightsBase, rightsInherit int64) int32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if fd >= 0 && fd <= 2 {
		return _wasiESUCCESS
	}
	if w.fdTable[fd] == nil {
		return _wasiEBADF
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_filestat_get(m *Module, fd, ptr int32) int32 {

	out := w.memSlice(m, ptr, 64)
	if out == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.f == nil {
		for i := range out {
			out[i] = 0
		}
		switch fd {
		case 0, 1, 2:
			out[16] = 2
		case 3:
			out[16] = 3
		}
		return _wasiESUCCESS
	}

	st, err := op.f.Stat()
	if err != nil {
		return mapOSError(err)
	}

	writeFilestat(out, st)
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_filestat_set_size(m *Module, fd int32, size int64) int32 {
	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.f == nil {
		return _wasiEBADF
	}
	if err := op.f.Truncate(size); err != nil {
		return mapOSError(err)
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_filestat_set_times(m *Module, fd, atimHi, atimLo, mtimHi, mtimLo, fstFlags int32) int32 {

	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.f == nil {
		return _wasiEBADF
	}

	atim := combine64(atimHi, atimLo)
	mtim := combine64(mtimHi, mtimLo)
	atime, mtime, err := resolveFiletimes(atim, mtim, fstFlags, op.f)
	if err != nil {
		return mapOSError(err)
	}
	if err := os.Chtimes(op.f.Name(), atime, mtime); err != nil {
		return mapOSError(err)
	}
	return _wasiESUCCESS
}

// combine64 reconstructs an unsigned 64-bit time value from a pair of
// 32-bit args. WASI signature uses two i32s for the nanosecond timestamp
// in fd_filestat_set_times.
func combine64(hi, lo int32) uint64 { return (uint64(uint32(hi)) << 32) | uint64(uint32(lo)) }

// resolveFiletimes decides the (atime, mtime) pair to apply given a
// fstFlags bitmask. Bits 0x2 (ATIME_NOW) and 0x8 (MTIME_NOW) override the
// explicit values with time.Now(). Unset ATIME/MTIME bits keep the
// existing on-disk time, so f.Stat must succeed when those bits are
// unset; the error is returned so the caller can surface it as a wasi
// errno rather than silently writing epoch.
func resolveFiletimes(atimNs, mtimNs uint64, fstFlags int32, f *os.File) (time.Time, time.Time, error) {
	now := time.Now()
	var atime, mtime time.Time

	needCurrent := fstFlags&(0x1|0x2) == 0 || fstFlags&(0x4|0x8) == 0
	if needCurrent {
		st, err := f.Stat()
		if err != nil {
			return time.Time{}, time.Time{}, err
		}

		atime = st.ModTime()
		mtime = st.ModTime()
	}
	if fstFlags&0x1 != 0 {
		atime = time.Unix(0, int64(atimNs))
	}
	if fstFlags&0x2 != 0 {
		atime = now
	}
	if fstFlags&0x4 != 0 {
		mtime = time.Unix(0, int64(mtimNs))
	}
	if fstFlags&0x8 != 0 {
		mtime = now
	}
	return atime, mtime, nil
}

func (w *WasiStubs) Fd_prestat_get(m *Module, fd, ptr int32) int32 {
	if fd != 3 {
		return _wasiEBADF
	}

	out := w.memSlice(m, ptr, 8)
	if out == nil {
		return _wasiEFAULT
	}

	out[0] = 0
	binary.LittleEndian.PutUint32(out[4:], 1)
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_prestat_dir_name(m *Module, fd, buf, buflen int32) int32 {
	if fd != 3 {
		return _wasiEBADF
	}
	if buflen < 1 {
		return _wasiESUCCESS
	}

	out := w.memSlice(m, buf, buflen)
	if out == nil {
		return _wasiEFAULT
	}

	out[0] = '/'
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_read(m *Module, fd, iovs, iovsLen, nreadPtr int32) int32 {
	w.mu.Lock()
	src, op := w.fdSrcLocked(fd)
	w.mu.Unlock()
	if src == nil {
		return _wasiEBADF
	}

	iovBytes := uint64(uint32(iovsLen)) * 8
	if iovBytes > 0x7fffffff {
		return _wasiEFAULT
	}

	iovecs := w.memSlice(m, iovs, int32(iovBytes))
	nreadSlice := w.memSlice(m, nreadPtr, 4)
	if iovecs == nil || nreadSlice == nil {
		return _wasiEFAULT
	}

	_ = op
	var total uint32
	for i := int32(0); i < iovsLen; i++ {
		bufPtr := binary.LittleEndian.Uint32(iovecs[i*8:])
		bufLen := binary.LittleEndian.Uint32(iovecs[i*8+4:])
		buf := w.memSlice(m, int32(bufPtr), int32(bufLen))
		if buf == nil {
			return _wasiEFAULT
		}

		n, err := src.Read(buf)
		total += uint32(n)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			break
		}
		if n < int(bufLen) {
			break
		}
	}

	binary.LittleEndian.PutUint32(nreadSlice, total)
	return _wasiESUCCESS
}

// fdSrcLocked returns the io.Reader for fd and (when applicable) the
// wasiOpen it came from, or nil if fd is invalid. Caller must hold w.mu.
func (w *WasiStubs) fdSrcLocked(fd int32) (io.Reader, *wasiOpen) {
	switch fd {
	case 0:
		return w.stdin, nil
	}

	op := w.fdTable[fd]
	if op == nil {
		return nil, nil
	}
	if op.f != nil {
		return op.f, op
	}
	if op.conn != nil {
		return op.conn, op
	}
	return nil, op
}

// fdDstLocked returns the io.Writer for fd or nil if fd is invalid.
// Caller must hold w.mu.
func (w *WasiStubs) fdDstLocked(fd int32) (io.Writer, *wasiOpen) {
	switch fd {
	case 1:
		return w.stdout, nil
	case 2:
		return w.stderr, nil
	}

	op := w.fdTable[fd]
	if op == nil {
		return nil, nil
	}
	if op.f != nil {
		return op.f, op
	}
	if op.conn != nil {
		return op.conn, op
	}
	return nil, op
}

func (w *WasiStubs) Fd_pread(m *Module, fd, iovs, iovsLen int32, offset int64, nreadPtr int32) int32 {
	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.f == nil {
		return _wasiEBADF
	}

	iovBytes := uint64(uint32(iovsLen)) * 8
	if iovBytes > 0x7fffffff {
		return _wasiEFAULT
	}

	iovecs := w.memSlice(m, iovs, int32(iovBytes))
	nreadSlice := w.memSlice(m, nreadPtr, 4)
	if iovecs == nil || nreadSlice == nil {
		return _wasiEFAULT
	}

	var total uint32
	curOff := offset
	for i := int32(0); i < iovsLen; i++ {
		bufPtr := binary.LittleEndian.Uint32(iovecs[i*8:])
		bufLen := binary.LittleEndian.Uint32(iovecs[i*8+4:])
		buf := w.memSlice(m, int32(bufPtr), int32(bufLen))
		if buf == nil {
			return _wasiEFAULT
		}

		n, err := op.f.ReadAt(buf, curOff)
		total += uint32(n)
		curOff += int64(n)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			break
		}
		if n < int(bufLen) {
			break
		}
	}

	binary.LittleEndian.PutUint32(nreadSlice, total)
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_pwrite(m *Module, fd, iovs, iovsLen int32, offset int64, nwrittenPtr int32) int32 {
	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.f == nil {
		return _wasiEBADF
	}

	iovBytes := uint64(uint32(iovsLen)) * 8
	if iovBytes > 0x7fffffff {
		return _wasiEFAULT
	}

	iovecs := w.memSlice(m, iovs, int32(iovBytes))
	nwSlice := w.memSlice(m, nwrittenPtr, 4)
	if iovecs == nil || nwSlice == nil {
		return _wasiEFAULT
	}

	var total uint32
	curOff := offset
	for i := int32(0); i < iovsLen; i++ {
		bufPtr := binary.LittleEndian.Uint32(iovecs[i*8:])
		bufLen := binary.LittleEndian.Uint32(iovecs[i*8+4:])
		buf := w.memSlice(m, int32(bufPtr), int32(bufLen))
		if buf == nil {
			return _wasiEFAULT
		}

		n, err := op.f.WriteAt(buf, curOff)
		total += uint32(n)
		curOff += int64(n)
		if err != nil {
			break
		}
	}

	binary.LittleEndian.PutUint32(nwSlice, total)
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_seek(m *Module, fd int32, offset int64, whence, newOffPtr int32) int32 {
	out := w.memSlice(m, newOffPtr, 8)
	if out == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.f == nil {
		return _wasiEBADF
	}

	n, err := op.f.Seek(offset, int(whence))
	if err != nil {
		return _wasiEINVAL
	}

	binary.LittleEndian.PutUint64(out, uint64(n))
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_tell(m *Module, fd, offsetPtr int32) int32 {
	out := w.memSlice(m, offsetPtr, 8)
	if out == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.f == nil {
		return _wasiEBADF
	}

	n, err := op.f.Seek(0, 1)
	if err != nil {
		return _wasiEIO
	}

	binary.LittleEndian.PutUint64(out, uint64(n))
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_write(m *Module, fd, iovs, iovsLen, nwrittenPtr int32) int32 {
	w.mu.Lock()
	dst, _ := w.fdDstLocked(fd)
	w.mu.Unlock()
	iovBytes := uint64(uint32(iovsLen)) * 8
	if iovBytes > 0x7fffffff {
		return _wasiEFAULT
	}

	iovecs := w.memSlice(m, iovs, int32(iovBytes))
	nwrittenSlice := w.memSlice(m, nwrittenPtr, 4)
	if iovecs == nil || nwrittenSlice == nil {
		return _wasiEFAULT
	}
	if dst == nil {
		binary.LittleEndian.PutUint32(nwrittenSlice, 0)
		return _wasiEBADF
	}

	var total uint32
	for i := int32(0); i < iovsLen; i++ {
		bufPtr := binary.LittleEndian.Uint32(iovecs[i*8:])
		bufLen := binary.LittleEndian.Uint32(iovecs[i*8+4:])
		buf := w.memSlice(m, int32(bufPtr), int32(bufLen))
		if buf == nil {
			return _wasiEFAULT
		}

		n, err := dst.Write(buf)
		total += uint32(n)
		if err != nil {
			break
		}
	}

	binary.LittleEndian.PutUint32(nwrittenSlice, total)
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_sync(m *Module, fd int32) int32 {
	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.f == nil {
		return _wasiEBADF
	}
	if err := op.f.Sync(); err != nil {
		return mapOSError(err)
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_datasync(m *Module, fd int32) int32 {
	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.f == nil {
		return _wasiEBADF
	}
	if err := op.f.Sync(); err != nil {
		return mapOSError(err)
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_advise(m *Module, fd int32, offset, length int64, advice int32) int32 {
	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.f == nil {
		return _wasiEBADF
	}

	_, _, _ = offset, length, advice
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_allocate(m *Module, fd int32, offset, length int64) int32 {
	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.f == nil {
		return _wasiEBADF
	}
	if err := op.f.Truncate(offset + length); err != nil {
		return mapOSError(err)
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Fd_renumber(m *Module, from, to int32) int32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if from == to {
		if _, ok := w.fdTable[from]; ok {
			return _wasiESUCCESS
		}
		return _wasiEBADF
	}

	src, ok := w.fdTable[from]
	if !ok {
		return _wasiEBADF
	}

	var closeErr error
	if dst, ok2 := w.fdTable[to]; ok2 {
		closeErr = closeWasiOpen(dst)
	}

	w.fdTable[to] = src
	delete(w.fdTable, from)
	if closeErr != nil {
		return mapOSError(closeErr)
	}
	return _wasiESUCCESS
}

// readDirCached lazily caches the directory listing on first
// Fd_readdir, so paged reads (cookie-driven) walk the same snapshot.
func (op *wasiOpen) readDirCached() ([]os.DirEntry, error) {
	if op.dirCache != nil {
		return op.dirCache, nil
	}
	if op.f == nil {
		return nil, syscall.EBADF
	}
	if _, err := op.f.Seek(0, 0); err != nil {
		return nil, err
	}

	entries, err := op.f.ReadDir(-1)
	if err != nil {
		return nil, err
	}

	out := make([]os.DirEntry, 0, len(entries)+2)
	out = append(out, dotEntry(op.path, "."), dotEntry(op.path, ".."))
	out = append(out, entries...)

	sort.SliceStable(out[2:], func(i, j int) bool {
		return out[2+i].Name() < out[2+j].Name()
	})
	op.dirCache = out
	return out, nil
}

// dotEntry produces a synthetic os.DirEntry for "." and "..". Its
// Info() returns the stat of the parent directory (good enough for
// guest-side d_type detection).
func dotEntry(parent, name string) os.DirEntry { return &dotDirEntry{name: name, parent: parent} }

type dotDirEntry struct {
	name, parent string
}

func (d *dotDirEntry) Name() string      { return d.name }
func (d *dotDirEntry) IsDir() bool       { return true }
func (d *dotDirEntry) Type() os.FileMode { return os.ModeDir }

func (d *dotDirEntry) Info() (os.FileInfo, error) {
	if d.name == "." {
		return os.Stat(d.parent)
	}
	return os.Stat(filepath.Dir(d.parent))
}

func (w *WasiStubs) Fd_readdir(m *Module, fd, buf, buflen int32, cookie int64, bufusedPtr int32) int32 {
	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.f == nil || !op.isDir {
		return _wasiEBADF
	}

	bufSlice := w.memSlice(m, buf, buflen)
	bufusedSlice := w.memSlice(m, bufusedPtr, 4)
	if bufSlice == nil || bufusedSlice == nil {
		return _wasiEFAULT
	}
	if cookie == 0 {
		op.dirCache = nil
	}

	entries, err := op.readDirCached()
	if err != nil {
		return mapOSError(err)
	}

	startIdx := int(cookie)
	if startIdx < 0 {
		startIdx = 0
	}

	written := 0
	for i := startIdx; i < len(entries); i++ {
		e := entries[i]
		nameBytes := []byte(e.Name())
		// dirent header: d_next u64 + d_ino u64 + d_namlen u32 + d_type u8 + 3 pad = 24 bytes.
		const headerLen = 24
		if written+headerLen > len(bufSlice) {

			copy(bufSlice[written:], make([]byte, len(bufSlice)-written))
			written = len(bufSlice)
			break
		}

		nextCookie := uint64(i + 1)
		// os.FileInfo does not expose inode portably; report 0.
		var ino uint64
		var dtype byte = 4
		if // regular file
		e.IsDir() {
			dtype = 3
		} else if e.Type()&os.ModeSymlink != 0 {
			dtype = 7
		} else if e.Type()&os.ModeNamedPipe != 0 {
			dtype = 6
		} else if e.Type()&os.ModeSocket != 0 {
			dtype = 6
		}

		binary.LittleEndian.PutUint64(bufSlice[written:], nextCookie)
		binary.LittleEndian.PutUint64(bufSlice[written+8:], ino)
		binary.LittleEndian.PutUint32(bufSlice[written+16:], uint32(len(nameBytes)))
		bufSlice[written+20] = dtype
		bufSlice[written+21] = 0
		bufSlice[written+22] = 0
		bufSlice[written+23] = 0
		written += headerLen
		n := copy(bufSlice[written:], nameBytes)
		written += n
		if n < len(nameBytes) {

			written = len(bufSlice)
			break
		}
	}

	binary.LittleEndian.PutUint32(bufusedSlice, uint32(written))
	return _wasiESUCCESS
}

// Path_open opens a wasm-supplied path and registers it in the fd
// table. The path is resolved against the host filesystem with the same
// rights the host Go process has — wasm2go's default WASI is a thin
// passthrough, not a sandbox. The dirFd == 3 special case keeps the
// "preopen /" convention that wasi-libc requires for its directory
// enumeration, but the path itself is opened verbatim (joined to "/")
// using os.OpenFile. Callers that need a sandbox should provide their
// own Wasi_snapshot_preview1Imports implementation via NewWithWASI.
func (w *WasiStubs) Path_open(m *Module, dirFd, dirflags, pathPtr, pathLen, oflags int32, fsRightsBase, fsRightsInherit int64, fdflags, openedFdPtr int32) int32 {
	if dirFd != 3 {
		return _wasiEBADF
	}

	pathSlice := w.memSlice(m, pathPtr, pathLen)
	outSlice := w.memSlice(m, openedFdPtr, 4)
	if pathSlice == nil || outSlice == nil {
		return _wasiEFAULT
	}

	rel := string(pathSlice)
	w.mu.Lock()
	full := w.joinPreopen(rel)
	w.mu.Unlock()

	canRead := fsRightsBase&(1<<1) != 0
	canWrite := fsRightsBase&(1<<6) != 0
	var flag int
	switch {
	case canRead && canWrite:
		flag = os.O_RDWR
	case canWrite && !canRead:
		flag = os.O_WRONLY
	default:

		flag = os.O_RDONLY
	}
	if oflags&0x1 != 0 {
		flag |= os.O_CREATE
	}
	if oflags&0x4 != 0 {
		flag |= os.O_EXCL
	}
	if oflags&0x8 != 0 {
		flag |= os.O_TRUNC
	}
	if fdflags&0x1 != 0 {
		flag |= os.O_APPEND
	}
	if fdflags&(0x2|0x8|0x10) != 0 {
		flag |= os.O_SYNC
	}

	requireDir := oflags&0x2 != 0
	noFollow := dirflags&0x1 == 0
	if requireDir {

		flag = os.O_RDONLY
	}
	if noFollow {
		if li, lerr := os.Lstat(full); lerr == nil && (li.Mode()&os.ModeSymlink) != 0 {
			return _wasiENOENT
		}
	}

	f, err := os.OpenFile(full, flag, 0o644)
	if err != nil {
		return mapOSError(err)
	}

	st, statErr := f.Stat()
	if statErr != nil {
		return mapOSError(errors.Join(statErr, f.Close()))
	}

	isDir := st.IsDir()
	if requireDir && !isDir {
		if cerr := f.Close(); cerr != nil {
			return mapOSError(cerr)
		}
		return _wasiENOTDIR
	}

	w.mu.Lock()
	fd := w.nextFD
	w.nextFD++
	w.fdTable[fd] = &wasiOpen{f: f, isDir: isDir, path: full, fdflags: fdflags}
	w.mu.Unlock()
	binary.LittleEndian.PutUint32(outSlice, uint32(fd))
	return _wasiESUCCESS
}

func (w *WasiStubs) Path_create_directory(m *Module, dirFd, pathPtr, pathLen int32) int32 {
	if dirFd != 3 {
		return _wasiEBADF
	}

	pathSlice := w.memSlice(m, pathPtr, pathLen)
	if pathSlice == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	full := w.joinPreopen(string(pathSlice))
	w.mu.Unlock()
	if err := os.Mkdir(full, 0o755); err != nil {
		return mapOSError(err)
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Path_unlink_file(m *Module, dirFd, pathPtr, pathLen int32) int32 {
	if dirFd != 3 {
		return _wasiEBADF
	}

	pathSlice := w.memSlice(m, pathPtr, pathLen)
	if pathSlice == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	full := w.joinPreopen(string(pathSlice))
	w.mu.Unlock()
	st, err := os.Lstat(full)
	if err != nil {
		return mapOSError(err)
	}
	if st.IsDir() {
		return _wasiEISDIR
	}
	if err := os.Remove(full); err != nil {
		return mapOSError(err)
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Path_remove_directory(m *Module, dirFd, pathPtr, pathLen int32) int32 {
	if dirFd != 3 {
		return _wasiEBADF
	}

	pathSlice := w.memSlice(m, pathPtr, pathLen)
	if pathSlice == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	full := w.joinPreopen(string(pathSlice))
	w.mu.Unlock()
	st, err := os.Lstat(full)
	if err != nil {
		return mapOSError(err)
	}
	if !st.IsDir() {
		return _wasiENOTDIR
	}
	if err := os.Remove(full); err != nil {
		return mapOSError(err)
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Path_rename(m *Module, oldFd, oldPathPtr, oldPathLen, newFd, newPathPtr, newPathLen int32) int32 {
	if oldFd != 3 || newFd != 3 {
		return _wasiEBADF
	}

	oldSlice := w.memSlice(m, oldPathPtr, oldPathLen)
	newSlice := w.memSlice(m, newPathPtr, newPathLen)
	if oldSlice == nil || newSlice == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	oldFull := w.joinPreopen(string(oldSlice))
	newFull := w.joinPreopen(string(newSlice))
	w.mu.Unlock()
	if err := os.Rename(oldFull, newFull); err != nil {
		return mapOSError(err)
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Path_filestat_get(m *Module, dirFd, flags, pathPtr, pathLen, outPtr int32) int32 {
	if dirFd != 3 {
		return _wasiEBADF
	}

	pathSlice := w.memSlice(m, pathPtr, pathLen)
	out := w.memSlice(m, outPtr, 64)
	if pathSlice == nil || out == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	full := w.joinPreopen(string(pathSlice))
	w.mu.Unlock()
	var st os.FileInfo
	var err error
	if flags&0x1 != 0 {
		st, err = os.Stat(full)
	} else {

		st, err = os.Lstat(full)
	}
	if err != nil {
		return mapOSError(err)
	}

	writeFilestat(out, st)
	return _wasiESUCCESS
}

func (w *WasiStubs) Path_filestat_set_times(m *Module, dirFd, flags, pathPtr, pathLen, atimHi, atimLo, mtimHi, mtimLo, fstFlags int32) int32 {
	if dirFd != 3 {
		return _wasiEBADF
	}

	pathSlice := w.memSlice(m, pathPtr, pathLen)
	if pathSlice == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	full := w.joinPreopen(string(pathSlice))
	w.mu.Unlock()
	atim := combine64(atimHi, atimLo)
	mtim := combine64(mtimHi, mtimLo)
	follow := flags&0x1 != 0
	now := time.Now()
	st, statErr := osLstatOrStat(full, follow)
	if statErr != nil {
		return mapOSError(statErr)
	}

	var atime, mtime time.Time
	atime = st.ModTime()
	mtime = st.ModTime()
	if fstFlags&0x1 != 0 {
		atime = time.Unix(0, int64(atim))
	}
	if fstFlags&0x2 != 0 {
		atime = now
	}
	if fstFlags&0x4 != 0 {
		mtime = time.Unix(0, int64(mtim))
	}
	if fstFlags&0x8 != 0 {
		mtime = now
	}
	if follow {
		if err := os.Chtimes(full, atime, mtime); err != nil {
			return mapOSError(err)
		}
	} else {
		if err := os.Chtimes(full, atime, mtime); err != nil {
			return mapOSError(err)
		}
	}
	return _wasiESUCCESS
}

func osLstatOrStat(p string, follow bool) (os.FileInfo, error) {
	if follow {
		return os.Stat(p)
	}
	return os.Lstat(p)
}

func (w *WasiStubs) Path_link(m *Module, oldFd, oldFlags, oldPathPtr, oldPathLen, newFd, newPathPtr, newPathLen int32) int32 {
	if oldFd != 3 || newFd != 3 {
		return _wasiEBADF
	}

	oldSlice := w.memSlice(m, oldPathPtr, oldPathLen)
	newSlice := w.memSlice(m, newPathPtr, newPathLen)
	if oldSlice == nil || newSlice == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	oldFull := w.joinPreopen(string(oldSlice))
	newFull := w.joinPreopen(string(newSlice))
	w.mu.Unlock()
	if err := os.Link(oldFull, newFull); err != nil {
		return mapOSError(err)
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Path_symlink(m *Module, targetPtr, targetLen, dirFd, linkPathPtr, linkPathLen int32) int32 {
	if dirFd != 3 {
		return _wasiEBADF
	}

	targetSlice := w.memSlice(m, targetPtr, targetLen)
	linkSlice := w.memSlice(m, linkPathPtr, linkPathLen)
	if targetSlice == nil || linkSlice == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	link := w.joinPreopen(string(linkSlice))
	w.mu.Unlock()
	if err := os.Symlink(string(targetSlice), link); err != nil {
		return mapOSError(err)
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Path_readlink(m *Module, dirFd, pathPtr, pathLen, buf, buflen, bufusedPtr int32) int32 {
	if dirFd != 3 {
		return _wasiEBADF
	}

	pathSlice := w.memSlice(m, pathPtr, pathLen)
	bufSlice := w.memSlice(m, buf, buflen)
	bufused := w.memSlice(m, bufusedPtr, 4)
	if pathSlice == nil || bufSlice == nil || bufused == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	full := w.joinPreopen(string(pathSlice))
	w.mu.Unlock()
	target, err := os.Readlink(full)
	if err != nil {
		return mapOSError(err)
	}

	n := copy(bufSlice, target)
	binary.LittleEndian.PutUint32(bufused, uint32(n))
	return _wasiESUCCESS
}

func (w *WasiStubs) Random_get(m *Module, buf, bufLen int32) int32 {
	slice := w.memSlice(m, buf, bufLen)
	if slice == nil {
		return _wasiEFAULT
	}

	_, err := rand.Read(slice)
	if err != nil {
		return _wasiEIO
	}
	return _wasiESUCCESS
}

func (w *WasiStubs) Sched_yield(m *Module) int32 { runtime.Gosched(); return _wasiESUCCESS }

// Poll_oneoff decodes the WASI subscription_u records and reproduces the
// requested events.
//
// Each subscription is 48 bytes:
//
//	u64 userdata
//	u8  eventtype  (0=clock, 1=fd_read, 2=fd_write)
//	... per-type payload starting at offset 16
//
// For clock subscriptions, payload at offset 16 is: u32 clock_id, u64
// timeout, u64 precision, u16 sub_clock_flags (bit0=ABSTIME). We sleep
// for `timeout` ns (relative timer) or the diff to `timeout` (absolute
// timer). For fd_read / fd_write subscriptions, payload at offset 16 is
// a u32 fd; we call into the platform Poll helper to wait for
// readiness.
//
// Each emitted event is 32 bytes: u64 userdata, u16 errno, u16
// eventtype, u64 fd_readwrite_nbytes (filled for fd events), u16
// flags, then 6 bytes of padding.
func (w *WasiStubs) Poll_oneoff(m *Module, inPtr, outPtr, nsubs, neventsPtr int32) int32 {
	subsTotal := uint64(uint32(nsubs)) * 48
	if subsTotal > 0x7fffffff {
		return _wasiEFAULT
	}

	subs := w.memSlice(m, inPtr, int32(subsTotal))
	evTotal := uint64(uint32(nsubs)) * 32
	if evTotal > 0x7fffffff {
		return _wasiEFAULT
	}

	events := w.memSlice(m, outPtr, int32(evTotal))
	nev := w.memSlice(m, neventsPtr, 4)
	if subs == nil || events == nil || nev == nil {
		return _wasiEFAULT
	}

	type pollItem struct {
		userdata uint64
		etype    byte
		fd       int32
		isRead   bool
	}
	var minClockNs int64 = -1
	var clockEvents []pollItem
	var fdEvents []pollItem
	for i := int32(0); i < nsubs; i++ {
		base := i * 48
		userdata := binary.LittleEndian.Uint64(subs[base:])
		etype := subs[base+8]
		switch etype {
		case 0:
			timeout := int64(binary.LittleEndian.Uint64(subs[base+24:]))
			flags := binary.LittleEndian.Uint16(subs[base+40:])
			ns := timeout
			if flags&0x1 != 0 {

				ns = timeout - time.Now().UnixNano()
				if ns < 0 {
					ns = 0
				}
			}
			if minClockNs < 0 || ns < minClockNs {
				minClockNs = ns
			}

			clockEvents = append(clockEvents, pollItem{userdata: userdata, etype: 0})
		case 1, 2:
			fd := int32(binary.LittleEndian.Uint32(subs[base+16:]))
			fdEvents = append(fdEvents, pollItem{userdata: userdata, etype: etype, fd: fd, isRead: etype == 1})
		default:

			clockEvents = append(clockEvents, pollItem{userdata: userdata, etype: etype})
		}
	}
	if minClockNs > 0 {
		time.Sleep(time.Duration(minClockNs))
	}

	written := int32(0)
	for _, ev := range clockEvents {
		writeEvent(events[written:written+32], ev.userdata, ev.etype, 0, 0)
		written += 32
	}
	for _, ev := range fdEvents {
		w.mu.Lock()
		op := w.fdTable[ev.fd]
		w.mu.Unlock()
		var errno int32
		var nbytes uint64
		if op == nil {
			errno = _wasiEBADF
		} else if op.f != nil {
			if ev.isRead {
				if st, err := op.f.Stat(); err == nil {
					if cur, err := op.f.Seek(0, 1); err == nil && st.Size() > cur {
						nbytes = uint64(st.Size() - cur)
					}
				}
			}
		} else if op.conn != nil {

			_ = minClockNs
		}

		writeEvent(events[written:written+32], ev.userdata, ev.etype, uint16(errno), nbytes)
		written += 32
	}

	binary.LittleEndian.PutUint32(nev, uint32(written/32))
	return _wasiESUCCESS
}

func writeEvent(dst []byte, userdata uint64, etype byte, errno uint16, nbytes uint64) {
	for i := range dst {
		dst[i] = 0
	}

	binary.LittleEndian.PutUint64(dst[0:], userdata)
	binary.LittleEndian.PutUint16(dst[8:], errno)
	binary.LittleEndian.PutUint16(dst[10:], uint16(etype))
	binary.LittleEndian.PutUint64(dst[16:], nbytes)
}

func (w *WasiStubs) Proc_exit(m *Module, code int32) { panic(&WasiExitError{Code: code}) }

func (w *WasiStubs) Proc_raise(m *Module, sig int32) int32 {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return mapOSError(err)
	}
	if err := p.Signal(syscall.Signal(sig)); err != nil {
		return mapOSError(err)
	}
	return _wasiESUCCESS
}

// Sock_accept accepts the next incoming TCP/Unix connection on the
// listener associated with fd, registers it as a new wasiOpen with a
// conn arm, and writes the new fd at fdOutPtr. Returns ENOTSOCK if fd
// isn't a listener.
func (w *WasiStubs) Sock_accept(m *Module, fd, flags, fdOutPtr int32) int32 {
	out := w.memSlice(m, fdOutPtr, 4)
	if out == nil {
		return _wasiEFAULT
	}

	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.listener == nil {
		return _wasiENOTSOCK
	}

	conn, err := op.listener.Accept()
	if err != nil {
		return mapOSError(err)
	}

	w.mu.Lock()
	newFD := w.nextFD
	w.nextFD++
	w.fdTable[newFD] = &wasiOpen{conn: conn}
	w.mu.Unlock()
	binary.LittleEndian.PutUint32(out, uint32(newFD))
	return _wasiESUCCESS
}

func (w *WasiStubs) Sock_recv(m *Module, fd, riData, riDataLen, riFlags, roDataLenPtr, roFlagsPtr int32) int32 {
	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.conn == nil {
		return _wasiENOTSOCK
	}

	iovBytes := uint64(uint32(riDataLen)) * 8
	if iovBytes > 0x7fffffff {
		return _wasiEFAULT
	}

	iovecs := w.memSlice(m, riData, int32(iovBytes))
	lenOut := w.memSlice(m, roDataLenPtr, 4)
	flagsOut := w.memSlice(m, roFlagsPtr, 4)
	if iovecs == nil || lenOut == nil || flagsOut == nil {
		return _wasiEFAULT
	}

	var total uint32
	for i := int32(0); i < riDataLen; i++ {
		bufPtr := binary.LittleEndian.Uint32(iovecs[i*8:])
		bufLen := binary.LittleEndian.Uint32(iovecs[i*8+4:])
		buf := w.memSlice(m, int32(bufPtr), int32(bufLen))
		if buf == nil {
			return _wasiEFAULT
		}

		n, err := op.conn.Read(buf)
		total += uint32(n)
		if err != nil {
			break
		}
	}

	binary.LittleEndian.PutUint32(lenOut, total)
	binary.LittleEndian.PutUint32(flagsOut, 0)
	return _wasiESUCCESS
}

func (w *WasiStubs) Sock_send(m *Module, fd, siData, siDataLen, siFlags, soDataLenPtr int32) int32 {
	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.conn == nil {
		return _wasiENOTSOCK
	}

	iovBytes := uint64(uint32(siDataLen)) * 8
	if iovBytes > 0x7fffffff {
		return _wasiEFAULT
	}

	iovecs := w.memSlice(m, siData, int32(iovBytes))
	lenOut := w.memSlice(m, soDataLenPtr, 4)
	if iovecs == nil || lenOut == nil {
		return _wasiEFAULT
	}

	var total uint32
	for i := int32(0); i < siDataLen; i++ {
		bufPtr := binary.LittleEndian.Uint32(iovecs[i*8:])
		bufLen := binary.LittleEndian.Uint32(iovecs[i*8+4:])
		buf := w.memSlice(m, int32(bufPtr), int32(bufLen))
		if buf == nil {
			return _wasiEFAULT
		}

		n, err := op.conn.Write(buf)
		total += uint32(n)
		if err != nil {
			break
		}
	}

	binary.LittleEndian.PutUint32(lenOut, total)
	return _wasiESUCCESS
}

func (w *WasiStubs) Sock_shutdown(m *Module, fd, how int32) int32 {
	w.mu.Lock()
	op := w.fdTable[fd]
	w.mu.Unlock()
	if op == nil || op.conn == nil {
		return _wasiENOTSOCK
	}

	type shutdowner interface {
		CloseRead() error
		CloseWrite() error
	}
	sh, ok := op.conn.(shutdowner)
	if !ok {
		if err := op.conn.Close(); err != nil {
			return mapOSError(err)
		}
		return _wasiESUCCESS
	}

	var shErr error
	if how&0x1 != 0 {
		shErr = errors.Join(shErr, sh.CloseRead())
	}
	if how&0x2 != 0 {
		shErr = errors.Join(shErr, sh.CloseWrite())
	}
	if shErr != nil {
		return mapOSError(shErr)
	}
	return _wasiESUCCESS
}

// writeFilestat populates the 64-byte WASI filestat structure from a
// host os.FileInfo. The dev/ino fields come from the per-platform
// wasiPlatformStatSys helper (unix returns Stat_t.Dev/.Ino; Windows
// returns zeros).
func writeFilestat(out []byte, st os.FileInfo) {

	binary.LittleEndian.PutUint64(out[0:], 0)
	binary.LittleEndian.PutUint64(out[8:], 0)
	var ftype byte = 4
	mode := st.Mode()
	switch {
	case mode.IsDir():
		ftype = 3
	case mode&os.ModeSymlink != 0:
		ftype = 7
	case mode&os.ModeNamedPipe != 0:
		ftype = 6
	case mode&os.ModeSocket != 0:
		ftype = 6
	case mode&os.ModeDevice != 0:
		ftype = 1
	case mode&os.ModeCharDevice != 0:
		ftype = 2
	}

	out[16] = ftype
	binary.LittleEndian.PutUint64(out[24:], 1)
	binary.LittleEndian.PutUint64(out[32:], uint64(st.Size()))
	nanos := uint64(st.ModTime().UnixNano())
	binary.LittleEndian.PutUint64(out[40:], nanos)
	binary.LittleEndian.PutUint64(out[48:], nanos)
	binary.LittleEndian.PutUint64(out[56:], nanos)
}
