package forth_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/richardwooding/forth/forth"
)

func TestCase(t *testing.T) {
	src := `
		: name ( n -- )
			case
				1 of ." one"   endof
				2 of ." two"   endof
				3 of ." three" endof
				." many"
			endcase ;
		: t 1 name space 2 name space 3 name space 7 name ;
		t
	`
	if got := evalOut(t, src); got != "one two three many" {
		t.Errorf("got %q", got)
	}
	// The value under test must be dropped in every branch.
	if got := evalStack(t, src); len(got) != 0 {
		t.Errorf("CASE left %v on the stack", got)
	}
}

func TestCaseErrors(t *testing.T) {
	tests := []struct{ src, want string }{
		{": t case 1 of ." + `" x" ` + "endof ;", "unstructured: CASE left open"},
		{": t 1 of endof endcase ;", "OF outside CASE"},
		{": t case endof endcase ;", "unstructured: expected OF"},
		{"case", "CASE: compile-only word"},
	}
	for _, tt := range tests {
		vm := forth.New(nil, nil)
		err := vm.Eval(tt.src)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s -> %v, want it to contain %q", tt.src, err, tt.want)
		}
	}
}

func TestPicturedOutput(t *testing.T) {
	tests := []struct{ src, want string }{
		{"0 0 <# #S #> TYPE", "0"},
		{"12345 0 <# #S #> TYPE", "12345"},
		{"255 0 HEX <# #S #> TYPE DECIMAL", "FF"},
		{"-1 0 <# #S #> TYPE", "18446744073709551615"},
		{"65 0 <# 2DROP 0 0 #> TYPE", ""},
		{"1234 0 <# # # CHAR . HOLD #S #> TYPE", "12.34"},
		{`12 0 <# #S S" $" HOLDS #> TYPE`, "$12"},
		{"-42 DUP ABS 0 <# #S ROT SIGN #> TYPE", "-42"},
	}
	for _, tt := range tests {
		if got := evalOut(t, tt.src); got != tt.want {
			t.Errorf("%s -> %q, want %q", tt.src, got, tt.want)
		}
	}
	// Overflowing the hold buffer is an error, not a memory corruption.
	vm := forth.New(nil, nil)
	err := vm.Eval(": t <# 300 0 do [char] x hold loop ; t")
	if err == nil || !strings.Contains(err.Error(), "pictured numeric output overflow") {
		t.Errorf("got %v, want an overflow error", err)
	}
}

func TestDoubleArithmetic(t *testing.T) {
	tests := []struct {
		src  string
		want []forth.Cell
	}{
		{"7 S>D", []forth.Cell{7, 0}},
		{"-7 S>D", []forth.Cell{-7, -1}},
		{"5 S>D D>S", []forth.Cell{5}},
		{"-1 0 1 0 D+", []forth.Cell{0, 1}}, // carry into the high cell
		{"0 1 1 0 D-", []forth.Cell{-1, 0}}, // borrow out of the high cell
		{"1 0 DNEGATE", []forth.Cell{-1, -1}},
		{"1 -1 DABS", []forth.Cell{-1, 0}},
		{"1 0 D2*", []forth.Cell{2, 0}},
		{"-1 0 D2*", []forth.Cell{-2, 1}},
		{"0 1 D2/", []forth.Cell{-1 << 63, 0}}, // 2^64 / 2 = 2^63
		{"0 0 D0=", []forth.Cell{-1}},
		{"1 0 D0=", []forth.Cell{0}},
		{"0 -1 D0<", []forth.Cell{-1}},
		{"1 2 1 2 D=", []forth.Cell{-1}},
		{"1 0 2 0 D<", []forth.Cell{-1}},
		{"1 0 -1 -1 D<", []forth.Cell{0}},   // signed: -1 is smaller than 1
		{"1 0 -1 -1 DU<", []forth.Cell{-1}}, // unsigned: it is the largest double
		{"1 0 2 0 DMAX", []forth.Cell{2, 0}},
		{"1 0 2 0 DMIN", []forth.Cell{1, 0}},
		{"10 0 -3 M+", []forth.Cell{7, 0}},
		{"-1 -1 UM*", []forth.Cell{1, -2}}, // 2^64-1 squared
		{"-1 2 M*", []forth.Cell{-2, -1}},  // -1 * 2 = -2
		{"6 7 M*", []forth.Cell{42, 0}},
		{"0 1 10 UM/MOD", []forth.Cell{6, 1844674407370955161}},
		{"-7 S>D 2 FM/MOD", []forth.Cell{1, -4}},  // floored
		{"-7 S>D 2 SM/REM", []forth.Cell{-1, -3}}, // symmetric
		{"7 S>D -2 FM/MOD", []forth.Cell{-1, -4}},
		{"7 S>D -2 SM/REM", []forth.Cell{1, -3}},
	}
	for _, tt := range tests {
		if got := evalStack(t, tt.src); !slices.Equal(got, tt.want) {
			t.Errorf("%s -> %v, want %v", tt.src, got, tt.want)
		}
	}
}

