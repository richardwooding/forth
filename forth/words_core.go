package forth

// installPrims defines the whole standard dictionary.
func (vm *VM) installPrims() {
	vm.installStack()
	vm.installArith()
	vm.installMemory()
	vm.installCompile()
	vm.installControl()
	vm.installIO()
	vm.installPictured()
	vm.installDouble()
	vm.installSystem()
}

func (vm *VM) installStack() {
	vm.prim("DUP", func(vm *VM) { vm.push(vm.peek(0)) })
	vm.prim("?DUP", func(vm *VM) {
		if v := vm.peek(0); v != 0 {
			vm.push(v)
		}
	})
	vm.prim("DROP", func(vm *VM) { vm.pop() })
	vm.prim("SWAP", func(vm *VM) { a, b := vm.pop2(); vm.push(b); vm.push(a) })
	vm.prim("OVER", func(vm *VM) { vm.push(vm.peek(1)) })
	vm.prim("NIP", func(vm *VM) { b := vm.pop(); vm.pop(); vm.push(b) })
	vm.prim("TUCK", func(vm *VM) { a, b := vm.pop2(); vm.push(b); vm.push(a); vm.push(b) })
	vm.prim("ROT", func(vm *VM) {
		c := vm.pop()
		b := vm.pop()
		a := vm.pop()
		vm.push(b)
		vm.push(c)
		vm.push(a)
	})
	vm.prim("-ROT", func(vm *VM) {
		c := vm.pop()
		b := vm.pop()
		a := vm.pop()
		vm.push(c)
		vm.push(a)
		vm.push(b)
	})
	vm.prim("2DUP", func(vm *VM) { a, b := vm.peek(1), vm.peek(0); vm.push(a); vm.push(b) })
	vm.prim("2DROP", func(vm *VM) { vm.pop(); vm.pop() })
	vm.prim("2SWAP", func(vm *VM) {
		d := vm.pop()
		c := vm.pop()
		b := vm.pop()
		a := vm.pop()
		vm.push(c)
		vm.push(d)
		vm.push(a)
		vm.push(b)
	})
	vm.prim("2OVER", func(vm *VM) { a, b := vm.peek(3), vm.peek(2); vm.push(a); vm.push(b) })
	vm.prim("DEPTH", func(vm *VM) { vm.push(Cell(len(vm.stack))) })
	vm.prim("PICK", func(vm *VM) { n := vm.pop(); vm.push(vm.peek(int(n))) })
	vm.prim("ROLL", func(vm *VM) {
		n := int(vm.pop())
		if n < 0 || n >= len(vm.stack) {
			vm.throw("stack underflow")
		}
		i := len(vm.stack) - 1 - n
		v := vm.stack[i]
		copy(vm.stack[i:], vm.stack[i+1:])
		vm.stack[len(vm.stack)-1] = v
	})

	vm.prim(">R", func(vm *VM) { vm.rpush(vm.pop()) })
	vm.prim("R>", func(vm *VM) { vm.push(vm.rpop()) })
	vm.prim("R@", func(vm *VM) {
		if len(vm.rstack) == 0 {
			vm.throw("return stack underflow")
		}
		vm.push(vm.rstack[len(vm.rstack)-1])
	})
	vm.prim("RDROP", func(vm *VM) { vm.rpop() })
	vm.prim("2>R", func(vm *VM) { a, b := vm.pop2(); vm.rpush(a); vm.rpush(b) })
	vm.prim("2R>", func(vm *VM) { b := vm.rpop(); a := vm.rpop(); vm.push(a); vm.push(b) })
}

