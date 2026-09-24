package forth

// opcode is the instruction set of the inner interpreter. Colon definitions
// are compiled to a slice of instr; everything that needs to touch the
// instruction pointer (branches, loops, EXIT) is an opcode rather than a
// primitive.
type opcode uint8

const (
	opCall     opcode = iota // call word
	opCompile                // compile a call to word (POSTPONE)
	opLit                    // push val
	opString                 // push addr len of a compiled string
	opBranch                 // ip = dest
	opZBranch                // ip = dest if the popped flag is zero
	opDo                     // start a DO loop
	opQDo                    // ?DO: skip to dest when limit = index
	opLoop                   // LOOP
	opPlusLoop               // +LOOP
	opLeave                  // pop the innermost loop and branch to dest
	opDoes                   // DOES>: attach code[dest:] to the last CREATEd word
	opExit                   // return from the current definition
)

type instr struct {
	op   opcode
	word *Word // opCall
	val  Cell  // opLit, opString address
	len  Cell  // opString length
	dest int   // branch and loop target
}

// Execute runs a word, catching Forth errors.
func (vm *VM) Execute(w *Word) (err error) {
	defer vm.catch(&err)
	vm.exec(w)
	return nil
}

func (vm *VM) exec(w *Word) {
	vm.depth++
	if vm.depth > maxCallDepth {
		vm.throw("call stack overflow (word %s)", w.Name)
	}
	defer func() { vm.depth-- }()

	switch w.kind {
	case kindPrim:
		w.prim(vm)
	case kindData:
		vm.push(w.data)
		if w.does != nil {
			vm.run(w.does)
		}
	case kindConstant:
		for _, v := range w.vals {
			vm.push(v)
		}
	case kindValue:
		vm.push(vm.cell(w.data))
	default:
		vm.run(w.code)
	}
}

// run is the inner interpreter: it executes one compiled code list.
func (vm *VM) run(code []instr) {
	loopBase := len(vm.loops)
	for ip := 0; ip < len(code); {
		in := &code[ip]
		switch in.op {
		case opCall:
			vm.exec(in.word)
			ip++
		case opCompile:
			vm.compile(instr{op: opCall, word: in.word})
			ip++
		case opLit:
			vm.push(in.val)
			ip++
		case opString:
			vm.push(in.val)
			vm.push(in.len)
			ip++
		case opBranch:
			ip = in.dest
		case opZBranch:
			if vm.pop() == 0 {
				ip = in.dest
			} else {
				ip++
			}
		case opDo:
			limit, index := vm.pop2()
			vm.loops = append(vm.loops, loopFrame{index: index, limit: limit})
			ip++
		case opQDo:
			limit, index := vm.pop2()
			if limit == index {
				ip = in.dest
			} else {
				vm.loops = append(vm.loops, loopFrame{index: index, limit: limit})
				ip++
			}
		case opLoop, opPlusLoop:
			step := Cell(1)
			if in.op == opPlusLoop {
				step = vm.pop()
			}
			if len(vm.loops) <= loopBase {
				vm.throw("LOOP without DO")
			}
			f := &vm.loops[len(vm.loops)-1]
			old := f.index - f.limit
			f.index += step
			// Terminate when the index crosses the limit boundary.
			if (old ^ (f.index - f.limit)) < 0 {
				vm.loops = vm.loops[:len(vm.loops)-1]
				ip++
			} else {
				ip = in.dest
			}
		case opLeave:
			if len(vm.loops) > loopBase {
				vm.loops = vm.loops[:len(vm.loops)-1]
			}
			ip = in.dest
		case opDoes:
			if vm.lastDef == nil {
				vm.throw("DOES> without CREATE")
			}
			vm.lastDef.does = code[in.dest:]
			vm.lastDef.kind = kindData
			vm.loops = vm.loops[:loopBase]
			return
		case opExit:
			vm.loops = vm.loops[:loopBase]
			return
		}
	}
	vm.loops = vm.loops[:loopBase]
}

// --- compilation -----------------------------------------------------------

// ctrlKind tags entries on the compile time control-flow stack so that
// mismatched control structures can be reported.
type ctrlKind uint8

const (
	ctrlIf ctrlKind = iota
	ctrlElse
	ctrlBegin
	ctrlWhile
	ctrlDo
	ctrlCase
	ctrlOf
)

func (k ctrlKind) String() string {
	switch k {
	case ctrlIf:
		return "IF"
	case ctrlElse:
		return "ELSE"
	case ctrlBegin:
		return "BEGIN"
	case ctrlWhile:
		return "WHILE"
	case ctrlDo:
		return "DO"
	case ctrlCase:
		return "CASE"
	case ctrlOf:
		return "OF"
	}
	return "?"
}

type ctrlEntry struct {
	kind   ctrlKind
	addr   int   // index of the instruction to patch, or a loop target
	back   int   // backward branch target (ctrlWhile)
	leaves []int // LEAVE and ?DO branches to resolve (ctrlDo)
}

// compile appends an instruction to the definition being compiled and returns
// its index.
func (vm *VM) compile(in instr) int {
	if vm.current == nil {
		vm.throw("no definition in progress")
	}
	vm.current.code = append(vm.current.code, in)
	return len(vm.current.code) - 1
}

func (vm *VM) compilePos() int {
	if vm.current == nil {
		vm.throw("no definition in progress")
	}
	return len(vm.current.code)
}

func (vm *VM) patch(at, dest int) {
	vm.current.code[at].dest = dest
}

func (vm *VM) cpush(e ctrlEntry) { vm.cstack = append(vm.cstack, e) }

func (vm *VM) cpop(want ctrlKind) ctrlEntry {
	n := len(vm.cstack)
	if n == 0 {
		vm.throw("unstructured: expected %v", want)
	}
	e := vm.cstack[n-1]
	if e.kind != want {
		vm.throw("unstructured: expected %v, found %v", want, e.kind)
	}
	vm.cstack = vm.cstack[:n-1]
	return e
}

// ctop returns the innermost control entry of the given kind, or nil.
func (vm *VM) ctop(kind ctrlKind) *ctrlEntry {
	for i := len(vm.cstack) - 1; i >= 0; i-- {
		if vm.cstack[i].kind == kind {
			return &vm.cstack[i]
		}
	}
	return nil
}

// compileOnly guards words that make no sense while interpreting.
func (vm *VM) compileOnly(name string) {
	if !vm.Compiling() {
		vm.throw("%s: compile-only word", name)
	}
}
