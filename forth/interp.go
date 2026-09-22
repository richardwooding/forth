package forth

import "strings"

// Eval interprets src, which may span several lines and may leave the VM in
// compiling state (the REPL relies on that). A Forth level error is returned
// and, like ABORT, resets the VM: the stacks are emptied and a partial
// definition is discarded.
func (vm *VM) Eval(src string) error { return vm.EvalNamed(src, "") }

// EvalNamed is Eval with a source name used in error messages.
func (vm *VM) EvalNamed(src, name string) error {
	err := vm.eval(src, name)
	if err != nil && err != ErrBye {
		vm.Abort()
	}
	return err
}

func (vm *VM) eval(src, name string) (err error) {
	defer vm.catch(&err)
	vm.pushInput(src, name)
	defer vm.popInput()
	vm.interpret()
	return nil
}

func (vm *VM) interpret() {
	for {
		name, ok := vm.parseName()
		if !ok {
			return
		}
		if w := vm.Lookup(name); w != nil {
			if vm.Compiling() && !w.Immediate {
				vm.compile(instr{op: opCall, word: w})
			} else {
				vm.exec(w)
			}
			continue
		}
		if v, ok := vm.number(name); ok {
			if vm.Compiling() {
				vm.compile(instr{op: opLit, val: v})
			} else {
				vm.push(v)
			}
			continue
		}
		vm.throw("%s?", strings.ToUpper(name))
	}
}

// --- input source ----------------------------------------------------------

func (vm *VM) in() *input {
	if len(vm.inputs) == 0 {
		vm.throw("no input source")
	}
	return &vm.inputs[len(vm.inputs)-1]
}

func (vm *VM) pushInput(src, name string) {
	vm.inputs = append(vm.inputs, input{buf: src, name: name})
}

// popInput drops the current input source. It is called from defers, so it
// tolerates an already empty stack.
func (vm *VM) popInput() {
	if len(vm.inputs) > 0 {
		vm.inputs = vm.inputs[:len(vm.inputs)-1]
	}
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// parseName skips leading whitespace and returns the next blank delimited
// word. ok is false at the end of the input source.
func (vm *VM) parseName() (name string, ok bool) {
	in := vm.in()
	for in.pos < len(in.buf) && isSpace(in.buf[in.pos]) {
		in.pos++
	}
	if in.pos >= len(in.buf) {
		return "", false
	}
	start := in.pos
	for in.pos < len(in.buf) && !isSpace(in.buf[in.pos]) {
		in.pos++
	}
	name = in.buf[start:in.pos]
	if in.pos < len(in.buf) {
		in.pos++ // the parse area advances past the delimiter
	}
	return name, true
}

// nextName is parseName but errors out at the end of the input, for words such
// as : and ' that require a following name.
func (vm *VM) nextName(who string) string {
	name, ok := vm.parseName()
	if !ok {
		vm.throw("%s expects a name", who)
	}
	return name
}

// parse returns the text up to the next occurrence of delim, consuming the
// delimiter. The text may be empty. ok is false if the delimiter is missing.
func (vm *VM) parse(delim byte) (text string, ok bool) {
	in := vm.in()
	start := in.pos
	for in.pos < len(in.buf) {
		if in.buf[in.pos] == delim {
			text = in.buf[start:in.pos]
			in.pos++
			return text, true
		}
		in.pos++
	}
	return in.buf[start:], false
}

func (vm *VM) parseDelimited(who string, delim byte) string {
	text, ok := vm.parse(delim)
	if !ok {
		vm.throw("%s: missing closing %q", who, rune(delim))
	}
	return text
}

// --- numbers ---------------------------------------------------------------

// base returns the current numeric base, validated.
func (vm *VM) base() int {
	b := vm.cell(vm.baseVar)
	if b < 2 || b > 36 {
		vm.throw("invalid BASE %d", b)
	}
	return int(b)
}

// number converts a string to a cell using the current BASE. It understands a
// leading minus sign, the standard $ # % base prefixes and 'c' character
// literals.
func (vm *VM) number(s string) (Cell, bool) {
	if s == "" {
		return 0, false
	}
	if len(s) == 3 && s[0] == '\'' && s[2] == '\'' {
		return Cell(s[1]), true
	}
	base := vm.base()
	switch s[0] {
	case '$':
		base, s = 16, s[1:]
	case '#':
		base, s = 10, s[1:]
	case '%':
		base, s = 2, s[1:]
	}
	neg := false
	if len(s) > 1 && (s[0] == '-' || s[0] == '+') {
		neg = s[0] == '-'
		s = s[1:]
	}
	if s == "" {
		return 0, false
	}
	var v Cell
	for i := 0; i < len(s); i++ {
		d := digit(s[i])
		if d < 0 || d >= base {
			return 0, false
		}
		v = v*Cell(base) + Cell(d)
	}
	if neg {
		v = -v
	}
	return v, true
}

func digit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	}
	return -1
}

// format renders v in the current base, like Forth's . does.
func (vm *VM) format(v Cell) string { return formatBase(v, vm.base()) }

func formatBase(v Cell, base int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	u := uint64(v)
	if neg {
		u = uint64(-v)
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	var buf [72]byte
	i := len(buf)
	for u > 0 {
		i--
		buf[i] = digits[u%uint64(base)]
		u /= uint64(base)
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return strings.ToUpper(string(buf[i:]))
}

func formatUnsigned(v Cell, base int) string {
	u := uint64(v)
	if u == 0 {
		return "0"
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	var buf [72]byte
	i := len(buf)
	for u > 0 {
		i--
		buf[i] = digits[u%uint64(base)]
		u /= uint64(base)
	}
	return strings.ToUpper(string(buf[i:]))
}