func (vm *VM) installArith() {
	vm.prim("+", func(vm *VM) { a, b := vm.pop2(); vm.push(a + b) })
	vm.prim("-", func(vm *VM) { a, b := vm.pop2(); vm.push(a - b) })
	vm.prim("*", func(vm *VM) { a, b := vm.pop2(); vm.push(a * b) })
	vm.prim("/", func(vm *VM) { a, b := vm.pop2(); q, _ := vm.floorDiv(a, b); vm.push(q) })
	vm.prim("MOD", func(vm *VM) { a, b := vm.pop2(); _, r := vm.floorDiv(a, b); vm.push(r) })
	vm.prim("/MOD", func(vm *VM) {
		a, b := vm.pop2()
		q, r := vm.floorDiv(a, b)
		vm.push(r)
		vm.push(q)
	})
	vm.prim("*/", func(vm *VM) {
		c := vm.pop()
		a, b := vm.pop2()
		q, _ := vm.floorDiv(a*b, c)
		vm.push(q)
	})
	vm.prim("*/MOD", func(vm *VM) {
		c := vm.pop()
		a, b := vm.pop2()
		q, r := vm.floorDiv(a*b, c)
		vm.push(r)
		vm.push(q)
	})
	vm.prim("1+", func(vm *VM) { vm.push(vm.pop() + 1) })
	vm.prim("1-", func(vm *VM) { vm.push(vm.pop() - 1) })
	vm.prim("2*", func(vm *VM) { vm.push(vm.pop() * 2) })
	vm.prim("2/", func(vm *VM) { vm.push(vm.pop() >> 1) })
	vm.prim("ABS", func(vm *VM) {
		if v := vm.pop(); v < 0 {
			vm.push(-v)
		} else {
			vm.push(v)
		}
	})
	vm.prim("NEGATE", func(vm *VM) { vm.push(-vm.pop()) })
	vm.prim("MIN", func(vm *VM) { a, b := vm.pop2(); vm.push(min(a, b)) })
	vm.prim("MAX", func(vm *VM) { a, b := vm.pop2(); vm.push(max(a, b)) })

	vm.prim("AND", func(vm *VM) { a, b := vm.pop2(); vm.push(a & b) })
	vm.prim("OR", func(vm *VM) { a, b := vm.pop2(); vm.push(a | b) })
	vm.prim("XOR", func(vm *VM) { a, b := vm.pop2(); vm.push(a ^ b) })
	vm.prim("INVERT", func(vm *VM) { vm.push(^vm.pop()) })
	vm.prim("LSHIFT", func(vm *VM) { a, b := vm.pop2(); vm.push(Cell(uint64(a) << vm.shift(b))) })
	vm.prim("RSHIFT", func(vm *VM) { a, b := vm.pop2(); vm.push(Cell(uint64(a) >> vm.shift(b))) })

	vm.prim("=", func(vm *VM) { a, b := vm.pop2(); vm.pushBool(a == b) })
	vm.prim("<>", func(vm *VM) { a, b := vm.pop2(); vm.pushBool(a != b) })
	vm.prim("<", func(vm *VM) { a, b := vm.pop2(); vm.pushBool(a < b) })
	vm.prim(">", func(vm *VM) { a, b := vm.pop2(); vm.pushBool(a > b) })
	vm.prim("<=", func(vm *VM) { a, b := vm.pop2(); vm.pushBool(a <= b) })
	vm.prim(">=", func(vm *VM) { a, b := vm.pop2(); vm.pushBool(a >= b) })
	vm.prim("U<", func(vm *VM) { a, b := vm.pop2(); vm.pushBool(uint64(a) < uint64(b)) })
	vm.prim("U>", func(vm *VM) { a, b := vm.pop2(); vm.pushBool(uint64(a) > uint64(b)) })
	vm.prim("0=", func(vm *VM) { vm.pushBool(vm.pop() == 0) })
	vm.prim("0<>", func(vm *VM) { vm.pushBool(vm.pop() != 0) })
	vm.prim("0<", func(vm *VM) { vm.pushBool(vm.pop() < 0) })
	vm.prim("0>", func(vm *VM) { vm.pushBool(vm.pop() > 0) })

	vm.prim("TRUE", func(vm *VM) { vm.push(-1) })
	vm.prim("FALSE", func(vm *VM) { vm.push(0) })
	vm.prim("BL", func(vm *VM) { vm.push(' ') })
}

