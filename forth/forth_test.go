package forth_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/richardwooding/forth/forth"
)

// evalStack evaluates src and returns the resulting data stack.
func evalStack(t *testing.T, src string) []forth.Cell {
	t.Helper()
	vm := forth.New(nil, nil)
	if err := vm.Eval(src); err != nil {
		t.Fatalf("Eval(%q): %v", src, err)
	}
	return vm.Stack()
}

// evalOut evaluates src and returns everything it printed.
func evalOut(t *testing.T, src string) string {
	t.Helper()
	var out bytes.Buffer
	vm := forth.New(nil, &out)
	if err := vm.Eval(src); err != nil {
		t.Fatalf("Eval(%q): %v", src, err)
	}
	return out.String()
}

func TestStackWords(t *testing.T) {
	tests := []struct {
		src  string
		want []forth.Cell
	}{
		{"1 2 3", []forth.Cell{1, 2, 3}},
		{"1 DUP", []forth.Cell{1, 1}},
		{"1 2 DROP", []forth.Cell{1}},
		{"1 2 SWAP", []forth.Cell{2, 1}},
		{"1 2 OVER", []forth.Cell{1, 2, 1}},
		{"1 2 NIP", []forth.Cell{2}},
		{"1 2 TUCK", []forth.Cell{2, 1, 2}},
		{"1 2 3 ROT", []forth.Cell{2, 3, 1}},
		{"1 2 3 -ROT", []forth.Cell{3, 1, 2}},
		{"1 2 2DUP", []forth.Cell{1, 2, 1, 2}},
		{"1 2 3 4 2SWAP", []forth.Cell{3, 4, 1, 2}},
		{"1 2 3 4 2OVER", []forth.Cell{1, 2, 3, 4, 1, 2}},
		{"1 2 3 DEPTH", []forth.Cell{1, 2, 3, 3}},
		{"0 ?DUP", []forth.Cell{0}},
		{"5 ?DUP", []forth.Cell{5, 5}},
		{"10 20 30 2 PICK", []forth.Cell{10, 20, 30, 10}},
		{"10 20 30 2 ROLL", []forth.Cell{20, 30, 10}},
		{"1 2 >R R@ R>", []forth.Cell{1, 2, 2}},
	}
	for _, tt := range tests {
		if got := evalStack(t, tt.src); !slices.Equal(got, tt.want) {
			t.Errorf("%s -> %v, want %v", tt.src, got, tt.want)
		}
	}
}

func TestArithmetic(t *testing.T) {
	tests := []struct {
		src  string
		want forth.Cell
	}{
		{"2 3 +", 5},
		{"2 3 -", -1},
		{"6 7 *", 42},
		{"7 2 /", 3},
		{"-7 2 /", -4},  // floored division
		{"-7 2 MOD", 1}, // floored remainder
		{"7 -2 MOD", -1},
		{"-7 S>D 2 SM/REM SWAP DROP", -3}, // symmetric division of a double
		{"100 3 7 */", 42},
		{"1 2 3 */MOD DROP", 2},
		{"41 1+", 42},
		{"43 1-", 42},
		{"21 2*", 42},
		{"85 2/", 42},
		{"-42 ABS", 42},
		{"42 NEGATE", -42},
		{"3 4 MIN", 3},
		{"3 4 MAX", 4},
		{"6 3 AND", 2},
		{"6 3 OR", 7},
		{"6 3 XOR", 5},
		{"0 INVERT", -1},
		{"1 6 LSHIFT", 64},
		{"64 6 RSHIFT", 1},
		{"-1 1 RSHIFT 0<", 0}, // logical shift
		{"1 1 =", -1},
		{"1 2 =", 0},
		{"1 2 <", -1},
		{"1 2 >", 0},
		{"-1 1 U<", 0}, // unsigned comparison
		{"0 0=", -1},
		{"5 0<>", -1},
		{"-5 0<", -1},
		{"5 0>", -1},
		{"5 1 10 WITHIN", -1},
		{"10 1 10 WITHIN", 0},
		{"TRUE", -1},
		{"FALSE", 0},
		{"BL", 32},
		{"CELL", 8},
	}
	for _, tt := range tests {
		got := evalStack(t, tt.src)
		if len(got) != 1 || got[0] != tt.want {
			t.Errorf("%s -> %v, want [%d]", tt.src, got, tt.want)
		}
	}
}