func TestDoubleOutput(t *testing.T) {
	tests := []struct{ src, want string }{
		{"123 0 D.", "123 "},
		{"123 0 DNEGATE D.", "-123 "},
		{"0 1 D.", "18446744073709551616 "}, // 2^64
		{"42 0 8 D.R", "      42"},
		{"-1 0 UD.", "18446744073709551615 "},
	}
	for _, tt := range tests {
		if got := evalOut(t, tt.src); got != tt.want {
			t.Errorf("%s -> %q, want %q", tt.src, got, tt.want)
		}
	}
}

func TestDoubleDefiningWords(t *testing.T) {
	src := `
		1 2 2CONSTANT big big
		2VARIABLE dv 3 4 dv 2! dv 2@
		: t [ 5 6 ] 2LITERAL ; t
		1 0 2 0 3 0 2ROT
	`
	got := evalStack(t, src)
	want := []forth.Cell{1, 2, 3, 4, 5, 6, 2, 0, 3, 0, 1, 0}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDivisionOverflow(t *testing.T) {
	for _, src := range []string{"0 1 1 UM/MOD", "0 1 1 FM/MOD", "0 1 1 SM/REM"} {
		vm := forth.New(nil, nil)
		err := vm.Eval(src)
		if err == nil || !strings.Contains(err.Error(), "overflow") {
			t.Errorf("%s -> %v, want an overflow error", src, err)
		}
	}
}

// fileVM returns a VM with access to a temporary directory.
func fileVM(t *testing.T) (*forth.VM, string) {
	t.Helper()
	dir := t.TempDir()
	vm := forth.New(nil, nil)
	vm.SetFileSystem(forth.OSFileSystem{})
	return vm, dir
}

func TestFileWords(t *testing.T) {
	vm, dir := fileVM(t)
	name := filepath.Join(dir, "data.txt")
	var out strings.Builder
	vm.Out = &out

	src := `
		S" ` + name + `" R/W CREATE-FILE ABORT" create failed" CONSTANT fh
		: put ( addr u -- ) fh WRITE-LINE ABORT" write failed" ;
		: show ( -- ) PAD 80 fh READ-LINE ABORT" read failed"
			IF PAD SWAP TYPE SPACE ELSE DROP THEN ;
		: last ( -- ) PAD 80 fh READ-LINE ABORT" read failed"
			0= IF ." eof" THEN DROP ;
		S" hello" put S" world" put
		fh FLUSH-FILE ABORT" flush failed"
		fh FILE-SIZE ABORT" size failed" D>S .
		0 0 fh REPOSITION-FILE ABORT" reposition failed"
		show show last
		fh CLOSE-FILE ABORT" close failed"
	`
	if err := vm.Eval(src); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got, want := out.String(), "12 hello world eof"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if b, err := os.ReadFile(name); err != nil || string(b) != "hello\nworld\n" {
		t.Errorf("file contents %q, err %v", b, err)
	}
}

func TestFileReadWriteAndDelete(t *testing.T) {
	vm, dir := fileVM(t)
	name := filepath.Join(dir, "raw.bin")
	src := `
		S" ` + name + `" W/O BIN CREATE-FILE ABORT" create failed" CONSTANT fh
		PAD 5 65 FILL PAD 5 fh WRITE-FILE ABORT" write failed"
		fh CLOSE-FILE ABORT" close failed"
		S" ` + name + `" R/O OPEN-FILE ABORT" open failed" CONSTANT fh2
		PAD 5 32 FILL PAD 10 fh2 READ-FILE ABORT" read failed"
		fh2 CLOSE-FILE ABORT" close failed"
	`
	if err := vm.Eval(src); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got := vm.Stack(); len(got) != 1 || got[0] != 5 {
		t.Fatalf("READ-FILE returned %v, want [5]", got)
	}
	if err := vm.Eval(`S" ` + name + `" DELETE-FILE .`); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Errorf("file still exists: %v", err)
	}
}