// floorDiv is floored division, the flavour used by / MOD /MOD and FM/MOD.
func (vm *VM) floorDiv(a, b Cell) (q, r Cell) {
	if b == 0 {
		vm.throw("division by zero")
	}
	q, r = a/b, a%b
	if r != 0 && (r < 0) != (b < 0) {
		q--
		r += b
	}
	return q, r
}

func (vm *VM) shift(n Cell) uint {
	if n < 0 || n > 64 {
		vm.throw("invalid shift count %d", n)
	}
	return uint(n)
}

func (vm *VM) installMemory() {
	vm.prim("@", func(vm *VM) { vm.push(vm.cell(vm.pop())) })
	vm.prim("!", func(vm *VM) { addr := vm.pop(); vm.setCell(addr, vm.pop()) })
	vm.prim("+!", func(vm *VM) { addr := vm.pop(); vm.setCell(addr, vm.cell(addr)+vm.pop()) })
	vm.prim("C@", func(vm *VM) { vm.push(Cell(vm.byteAt(vm.pop()))) })
	vm.prim("C!", func(vm *VM) { addr := vm.pop(); vm.setByte(addr, byte(vm.pop())) })
	vm.prim("2@", func(vm *VM) {
		addr := vm.pop()
		vm.push(vm.cell(addr + CellSize))
		vm.push(vm.cell(addr))
	})
	vm.prim("2!", func(vm *VM) {
		addr := vm.pop()
		vm.setCell(addr, vm.pop())
		vm.setCell(addr+CellSize, vm.pop())
	})
	vm.prim(",", func(vm *VM) { vm.setCell(vm.allot(CellSize), vm.pop()) })
	vm.prim("C,", func(vm *VM) { vm.setByte(vm.allot(1), byte(vm.pop())) })
	vm.prim("ALLOT", func(vm *VM) { vm.allot(vm.pop()) })
	vm.prim("HERE", func(vm *VM) { vm.push(vm.here) })
	vm.prim("UNUSED", func(vm *VM) { vm.push(Cell(len(vm.mem)) - vm.here) })
	vm.prim("ALIGN", func(vm *VM) { vm.allot((CellSize - vm.here%CellSize) % CellSize) })
	vm.prim("ALIGNED", func(vm *VM) {
		a := vm.pop()
		vm.push(a + (CellSize-a%CellSize)%CellSize)
	})
	vm.prim("CELL+", func(vm *VM) { vm.push(vm.pop() + CellSize) })
	vm.prim("CELLS", func(vm *VM) { vm.push(vm.pop() * CellSize) })
	vm.prim("CHAR+", func(vm *VM) { vm.push(vm.pop() + 1) })
	vm.prim("CHARS", func(vm *VM) {})
	vm.prim("FILL", func(vm *VM) {
		c := byte(vm.pop())
		n := vm.pop()
		addr := vm.pop()
		vm.checkAddr(addr, n)
		for i := Cell(0); i < n; i++ {
			vm.mem[addr+i] = c
		}
	})
	vm.prim("ERASE", func(vm *VM) {
		n := vm.pop()
		addr := vm.pop()
		vm.checkAddr(addr, n)
		clear(vm.mem[addr : addr+n])
	})
	vm.prim("MOVE", func(vm *VM) {
		n := vm.pop()
		dst := vm.pop()
		src := vm.pop()
		vm.checkAddr(src, n)
		vm.checkAddr(dst, n)
		copy(vm.mem[dst:dst+n], vm.mem[src:src+n])
	})
	vm.prim("COUNT", func(vm *VM) {
		addr := vm.pop()
		n := Cell(vm.byteAt(addr))
		vm.push(addr + 1)
		vm.push(n)
	})
	vm.prim("BASE", func(vm *VM) { vm.push(vm.baseVar) })
	vm.prim("STATE", func(vm *VM) { vm.push(vm.stateV) })
	vm.prim("PAD", func(vm *VM) { vm.push(vm.pad) })
}