func TestNumberBases(t *testing.T) {
	tests := []struct {
		src  string
		want forth.Cell
	}{
		{"$ff", 255},
		{"%1010", 10},
		{"#99", 99},
		{"-42", -42},
		{"'A'", 65},
		{"HEX ff", 255},
		{"HEX 10 DECIMAL", 16},
		{"BINARY 101", 5},
		{"OCTAL 17", 15},
	}
	for _, tt := range tests {
		got := evalStack(t, tt.src)
		if len(got) != 1 || got[0] != tt.want {
			t.Errorf("%s -> %v, want [%d]", tt.src, got, tt.want)
		}
	}
}

func TestDefinitions(t *testing.T) {
	src := `
		: square dup * ;
		: sum-of-squares square swap square + ;
		3 4 sum-of-squares
	`
	got := evalStack(t, src)
	if len(got) != 1 || got[0] != 25 {
		t.Errorf("got %v, want [25]", got)
	}
}

func TestDefinitionIsCaseInsensitive(t *testing.T) {
	got := evalStack(t, ": Double 2 * ; 21 DOUBLE 21 double")
	if want := []forth.Cell{42, 42}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRedefinitionUsesPreviousWord(t *testing.T) {
	// The FOO inside the new definition must refer to the old FOO.
	got := evalStack(t, ": foo 1 ; : foo foo 1 + ; foo")
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("got %v, want [2]", got)
	}
}

func TestRecurse(t *testing.T) {
	src := ": fact dup 1 > if dup 1- recurse * then ; 10 fact"
	got := evalStack(t, src)
	if len(got) != 1 || got[0] != 3628800 {
		t.Errorf("got %v, want [3628800]", got)
	}
}

