package forth

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

func (vm *VM) emitString(s string) {
	if _, err := io.WriteString(vm.Out, s); err != nil {
		vm.throw("output error: %v", err)
	}
}

func (vm *VM) installIO() {
	vm.prim(".", func(vm *VM) { vm.emitString(vm.format(vm.pop()) + " ") })
	vm.prim("U.", func(vm *VM) { vm.emitString(formatUnsigned(vm.pop(), vm.base()) + " ") })
	vm.prim(".R", func(vm *VM) {
		w := int(vm.pop())
		vm.emitString(pad(vm.format(vm.pop()), w))
	})
	vm.prim("U.R", func(vm *VM) {
		w := int(vm.pop())
		vm.emitString(pad(formatUnsigned(vm.pop(), vm.base()), w))
	})
	vm.prim("?", func(vm *VM) { vm.emitString(vm.format(vm.cell(vm.pop())) + " ") })
	vm.prim(".S", func(vm *VM) {
		var b strings.Builder
		fmt.Fprintf(&b, "<%d> ", len(vm.stack))
		for _, v := range vm.stack {
			b.WriteString(vm.format(v))
			b.WriteByte(' ')
		}
		vm.emitString(b.String())
	})
	vm.prim("EMIT", func(vm *VM) {
		c := vm.pop()
		if c < 0 || c > utf8.MaxRune {
			vm.throw("EMIT: %d is not a character", c)
		}
		if c < 0x80 {
			vm.emitString(string([]byte{byte(c)}))
		} else {
			vm.emitString(string(rune(c)))
		}
	})
	vm.prim("CR", func(vm *VM) { vm.emitString("\n") })
	vm.prim("SPACE", func(vm *VM) { vm.emitString(" ") })
	vm.prim("SPACES", func(vm *VM) {
		if n := int(vm.pop()); n > 0 {
			vm.emitString(strings.Repeat(" ", n))
		}
	})
	vm.prim("TYPE", func(vm *VM) {
		n := vm.pop()
		addr := vm.pop()
		if n > 0 {
			vm.emitString(vm.readString(addr, n))
		}
	})

	vm.prim("DECIMAL", func(vm *VM) { vm.setCell(vm.baseVar, 10) })
	vm.prim("HEX", func(vm *VM) { vm.setCell(vm.baseVar, 16) })
	vm.prim("OCTAL", func(vm *VM) { vm.setCell(vm.baseVar, 8) })
	vm.prim("BINARY", func(vm *VM) { vm.setCell(vm.baseVar, 2) })

	vm.imm("(", func(vm *VM) { vm.parse(')') })
	vm.imm("\\", func(vm *VM) { vm.parse('\n') })

	typeWord := vm.Lookup("TYPE")
	vm.imm(`."`, func(vm *VM) {
		s := vm.parseDelimited(`."`, '"')
		if vm.Compiling() {
			addr, n := vm.storeString(s)
			vm.compile(instr{op: opString, val: addr, len: n})
			vm.compile(instr{op: opCall, word: typeWord})
		} else {
			vm.emitString(s)
		}
	})
	vm.imm(`S"`, func(vm *VM) {
		s := vm.parseDelimited(`S"`, '"')
		if vm.Compiling() {
			addr, n := vm.storeString(s)
			vm.compile(instr{op: opString, val: addr, len: n})
		} else {
			addr, n := vm.transient(s)
			vm.push(addr)
			vm.push(n)
		}
	})
	vm.imm(`C"`, func(vm *VM) {
		s := vm.parseDelimited(`C"`, '"')
		addr := vm.storeCounted(s)
		if vm.Compiling() {
			vm.compile(instr{op: opLit, val: addr})
		} else {
			vm.push(addr)
		}
	})
	vm.prim("CHAR", func(vm *VM) {
		name := vm.nextName("CHAR")
		vm.push(Cell(name[0]))
	})
	vm.imm("[CHAR]", func(vm *VM) {
		vm.compileOnly("[CHAR]")
		name := vm.nextName("[CHAR]")
		vm.compile(instr{op: opLit, val: Cell(name[0])})
	})

	vm.prim("KEY", func(vm *VM) {
		r, _, err := vm.In.ReadRune()
		if err != nil {
			vm.throw("KEY: %v", err)
		}
		vm.push(Cell(r))
	})
	vm.prim("ACCEPT", func(vm *VM) {
		limit := vm.pop()
		addr := vm.pop()
		vm.checkAddr(addr, limit)
		line, err := vm.In.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if err != nil && line == "" {
			vm.throw("ACCEPT: %v", err)
		}
		if Cell(len(line)) > limit {
			line = line[:limit]
		}
		copy(vm.mem[addr:], line)
		vm.push(Cell(len(line)))
	})

	vm.prim("ABORT", func(vm *VM) { vm.throw("aborted") })
	vm.imm(`ABORT"`, func(vm *VM) {
		msg := vm.parseDelimited(`ABORT"`, '"')
		if vm.Compiling() {
			vm.compile(instr{op: opCall, word: &Word{
				Name: `(abort")`,
				prim: func(vm *VM) {
					if vm.pop() != 0 {
						vm.throw("%s", msg)
					}
				},
			}})
		} else if vm.pop() != 0 {
			vm.throw("%s", msg)
		}
	})
	vm.prim("BYE", func(vm *VM) { panic(ErrBye) })
	vm.prim("WORDS", func(vm *VM) {
		var b strings.Builder
		col := 0
		for i := len(vm.order) - 1; i >= 0; i-- {
			w := vm.order[i]
			if w.hidden {
				continue
			}
			if col > 0 && col+len(w.Name)+1 > 72 {
				b.WriteByte('\n')
				col = 0
			}
			b.WriteString(w.Name)
			b.WriteByte(' ')
			col += len(w.Name) + 1
		}
		b.WriteByte('\n')
		vm.emitString(b.String())
	})

	vm.prim("EVALUATE", func(vm *VM) {
		n := vm.pop()
		addr := vm.pop()
		src := vm.readString(addr, n)
		vm.pushInput(src, "EVALUATE")
		defer vm.popInput()
		vm.interpret()
	})
	included := func(vm *VM, name string) {
		src, err := vm.loadSource(name)
		if err != nil {
			vm.throw("%v", err)
		}
		vm.pushInput(src, name)
		defer vm.popInput()
		vm.interpret()
	}
	vm.prim("INCLUDED", func(vm *VM) {
		n := vm.pop()
		addr := vm.pop()
		included(vm, vm.readString(addr, n))
	})
	vm.prim("INCLUDE", func(vm *VM) { included(vm, vm.nextName("INCLUDE")) })
}

// storeString copies s into data space and returns its address and length.
func (vm *VM) storeString(s string) (Cell, Cell) {
	addr := vm.allot(Cell(len(s)))
	copy(vm.mem[addr:], s)
	vm.alignHere()
	return addr, Cell(len(s))
}

// storeCounted copies s into data space as a counted string.
func (vm *VM) storeCounted(s string) Cell {
	if len(s) > 255 {
		vm.throw("counted string longer than 255 characters")
	}
	addr := vm.allot(Cell(len(s)) + 1)
	vm.mem[addr] = byte(len(s))
	copy(vm.mem[addr+1:], s)
	vm.alignHere()
	return addr
}

func pad(s string, width int) string {
	if n := width - len(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}
