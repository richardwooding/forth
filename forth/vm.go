// Package forth implements an interpreter for a subset of the Forth
// programming language, close to the ANS Forth / Forth-2012 core word set.
package forth

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// Cell is the machine word of the virtual machine. Addresses are byte
// addresses into VM.mem and a cell occupies CellSize bytes.
type Cell int64

// CellSize is the width of a cell in address units (bytes).
const CellSize = 8

// Defaults for the sizes of the VM's memory areas.
const (
	DefaultMemSize   = 1 << 20 // data space, in bytes
	DefaultStackSize = 1024    // maximum depth of the data and return stacks
	maxCallDepth     = 20000   // nesting limit of the inner interpreter
)

// Error is a Forth level error: it aborts the current evaluation, but leaves
// the VM usable.
type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

// ErrBye is returned by Eval when the program executed BYE.
var ErrBye = &Error{Msg: "bye"}

// A Word is a dictionary entry.
type Word struct {
	Name      string // display name, as it was defined
	Immediate bool   // executed even while compiling

	prim    func(*VM) // primitive behaviour, nil for colon definitions
	code    []instr   // body of a colon definition
	does    []instr   // run-time behaviour attached by DOES>
	data    Cell      // data field address (VARIABLE, CREATE, VALUE, ...)
	hasData bool      // word pushes its data field address
	isValue bool      // defined by VALUE, so TO can assign to it
	hidden  bool      // not yet findable (being compiled)
	xt      Cell      // execution token, index into VM.xts plus one
}

type loopFrame struct{ index, limit Cell }

// input is one level of the input source stack.
type input struct {
	buf  string
	pos  int
	name string // source name, for error messages
}

// line returns the 1-based line number of the parse position.
func (in *input) line() int {
	return 1 + strings.Count(in.buf[:min(in.pos, len(in.buf))], "\n")
}

// VM is a Forth virtual machine. Use New to create one; a VM is not safe for
// concurrent use.
type VM struct {
	Out io.Writer     // where . EMIT TYPE ... write
	In  *bufio.Reader // where KEY and ACCEPT read

	stack  []Cell // data stack
	rstack []Cell // return stack (>R R@ R>)
	loops  []loopFrame

	mem  []byte // data space; address 0 is never used
	here Cell

	words map[string]*Word // lookup by upper-cased name
	order []*Word          // definition order, for WORDS
	xts   []*Word          // execution tokens

	inputs []input // input source stack, last entry is current

	current *Word       // definition being compiled by :
	cstack  []ctrlEntry // compile time control-flow stack
	strBuf  Cell        // rotating buffer for transient strings
	strOff  Cell        // next slot of strBuf to use
	pad     Cell        // address of PAD
	holdBuf Cell        // pictured numeric output buffer
	holdPtr Cell        // first character held in holdBuf
	baseVar Cell        // address of BASE
	stateV  Cell        // address of STATE
	lastDef *Word       // most recent CREATEd word, for DOES>
	depth   int         // inner interpreter nesting
	source  func(string) (string, error)

	fs        FileSystem    // host file system, nil if file access is denied
	files     map[Cell]File // open files, by file identifier
	nextFid   Cell          // last file identifier handed out
	lastIOErr string        // message reported by FILE-ERROR
}

// New returns a VM with the standard dictionary loaded, reading from in and
// writing to out. Either may be nil.
func New(in io.Reader, out io.Writer) *VM {
	if out == nil {
		out = io.Discard
	}
	if in == nil {
		in = strings.NewReader("")
	}
	vm := &VM{
		Out:   out,
		In:    bufio.NewReader(in),
		mem:   make([]byte, DefaultMemSize),
		here:  CellSize, // keep address 0 unused
		words: make(map[string]*Word),
	}
	vm.baseVar = vm.allot(CellSize)
	vm.setCell(vm.baseVar, 10)
	vm.stateV = vm.allot(CellSize)
	vm.strBuf = vm.allot(4 * 256) // four rotating 256 byte string slots
	vm.pad = vm.allot(1024)
	vm.holdBuf = vm.allot(holdSize)
	vm.holdPtr = vm.holdBuf + holdSize
	vm.installPrims()
	if err := vm.Eval(prelude); err != nil {
		panic("forth: broken prelude: " + err.Error())
	}
	return vm
}