func TestFileErrors(t *testing.T) {
	vm, dir := fileVM(t)
	var out strings.Builder
	vm.Out = &out
	src := `S" ` + filepath.Join(dir, "nope") + `" R/O OPEN-FILE . DROP FILE-ERROR TYPE`
	if err := vm.Eval(src); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if !strings.HasPrefix(out.String(), "-1 ") || !strings.Contains(out.String(), "no such file") {
		t.Errorf("got %q, want an ior of -1 and a message", out.String())
	}

	// Without a file system the words are present but refuse to do anything.
	denied := forth.New(nil, nil)
	if err := denied.Eval(`S" x" R/O OPEN-FILE`); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got := denied.Stack(); !slices.Equal(got, []forth.Cell{0, -1}) {
		t.Errorf("got %v, want [0 -1]", got)
	}
	if err := denied.Eval("invalid-fid"); err == nil {
		t.Error("expected an error")
	}
	if err := forth.New(nil, nil).Eval("42 CLOSE-FILE"); err == nil {
		t.Error("expected an invalid file identifier error")
	}
}

func TestIncludeFile(t *testing.T) {
	vm, dir := fileVM(t)
	name := filepath.Join(dir, "lib.fs")
	if err := os.WriteFile(name, []byte(": twice 2 * ;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := `S" ` + name + `" R/O OPEN-FILE ABORT" open failed" DUP INCLUDE-FILE CLOSE-FILE DROP 21 twice`
	if err := vm.Eval(src); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got := vm.Stack(); len(got) != 1 || got[0] != 42 {
		t.Errorf("got %v, want [42]", got)
	}
	// INCLUDED falls back to the file system when no loader is installed.
	vm2, _ := fileVM(t)
	if err := vm2.Eval(`S" ` + name + `" INCLUDED 4 twice`); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got := vm2.Stack(); len(got) != 1 || got[0] != 8 {
		t.Errorf("got %v, want [8]", got)
	}
}

func TestTimeWords(t *testing.T) {
	got := evalStack(t, "5 MS TIME&DATE")
	if len(got) != 6 {
		t.Fatalf("TIME&DATE left %d cells, want 6", len(got))
	}
	if year := got[5]; year < 2024 || year > 2200 {
		t.Errorf("year %d looks wrong", year)
	}
	if month := got[4]; month < 1 || month > 12 {
		t.Errorf("month %d out of range", month)
	}
	elapsed := evalStack(t, "MS@ 10 MS MS@ SWAP -")
	if len(elapsed) != 1 || elapsed[0] < 5 {
		t.Errorf("MS@ difference %v, want at least 5 ms", elapsed)
	}
}