func TestConditionals(t *testing.T) {
	src := `
		: sign dup 0> if drop 1 else 0< if -1 else 0 then then ;
		5 sign -5 sign 0 sign
	`
	got := evalStack(t, src)
	if want := []forth.Cell{1, -1, 0}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLoops(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"do-loop", ": t 5 0 do i . loop ; t", "0 1 2 3 4 "},
		{"do-loop-j", ": t 2 0 do 2 0 do j i . . loop loop ; t", "0 0 1 0 0 1 1 1 "},
		{"plus-loop", ": t 10 0 do i . 2 +loop ; t", "0 2 4 6 8 "},
		{"count-down", ": t 0 5 do i . -1 +loop ; t", "5 4 3 2 1 0 "},
		{"question-do", ": t 0 0 ?do i . loop 999 . ; t", "999 "},
		{"leave", ": t 100 0 do i . i 3 > if leave then loop 9 . ; t", "0 1 2 3 4 9 "},
		{"unloop-exit", ": t 10 0 do i 2 > if unloop exit then i . loop 9 . ; t", "0 1 2 "},
		{"begin-until", ": t 5 begin dup . 1- dup 0= until drop ; t", "5 4 3 2 1 "},
		{"begin-while", ": t 5 begin dup while dup . 1- repeat drop ; t", "5 4 3 2 1 "},
		{"begin-again-exit", ": t 0 begin dup . 1+ dup 3 = if drop exit then again ; t", "0 1 2 "},
		{"nested-leave", ": t 3 0 do 3 0 do i 1 = if leave then i . loop loop ; t", "0 0 0 "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := evalOut(t, tt.src); got != tt.want {
				t.Errorf("%s -> %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

func TestOutputWords(t *testing.T) {
	tests := []struct{ src, want string }{
		{"42 .", "42 "},
		{"-1 U.", "18446744073709551615 "},
		{"255 HEX . DECIMAL", "FF "},
		{"42 5 .R", "   42"},
		{"1 2 3 .S", "<3> 1 2 3 "},
		{"65 EMIT", "A"},
		{"CR", "\n"},
		{"3 SPACES", "   "},
		{`." hello, world"`, "hello, world"},
		{`: greet ." hi " ." there" ; greet`, "hi there"},
		{`S" abc" TYPE`, "abc"},
		{`: s S" xyz" ; s TYPE`, "xyz"},
		{`C" count" COUNT TYPE`, "count"},
		{"9731 EMIT", "\u2603"},
		{"( a comment ) 1 .", "1 "},
		{"\\ line comment\n2 .", "2 "},
		{`: showit S" quoted" TYPE SPACE ; showit showit`, "quoted quoted "},
	}
	for _, tt := range tests {
		if got := evalOut(t, tt.src); got != tt.want {
			t.Errorf("%s -> %q, want %q", tt.src, got, tt.want)
		}
	}
}

func TestVariablesAndMemory(t *testing.T) {
	src := `
		variable v
		41 v ! 1 v +! v @
		create buf 10 allot
		65 buf c! 66 buf 1+ c! buf c@ buf 1+ c@
		7 constant seven seven
		100 value counter counter 5 to counter counter
	`
	got := evalStack(t, src)
	want := []forth.Cell{42, 65, 66, 7, 100, 5}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMemoryBlockWords(t *testing.T) {
	src := `
		create b 16 allot
		b 16 65 fill
		b 4 + 4 66 fill
		b 16 type
	`
	if got := evalOut(t, src); got != "AAAABBBBAAAAAAAA" {
		t.Errorf("got %q", got)
	}
	src = `create s 8 allot s 8 blank s 8 type`
	if got := evalOut(t, src); got != "        " {
		t.Errorf("got %q", got)
	}
}

func TestCreateDoes(t *testing.T) {
	src := `
		: constant2 create , , does> dup @ swap cell+ @ ;
		2 3 constant2 pair pair
		: array create cells allot does> swap cells + ;
		5 array a
		42 0 a ! 43 4 a !
		0 a @ 4 a @
	`
	got := evalStack(t, src)
	want := []forth.Cell{3, 2, 42, 43}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTickExecuteAndPostpone(t *testing.T) {
	src := `
		: double 2 * ;
		' double execute
		: apply execute ;
		: quad ['] double apply ['] double apply ;
	`
	got := evalStack(t, "21 "+src+" 5 quad")
	if want := []forth.Cell{42, 20}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// POSTPONE of a non-immediate word defers compilation to the new word.
	out := evalOut(t, `: later postpone . ; immediate : show later ; 7 show`)
	if out != "7 " {
		t.Errorf("postpone -> %q, want %q", out, "7 ")
	}
	// POSTPONE of an immediate word compiles its action.
	out = evalOut(t, `: my-if postpone if ; immediate : t my-if ." yes" then ; -1 t 0 t`)
	if out != "yes" {
		t.Errorf("postpone immediate -> %q, want %q", out, "yes")
	}
}

func TestLiteralAndBracket(t *testing.T) {
	got := evalStack(t, ": t [ 6 7 * ] literal ; t")
	if len(got) != 1 || got[0] != 42 {
		t.Errorf("got %v, want [42]", got)
	}
}

func TestEvaluate(t *testing.T) {
	got := evalStack(t, `S" 2 3 +" EVALUATE`)
	if len(got) != 1 || got[0] != 5 {
		t.Errorf("got %v, want [5]", got)
	}
}

func TestCharWords(t *testing.T) {
	got := evalStack(t, "char A char B 2 - : t [char] C ; t")
	want := []forth.Cell{65, 64, 67}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestStateAndImmediate(t *testing.T) {
	got := evalStack(t, ": five 5 ; immediate : t five ; 0 t")
	// FIVE is immediate, so it ran while T was compiled: T compiles nothing
	// and the 5 was left on the stack at compile time.
	if want := []forth.Cell{5, 0}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct{ src, want string }{
		{"nosuchword", "NOSUCHWORD?"},
		{"drop", "stack underflow"},
		{"1 0 /", "division by zero"},
		{"0 @", "invalid memory address 0"},
		{": t ; ;", ";: compile-only word"},
		{"then", "THEN: compile-only word"},
		{": t if ;", "unstructured: IF left open"},
		{": t 1 else then ;", "unstructured: expected IF"},
		{": t loop ;", "unstructured: expected DO"},
		{"' nothere", "NOTHERE?"},
		{"1 0 abort\" boom\"", ""},
		{"-1 abort\" boom\"", "boom"},
		{"abort", "aborted"},
		{"-1 to fail", "FAIL?"},
		{"1 base ! 5 .", "invalid BASE 1"},
		{`S" abc" EVALUATE`, "ABC?"},
		{"i", "loop index used outside a DO loop"},
		{`S" x" INCLUDED`, "no file system installed"},
	}
	for _, tt := range tests {
		vm := forth.New(nil, nil)
		err := vm.Eval(tt.src)
		if tt.want == "" {
			if err != nil {
				t.Errorf("%s -> unexpected error %v", tt.src, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s -> no error, want %q", tt.src, tt.want)
			continue
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s -> %q, want it to contain %q", tt.src, err, tt.want)
		}
	}
}

func TestAbortResetsVM(t *testing.T) {
	vm := forth.New(nil, nil)
	if err := vm.Eval(": broken 1 2 3 nosuch"); err == nil {
		t.Fatal("expected an error")
	}
	// Eval already aborted; a second Abort must be harmless.
	vm.Abort()
	if got := vm.Stack(); len(got) != 0 {
		t.Errorf("stack not cleared: %v", got)
	}
	if vm.Compiling() {
		t.Error("still compiling after Abort")
	}
	if vm.Lookup("broken") != nil {
		t.Error("partial definition survived Abort")
	}
	if err := vm.Eval("2 2 +"); err != nil {
		t.Fatalf("VM unusable after Abort: %v", err)
	}
	if got := vm.Stack(); len(got) != 1 || got[0] != 4 {
		t.Errorf("got %v, want [4]", got)
	}
}

func TestBye(t *testing.T) {
	vm := forth.New(nil, nil)
	err := vm.Eval("1 2 bye 3")
	if !errors.Is(err, forth.ErrBye) {
		t.Fatalf("got %v, want ErrBye", err)
	}
	if got := vm.Stack(); !slices.Equal(got, []forth.Cell{1, 2}) {
		t.Errorf("stack %v, want [1 2]", got)
	}
}

func TestMultiLineDefinition(t *testing.T) {
	vm := forth.New(nil, nil)
	for _, line := range []string{": hypot-sq", "  dup * swap dup *", "  + ;", "3 4 hypot-sq"} {
		if err := vm.Eval(line); err != nil {
			t.Fatalf("Eval(%q): %v", line, err)
		}
	}
	if got := vm.Stack(); len(got) != 1 || got[0] != 25 {
		t.Errorf("got %v, want [25]", got)
	}
}

func TestInputWords(t *testing.T) {
	vm := forth.New(strings.NewReader("hello\nworld\n"), nil)
	var out bytes.Buffer
	vm.Out = &out
	if err := vm.Eval("pad 20 accept pad swap type key ."); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if out.String() != "hello119 " {
		t.Errorf("got %q, want %q", out.String(), "hello119 ")
	}
}

func TestIncluded(t *testing.T) {
	vm := forth.New(nil, nil)
	vm.SetSourceLoader(func(name string) (string, error) {
		if name != "lib.fs" {
			return "", errors.New("no such file: " + name)
		}
		return ": triple 3 * ;", nil
	})
	if err := vm.Eval(`S" lib.fs" INCLUDED 14 triple`); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got := vm.Stack(); len(got) != 1 || got[0] != 42 {
		t.Errorf("got %v, want [42]", got)
	}
	if err := vm.Eval(`INCLUDE missing.fs`); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestForget(t *testing.T) {
	vm := forth.New(nil, nil)
	if err := vm.Eval(": a 1 ; : b 2 ; forget a"); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if vm.Lookup("a") != nil || vm.Lookup("b") != nil {
		t.Error("FORGET did not remove A and B")
	}
	if vm.Lookup("dup") == nil {
		t.Error("FORGET removed too much")
	}
}

func TestWords(t *testing.T) {
	out := evalOut(t, "words")
	for _, name := range []string{"DUP", "SWAP", ":", "WITHIN"} {
		if !strings.Contains(out, name) {
			t.Errorf("WORDS output missing %s", name)
		}
	}
}

func TestDeepRecursionFailsCleanly(t *testing.T) {
	vm := forth.New(nil, nil)
	err := vm.Eval(": forever recurse ; forever")
	if err == nil || !strings.Contains(err.Error(), "call stack overflow") {
		t.Fatalf("got %v, want a call stack overflow error", err)
	}
	vm.Abort()
	if err := vm.Eval("1 1 +"); err != nil {
		t.Fatalf("VM unusable: %v", err)
	}
}

// A slightly larger program, to check the pieces work together.
func TestSieve(t *testing.T) {
	src := `
		1000 constant size
		create flags size allot
		: prime? ( n -- flag ) flags + c@ ;
		: strike ( n -- )       \ clear every multiple of n from n*n up
			dup dup *
			begin dup size < while
				0 over flags + c!
				over +
			repeat 2drop ;
		: sieve
			flags size 1 fill
			size 2 do i prime? if i strike then loop ;
		: count-primes 0 size 2 do i prime? if 1+ then loop ;
		sieve count-primes
	`
	got := evalStack(t, src)
	if len(got) != 1 || got[0] != 168 { // primes below 1000
		t.Errorf("got %v, want [168]", got)
	}
}