// SetSourceLoader installs the function used by INCLUDED and INCLUDE to read a
// file. Without a loader they fall back to the file system set by
// SetFileSystem, and a VM with neither cannot include files at all.
func (vm *VM) SetSourceLoader(load func(name string) (string, error)) {
	vm.source = load
}

// throw aborts the current evaluation with a Forth level error, tagged with
// the position in the input source when that source has a name.
func (vm *VM) throw(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if len(vm.inputs) > 0 {
		if in := &vm.inputs[len(vm.inputs)-1]; in.name != "" {
			msg = fmt.Sprintf("%s:%d: %s", in.name, in.line(), msg)
		}
	}
	panic(&Error{Msg: msg})
}

// catch converts a panic raised by throw into an error. Anything else keeps
// unwinding, so real bugs are not hidden.
func (vm *VM) catch(errp *error) {
	r := recover()
	if r == nil {
		return
	}
	if e, ok := r.(*Error); ok {
		*errp = e
		return
	}
	panic(r)
}

// Abort resets the VM to the interpreting state, discarding stacks and any
// partial definition.
func (vm *VM) Abort() {
	vm.stack = vm.stack[:0]
	vm.rstack = vm.rstack[:0]
	vm.loops = vm.loops[:0]
	vm.inputs = vm.inputs[:0]
	vm.cstack = vm.cstack[:0]
	if vm.current != nil {
		vm.forget(vm.current)
		vm.current = nil
	}
	vm.setCell(vm.stateV, 0)
	vm.depth = 0
}

// Stack returns a copy of the data stack, bottom first.
func (vm *VM) Stack() []Cell { return append([]Cell(nil), vm.stack...) }

// Compiling reports whether the VM is in the middle of a definition.
func (vm *VM) Compiling() bool { return vm.cell(vm.stateV) != 0 }

// --- data stack ------------------------------------------------------------

func (vm *VM) push(v Cell) {
	if len(vm.stack) >= DefaultStackSize {
		vm.throw("stack overflow")
	}
	vm.stack = append(vm.stack, v)
}

func (vm *VM) pop() Cell {
	n := len(vm.stack)
	if n == 0 {
		vm.throw("stack underflow")
	}
	v := vm.stack[n-1]
	vm.stack = vm.stack[:n-1]
	return v
}

func (vm *VM) pop2() (Cell, Cell) {
	b := vm.pop()
	a := vm.pop()
	return a, b
}

func (vm *VM) peek(i int) Cell {
	n := len(vm.stack)
	if i < 0 || i >= n {
		vm.throw("stack underflow")
	}
	return vm.stack[n-1-i]
}

func (vm *VM) pushBool(b bool) {
	if b {
		vm.push(-1)
	} else {
		vm.push(0)
	}
}

// --- return stack ----------------------------------------------------------

func (vm *VM) rpush(v Cell) {
	if len(vm.rstack) >= DefaultStackSize {
		vm.throw("return stack overflow")
	}
	vm.rstack = append(vm.rstack, v)
}

func (vm *VM) rpop() Cell {
	n := len(vm.rstack)
	if n == 0 {
		vm.throw("return stack underflow")
	}
	v := vm.rstack[n-1]
	vm.rstack = vm.rstack[:n-1]
	return v
}

// --- memory ----------------------------------------------------------------

func (vm *VM) checkAddr(addr, size Cell) {
	if addr < 1 || size < 0 || addr+size > Cell(len(vm.mem)) {
		vm.throw("invalid memory address %d", addr)
	}
}

