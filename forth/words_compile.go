package forth

import "strings"

func (vm *VM) installCompile() {
	vm.prim(":", func(vm *VM) {
		if vm.Compiling() {
			vm.throw(": inside a definition")
		}
		name := vm.nextName(":")
		w := vm.define(&Word{Name: name, hidden: true})
		vm.current = w
		vm.setCell(vm.stateV, 1)
	})
	vm.imm(";", func(vm *VM) {
		vm.compileOnly(";")
		if len(vm.cstack) > 0 {
			vm.throw("unstructured: %v left open", vm.cstack[len(vm.cstack)-1].kind)
		}
		vm.compile(instr{op: opExit})
		vm.current.hidden = false
		vm.current = nil
		vm.setCell(vm.stateV, 0)
	})
	vm.imm("[", func(vm *VM) { vm.setCell(vm.stateV, 0) })
	vm.prim("]", func(vm *VM) {
		if vm.current == nil {
			vm.throw("]: no definition in progress")
		}
		vm.setCell(vm.stateV, 1)
	})
	vm.prim("IMMEDIATE", func(vm *VM) {
		w := vm.latest()
		w.Immediate = true
	})
	vm.imm("LITERAL", func(vm *VM) {
		vm.compileOnly("LITERAL")
		vm.compile(instr{op: opLit, val: vm.pop()})
	})
	vm.imm("RECURSE", func(vm *VM) {
		vm.compileOnly("RECURSE")
		vm.compile(instr{op: opCall, word: vm.current})
	})
	vm.imm("POSTPONE", func(vm *VM) {
		vm.compileOnly("POSTPONE")
		name := vm.nextName("POSTPONE")
		w := vm.mustFind(name)
		if w.Immediate {
			vm.compile(instr{op: opCall, word: w})
			return
		}
		// Compile code that compiles a call to w when it is later executed.
		vm.compile(instr{op: opCall, word: &Word{
			Name: "(postpone " + w.Name + ")",
			prim: func(vm *VM) { vm.compile(instr{op: opCall, word: w}) },
		}})
	})

	tick := func(vm *VM, who string) {
		w := vm.mustFind(vm.nextName(who))
		if vm.Compiling() {
			vm.compile(instr{op: opLit, val: w.xt})
		} else {
			vm.push(w.xt)
		}
	}
	vm.imm("'", func(vm *VM) { tick(vm, "'") })
	vm.imm("[']", func(vm *VM) { vm.compileOnly("[']"); tick(vm, "[']") })
	vm.prim("EXECUTE", func(vm *VM) { vm.exec(vm.wordFromXT(vm.pop())) })

	vm.prim("VARIABLE", func(vm *VM) {
		name := vm.nextName("VARIABLE")
		vm.alignHere()
		addr := vm.allot(CellSize)
		vm.setCell(addr, 0)
		vm.lastDef = vm.define(&Word{Name: name, data: addr, hasData: true})
	})
	vm.prim("CONSTANT", func(vm *VM) {
		name := vm.nextName("CONSTANT")
		v := vm.pop()
		vm.define(&Word{Name: name, prim: func(vm *VM) { vm.push(v) }})
	})
	vm.prim("CREATE", func(vm *VM) {
		name := vm.nextName("CREATE")
		vm.alignHere()
		vm.lastDef = vm.define(&Word{Name: name, data: vm.here, hasData: true})
	})
	vm.imm("DOES>", func(vm *VM) {
		vm.compileOnly("DOES>")
		at := vm.compile(instr{op: opDoes})
		vm.patch(at, at+1)
	})
	vm.prim("VALUE", func(vm *VM) {
		name := vm.nextName("VALUE")
		vm.alignHere()
		addr := vm.allot(CellSize)
		vm.setCell(addr, vm.pop())
		vm.define(&Word{
			Name: name, data: addr, isValue: true,
			prim: func(vm *VM) { vm.push(vm.cell(addr)) },
		})
	})
	vm.imm("TO", func(vm *VM) {
		name := vm.nextName("TO")
		w := vm.mustFind(name)
		if !w.isValue {
			vm.throw("TO: %s is not a VALUE", strings.ToUpper(name))
		}
		addr := w.data
		if vm.Compiling() {
			vm.compile(instr{op: opCall, word: &Word{
				Name: "(to " + w.Name + ")",
				prim: func(vm *VM) { vm.setCell(addr, vm.pop()) },
			}})
		} else {
			vm.setCell(addr, vm.pop())
		}
	})
	vm.prim(">BODY", func(vm *VM) {
		w := vm.wordFromXT(vm.pop())
		if !w.hasData && !w.isValue {
			vm.throw(">BODY: %s has no data field", w.Name)
		}
		vm.push(w.data)
	})
	vm.prim("FORGET", func(vm *VM) {
		name := vm.nextName("FORGET")
		w := vm.mustFind(name)
		for i := len(vm.order) - 1; i >= 0; i-- {
			victim := vm.order[i]
			vm.forget(victim)
			if victim == w {
				break
			}
		}
	})
}

