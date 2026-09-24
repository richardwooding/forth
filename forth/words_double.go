package forth

import "math/bits"

// Double cell numbers occupy two stack cells, the low half below the high
// half, and represent a 128 bit two's complement value.

func (vm *VM) popD() (lo, hi uint64) {
	hi = uint64(vm.pop())
	lo = uint64(vm.pop())
	return lo, hi
}

func (vm *VM) pushD(lo, hi uint64) {
	vm.push(Cell(lo))
	vm.push(Cell(hi))
}

func neg128(lo, hi uint64) (uint64, uint64) {
	l, borrow := bits.Sub64(0, lo, 0)
	h, _ := bits.Sub64(0, hi, borrow)
	return l, h
}

func add128(lo1, hi1, lo2, hi2 uint64) (uint64, uint64) {
	lo, carry := bits.Add64(lo1, lo2, 0)
	hi, _ := bits.Add64(hi1, hi2, carry)
	return lo, hi
}

func sub128(lo1, hi1, lo2, hi2 uint64) (uint64, uint64) {
	lo, borrow := bits.Sub64(lo1, lo2, 0)
	hi, _ := bits.Sub64(hi1, hi2, borrow)
	return lo, hi
}

// less128 compares two signed doubles.
func less128(lo1, hi1, lo2, hi2 uint64) bool {
	if int64(hi1) != int64(hi2) {
		return int64(hi1) < int64(hi2)
	}
	return lo1 < lo2
}

// uless128 compares two unsigned doubles.
func uless128(lo1, hi1, lo2, hi2 uint64) bool {
	if hi1 != hi2 {
		return hi1 < hi2
	}
	return lo1 < lo2
}

// magnitude returns the absolute value of n as an unsigned cell.
func magnitude(n Cell) uint64 {
	u := uint64(n)
	if n < 0 {
		u = -u
	}
	return u
}