func (vm *VM) cell(addr Cell) Cell {
	vm.checkAddr(addr, CellSize)
	return Cell(binary.LittleEndian.Uint64(vm.mem[addr:]))
}

func (vm *VM) setCell(addr, v Cell) {
	vm.checkAddr(addr, CellSize)
	binary.LittleEndian.PutUint64(vm.mem[addr:], uint64(v))
}

func (vm *VM) byteAt(addr Cell) byte {
	vm.checkAddr(addr, 1)
	return vm.mem[addr]
}

func (vm *VM) setByte(addr Cell, v byte) {
	vm.checkAddr(addr, 1)
	vm.mem[addr] = v
}

// allot reserves n bytes of data space and returns the address of the first.
func (vm *VM) allot(n Cell) Cell {
	if n < 0 {
		if vm.here+n < CellSize {
			vm.throw("cannot release %d bytes of data space", -n)
		}
		vm.here += n
		return vm.here
	}
	addr := vm.here
	if addr+n > Cell(len(vm.mem)) {
		vm.throw("out of data space")
	}
	vm.here += n
	return addr
}

func (vm *VM) readString(addr, n Cell) string {
	vm.checkAddr(addr, n)
	return string(vm.mem[addr : addr+n])
}

// transient stores s in a rotating buffer and returns its address and length,
// the representation used by S" while interpreting.
func (vm *VM) transient(s string) (Cell, Cell) {
	if len(s) > 255 {
		s = s[:255]
	}
	addr := vm.strBuf + vm.strOff*256
	vm.strOff = (vm.strOff + 1) % 4
	copy(vm.mem[addr:], s)
	return addr, Cell(len(s))
}

// --- dictionary ------------------------------------------------------------

func (vm *VM) define(w *Word) *Word {
	vm.xts = append(vm.xts, w)
	w.xt = Cell(len(vm.xts)) // 1-based, so 0 is never a valid xt
	vm.words[strings.ToUpper(w.Name)] = w
	vm.order = append(vm.order, w)
	return w
}

// Lookup finds a visible word by name, case-insensitively. A word that is
// still being compiled is skipped, so that a definition which mentions its own
// name refers to the previous definition, as Forth requires.
func (vm *VM) Lookup(name string) *Word {
	w := vm.words[strings.ToUpper(name)]
	if w == nil {
		return nil
	}
	if !w.hidden {
		return w
	}
	for i := len(vm.order) - 1; i >= 0; i-- {
		if prev := vm.order[i]; !prev.hidden && strings.EqualFold(prev.Name, name) {
			return prev
		}
	}
	return nil
}

// forget removes a word from the dictionary, restoring any earlier definition
// of the same name.
func (vm *VM) forget(w *Word) {
	for i := len(vm.order) - 1; i >= 0; i-- {
		if vm.order[i] == w {
			vm.order = append(vm.order[:i], vm.order[i+1:]...)
			break
		}
	}
	key := strings.ToUpper(w.Name)
	delete(vm.words, key)
	for i := len(vm.order) - 1; i >= 0; i-- {
		if strings.EqualFold(vm.order[i].Name, w.Name) {
			vm.words[key] = vm.order[i]
			break
		}
	}
	if w.xt >= 1 && int(w.xt) <= len(vm.xts) {
		vm.xts[w.xt-1] = nil
	}
}

func (vm *VM) wordFromXT(xt Cell) *Word {
	if xt < 1 || int(xt) > len(vm.xts) || vm.xts[xt-1] == nil {
		vm.throw("invalid execution token %d", xt)
	}
	return vm.xts[xt-1]
}

func (vm *VM) prim(name string, fn func(*VM)) *Word {
	return vm.define(&Word{Name: name, prim: fn})
}

func (vm *VM) imm(name string, fn func(*VM)) *Word {
	return vm.define(&Word{Name: name, prim: fn, Immediate: true})
}