// latest returns the most recently defined word.
func (vm *VM) latest() *Word {
	if len(vm.order) == 0 {
		vm.throw("dictionary is empty")
	}
	return vm.order[len(vm.order)-1]
}

func (vm *VM) mustFind(name string) *Word {
	w := vm.Lookup(name)
	if w == nil {
		vm.throw("%s?", strings.ToUpper(name))
	}
	return w
}

func (vm *VM) alignHere() { vm.allot((CellSize - vm.here%CellSize) % CellSize) }

func (vm *VM) installControl() {
	vm.imm("IF", func(vm *VM) {
		vm.compileOnly("IF")
		vm.cpush(ctrlEntry{kind: ctrlIf, addr: vm.compile(instr{op: opZBranch})})
	})
	vm.imm("ELSE", func(vm *VM) {
		vm.compileOnly("ELSE")
		e := vm.cpop(ctrlIf)
		at := vm.compile(instr{op: opBranch})
		vm.patch(e.addr, vm.compilePos())
		vm.cpush(ctrlEntry{kind: ctrlElse, addr: at})
	})
	vm.imm("THEN", func(vm *VM) {
		vm.compileOnly("THEN")
		n := len(vm.cstack)
		if n == 0 || (vm.cstack[n-1].kind != ctrlIf && vm.cstack[n-1].kind != ctrlElse) {
			vm.throw("unstructured: THEN without IF")
		}
		e := vm.cstack[n-1]
		vm.cstack = vm.cstack[:n-1]
		vm.patch(e.addr, vm.compilePos())
	})
	vm.imm("BEGIN", func(vm *VM) {
		vm.compileOnly("BEGIN")
		vm.cpush(ctrlEntry{kind: ctrlBegin, addr: vm.compilePos()})
	})
	vm.imm("UNTIL", func(vm *VM) {
		vm.compileOnly("UNTIL")
		e := vm.cpop(ctrlBegin)
		vm.compile(instr{op: opZBranch, dest: e.addr})
	})
	vm.imm("AGAIN", func(vm *VM) {
		vm.compileOnly("AGAIN")
		e := vm.cpop(ctrlBegin)
		vm.compile(instr{op: opBranch, dest: e.addr})
	})
	vm.imm("WHILE", func(vm *VM) {
		vm.compileOnly("WHILE")
		e := vm.cpop(ctrlBegin)
		vm.cpush(ctrlEntry{
			kind: ctrlWhile,
			addr: vm.compile(instr{op: opZBranch}),
			back: e.addr,
		})
	})
	vm.imm("REPEAT", func(vm *VM) {
		vm.compileOnly("REPEAT")
		e := vm.cpop(ctrlWhile)
		vm.compile(instr{op: opBranch, dest: e.back})
		vm.patch(e.addr, vm.compilePos())
	})
	vm.imm("DO", func(vm *VM) {
		vm.compileOnly("DO")
		vm.compile(instr{op: opDo})
		vm.cpush(ctrlEntry{kind: ctrlDo, addr: vm.compilePos()})
	})
	vm.imm("?DO", func(vm *VM) {
		vm.compileOnly("?DO")
		at := vm.compile(instr{op: opQDo})
		vm.cpush(ctrlEntry{kind: ctrlDo, addr: vm.compilePos(), leaves: []int{at}})
	})
	loopEnd := func(vm *VM, op opcode) {
		e := vm.cpop(ctrlDo)
		vm.compile(instr{op: op, dest: e.addr})
		after := vm.compilePos()
		for _, at := range e.leaves {
			vm.patch(at, after)
		}
	}
	vm.imm("LOOP", func(vm *VM) { vm.compileOnly("LOOP"); loopEnd(vm, opLoop) })
	vm.imm("+LOOP", func(vm *VM) { vm.compileOnly("+LOOP"); loopEnd(vm, opPlusLoop) })
	vm.imm("LEAVE", func(vm *VM) {
		vm.compileOnly("LEAVE")
		e := vm.ctop(ctrlDo)
		if e == nil {
			vm.throw("LEAVE outside a DO loop")
		}
		e.leaves = append(e.leaves, vm.compile(instr{op: opLeave}))
	})
	vm.imm("EXIT", func(vm *VM) {
		vm.compileOnly("EXIT")
		vm.compile(instr{op: opExit})
	})
	vm.prim("UNLOOP", func(vm *VM) {
		if len(vm.loops) == 0 {
			vm.throw("UNLOOP without DO")
		}
		vm.loops = vm.loops[:len(vm.loops)-1]
	})
	vm.prim("I", func(vm *VM) { vm.push(vm.loopIndex(0)) })
	vm.prim("J", func(vm *VM) { vm.push(vm.loopIndex(1)) })

	// CASE ... OF ... ENDOF ... ENDCASE compiles to the same branches as
	// nested IFs: OF is OVER = IF DROP, ENDOF is ELSE, and ENDCASE drops the
	// value that was being compared.
	over, equals, drop := vm.mustFind("OVER"), vm.mustFind("="), vm.mustFind("DROP")
	vm.imm("CASE", func(vm *VM) {
		vm.compileOnly("CASE")
		vm.cpush(ctrlEntry{kind: ctrlCase})
	})
	vm.imm("OF", func(vm *VM) {
		vm.compileOnly("OF")
		if vm.ctop(ctrlCase) == nil {
			vm.throw("OF outside CASE")
		}
		vm.compile(instr{op: opCall, word: over})
		vm.compile(instr{op: opCall, word: equals})
		at := vm.compile(instr{op: opZBranch})
		vm.compile(instr{op: opCall, word: drop})
		vm.cpush(ctrlEntry{kind: ctrlOf, addr: at})
	})
	vm.imm("ENDOF", func(vm *VM) {
		vm.compileOnly("ENDOF")
		e := vm.cpop(ctrlOf)
		at := vm.compile(instr{op: opBranch})
		vm.patch(e.addr, vm.compilePos())
		c := vm.ctop(ctrlCase)
		if c == nil {
			vm.throw("ENDOF outside CASE")
		}
		c.leaves = append(c.leaves, at)
	})
	vm.imm("ENDCASE", func(vm *VM) {
		vm.compileOnly("ENDCASE")
		e := vm.cpop(ctrlCase)
		vm.compile(instr{op: opCall, word: drop})
		after := vm.compilePos()
		for _, at := range e.leaves {
			vm.patch(at, after)
		}
	})
}

func (vm *VM) loopIndex(n int) Cell {
	if len(vm.loops) <= n {
		vm.throw("loop index used outside a DO loop")
	}
	return vm.loops[len(vm.loops)-1-n].index
}