func (vm *VM) installDouble() {
	vm.prim("S>D", func(vm *VM) {
		n := vm.pop()
		vm.push(n)
		vm.push(n >> 63)
	})
	vm.prim("D>S", func(vm *VM) { lo, _ := vm.popD(); vm.push(Cell(lo)) })
	vm.prim("D+", func(vm *VM) {
		lo2, hi2 := vm.popD()
		lo1, hi1 := vm.popD()
		vm.pushD(add128(lo1, hi1, lo2, hi2))
	})
	vm.prim("D-", func(vm *VM) {
		lo2, hi2 := vm.popD()
		lo1, hi1 := vm.popD()
		vm.pushD(sub128(lo1, hi1, lo2, hi2))
	})
	vm.prim("DNEGATE", func(vm *VM) { vm.pushD(neg128(vm.popD())) })
	vm.prim("DABS", func(vm *VM) {
		lo, hi := vm.popD()
		if int64(hi) < 0 {
			lo, hi = neg128(lo, hi)
		}
		vm.pushD(lo, hi)
	})
	vm.prim("D2*", func(vm *VM) {
		lo, hi := vm.popD()
		vm.pushD(lo<<1, hi<<1|lo>>63)
	})
	vm.prim("D2/", func(vm *VM) {
		lo, hi := vm.popD()
		vm.pushD(lo>>1|hi<<63, uint64(int64(hi)>>1))
	})
	vm.prim("D0=", func(vm *VM) { lo, hi := vm.popD(); vm.pushBool(lo == 0 && hi == 0) })
	vm.prim("D0<", func(vm *VM) { _, hi := vm.popD(); vm.pushBool(int64(hi) < 0) })
	vm.prim("D=", func(vm *VM) {
		lo2, hi2 := vm.popD()
		lo1, hi1 := vm.popD()
		vm.pushBool(lo1 == lo2 && hi1 == hi2)
	})
	vm.prim("D<", func(vm *VM) {
		lo2, hi2 := vm.popD()
		lo1, hi1 := vm.popD()
		vm.pushBool(less128(lo1, hi1, lo2, hi2))
	})
	vm.prim("DU<", func(vm *VM) {
		lo2, hi2 := vm.popD()
		lo1, hi1 := vm.popD()
		vm.pushBool(uless128(lo1, hi1, lo2, hi2))
	})
	vm.prim("DMAX", func(vm *VM) {
		lo2, hi2 := vm.popD()
		lo1, hi1 := vm.popD()
		if less128(lo1, hi1, lo2, hi2) {
			vm.pushD(lo2, hi2)
		} else {
			vm.pushD(lo1, hi1)
		}
	})
	vm.prim("DMIN", func(vm *VM) {
		lo2, hi2 := vm.popD()
		lo1, hi1 := vm.popD()
		if less128(lo1, hi1, lo2, hi2) {
			vm.pushD(lo1, hi1)
		} else {
			vm.pushD(lo2, hi2)
		}
	})
	vm.prim("M+", func(vm *VM) {
		n := vm.pop()
		lo, hi := vm.popD()
		vm.pushD(add128(lo, hi, uint64(n), uint64(n>>63)))
	})
	vm.prim("M*", func(vm *VM) {
		a, b := vm.pop2()
		hi, lo := bits.Mul64(magnitude(a), magnitude(b))
		if (a < 0) != (b < 0) {
			lo, hi = neg128(lo, hi)
		}
		vm.pushD(lo, hi)
	})
	vm.prim("UM*", func(vm *VM) {
		a, b := vm.pop2()
		hi, lo := bits.Mul64(uint64(a), uint64(b))
		vm.pushD(lo, hi)
	})
	vm.prim("UM/MOD", func(vm *VM) {
		n := uint64(vm.pop())
		lo, hi := vm.popD()
		if n == 0 {
			vm.throw("division by zero")
		}
		if hi >= n {
			vm.throw("division overflow")
		}
		q, r := bits.Div64(hi, lo, n)
		vm.push(Cell(r))
		vm.push(Cell(q))
	})
	vm.prim("SM/REM", func(vm *VM) {
		n := vm.pop()
		lo, hi := vm.popD()
		rem, quot := vm.symDiv(lo, hi, n)
		vm.push(rem)
		vm.push(quot)
	})
	vm.prim("FM/MOD", func(vm *VM) {
		n := vm.pop()
		lo, hi := vm.popD()
		rem, quot := vm.symDiv(lo, hi, n)
		if rem != 0 && (rem < 0) != (n < 0) {
			quot--
			rem += n
		}
		vm.push(rem)
		vm.push(quot)
	})

	vm.prim("2CONSTANT", func(vm *VM) {
		name := vm.nextName("2CONSTANT")
		lo, hi := vm.popD()
		vm.define(&Word{Name: name, kind: kindConstant, vals: []Cell{Cell(lo), Cell(hi)}})
	})
	vm.prim("2VARIABLE", func(vm *VM) {
		name := vm.nextName("2VARIABLE")
		vm.alignHere()
		addr := vm.allot(2 * CellSize)
		vm.setCell(addr, 0)
		vm.setCell(addr+CellSize, 0)
		vm.lastDef = vm.define(&Word{Name: name, kind: kindData, data: addr})
	})
	vm.imm("2LITERAL", func(vm *VM) {
		vm.compileOnly("2LITERAL")
		lo, hi := vm.popD()
		vm.compile(instr{op: opLit, val: Cell(lo)})
		vm.compile(instr{op: opLit, val: Cell(hi)})
	})
}

// symDiv divides the signed double (lo, hi) by n, truncating towards zero,
// which is what SM/REM needs.
func (vm *VM) symDiv(lo, hi uint64, n Cell) (rem, quot Cell) {
	if n == 0 {
		vm.throw("division by zero")
	}
	negD := int64(hi) < 0
	if negD {
		lo, hi = neg128(lo, hi)
	}
	un := magnitude(n)
	if hi >= un {
		vm.throw("division overflow")
	}
	q, r := bits.Div64(hi, lo, un)
	negQ := negD != (n < 0)
	if (negQ && q > 1<<63) || (!negQ && q > 1<<63-1) {
		vm.throw("division overflow")
	}
	quot, rem = Cell(q), Cell(r)
	if negQ {
		quot = -quot
	}
	if negD {
		rem = -rem
	}
	return rem, quot
}
