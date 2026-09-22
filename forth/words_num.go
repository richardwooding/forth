package forth

import "math/bits"

// holdSize is the size of the pictured numeric output buffer.
const holdSize = 256

// installPictured defines the pictured numeric output words. They build a
// string right to left in the hold buffer, which is what D. and friends use to
// print numbers in the current BASE.
func (vm *VM) installPictured() {
	vm.prim("<#", func(vm *VM) { vm.holdPtr = vm.holdBuf + holdSize })
	vm.prim("HOLD", func(vm *VM) { vm.hold(byte(vm.pop())) })
	vm.prim("HOLDS", func(vm *VM) {
		n := vm.pop()
		addr := vm.pop()
		s := vm.readString(addr, n)
		for i := len(s) - 1; i >= 0; i-- {
			vm.hold(s[i])
		}
	})
	vm.prim("#", func(vm *VM) {
		lo, hi := vm.popD()
		qlo, qhi, rem := divModD(lo, hi, uint64(vm.base()))
		vm.hold(digitChar(rem))
		vm.pushD(qlo, qhi)
	})
	vm.prim("#S", func(vm *VM) {
		lo, hi := vm.popD()
		base := uint64(vm.base())
		for {
			var rem uint64
			lo, hi, rem = divModD(lo, hi, base)
			vm.hold(digitChar(rem))
			if lo == 0 && hi == 0 {
				break
			}
		}
		vm.pushD(lo, hi)
	})
	vm.prim("SIGN", func(vm *VM) {
		if vm.pop() < 0 {
			vm.hold('-')
		}
	})
	vm.prim("#>", func(vm *VM) {
		vm.pop()
		vm.pop()
		vm.push(vm.holdPtr)
		vm.push(vm.holdBuf + holdSize - vm.holdPtr)
	})
}

// hold prepends one character to the pictured output buffer.
func (vm *VM) hold(c byte) {
	if vm.holdPtr <= vm.holdBuf {
		vm.throw("pictured numeric output overflow")
	}
	vm.holdPtr--
	vm.setByte(vm.holdPtr, c)
}

func digitChar(d uint64) byte {
	if d < 10 {
		return byte('0' + d)
	}
	return byte('A' + d - 10)
}

// divModD divides the unsigned double (lo, hi) by base.
func divModD(lo, hi, base uint64) (qlo, qhi, rem uint64) {
	if base == 0 {
		panic("forth: zero base")
	}
	qhi, rem = bits.Div64(0, hi, base)
	qlo, rem = bits.Div64(rem, lo, base)
	return qlo, qhi, rem
}
