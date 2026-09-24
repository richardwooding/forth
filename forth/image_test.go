package forth_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/richardwooding/forth/forth"
)

// roundTrip saves the VM to an image, loads it into a fresh one and returns
// the new VM.
func roundTrip(t *testing.T, vm *forth.VM) *forth.VM {
	t.Helper()
	var img bytes.Buffer
	if err := vm.SaveImage(&img); err != nil {
		t.Fatalf("SaveImage: %v", err)
	}
	loaded := forth.New(nil, nil)
	if err := loaded.LoadImage(bytes.NewReader(img.Bytes())); err != nil {
		t.Fatalf("LoadImage: %v", err)
	}
	return loaded
}

// TestImageRoundTrip saves a program that uses every kind of word, then checks
// that the loaded image behaves the same as the VM it came from.
func TestImageRoundTrip(t *testing.T) {
	src := `
		\ colon definitions, control flow and nesting
		: square dup * ;
		: sum-of-squares square swap square + ;
		: classify dup 0> if drop 1 else 0< if -1 else 0 then then ;
		: countdown 0 swap begin dup . 1- dup 0< until drop drop ;
		: fact dup 1 > if dup 1- recurse * then ;
		: table 4 0 do i square . loop ;

		\ data and constants of every kind
		42 constant answer
		variable counter  7 counter !
		100 value budget
		2 3 2constant pair
		2variable dv  11 22 dv 2!
		create buf 16 allot  65 buf c!  66 buf 1+ c!

		\ a defining word, so DOES> code has to survive too
		: array create cells allot does> swap cells + ;
		5 array data  10 0 data !  20 1 data !

		\ strings and messages compiled into definitions
		: greet ." hello from the image" ;
		: name S" forth" ;
		: guard 0= abort" guard tripped" ;

		\ words that compile things
		: later postpone . ; immediate
		: show later ;
		: bump 1 budget + to budget ;
		: xt-of ['] square ;
	`
	vm := forth.New(nil, nil)
	if err := vm.Eval(src); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	loaded := roundTrip(t, vm)

	checks := []struct{ src, want string }{
		{"3 4 sum-of-squares .", "25 "},
		{"-9 classify .", "-1 "},
		{"3 countdown", "3 2 1 0 "},
		{"10 fact .", "3628800 "},
		{"table", "0 1 4 9 "},
		{"answer .", "42 "},
		{"counter ? counter @ 1+ counter ! counter ?", "7 8 "},
		{"budget . bump budget .", "100 101 "},
		{"pair . .", "3 2 "},
		{"dv 2@ . .", "22 11 "},
		{"buf c@ emit buf 1+ c@ emit", "AB"},
		{"0 data @ . 1 data @ .", "10 20 "},
		{"greet", "hello from the image"},
		{"name type", "forth"},
		{"7 show", "7 "},
		{"xt-of 9 swap execute .", "81 "},
		{"answer 2 base ! . decimal", "101010 "},
	}
	for _, c := range checks {
		if got := evalOutVM(t, loaded, c.src); got != c.want {
			t.Errorf("after loading, %q printed %q, want %q", c.src, got, c.want)
		}
	}

	// ABORT" keeps its message.
	if err := loaded.Eval("0 guard"); err == nil || !strings.Contains(err.Error(), "guard tripped") {
		t.Errorf(`ABORT" after loading: %v, want "guard tripped"`, err)
	}
	if err := loaded.Eval("1 guard"); err != nil {
		t.Errorf("guard with a true flag: %v", err)
	}

	// The loaded VM is a working interpreter, not a frozen one.
	if err := loaded.Eval(": cube dup square * ; 3 cube"); err != nil {
		t.Fatalf("defining a word after loading: %v", err)
	}
	if got := loaded.Stack(); !slices.Equal(got, []forth.Cell{27}) {
		t.Errorf("3 cube -> %v, want [27]", got)
	}
}

// evalOutVM is evalOut for a VM that already exists.
func evalOutVM(t *testing.T, vm *forth.VM, src string) string {
	t.Helper()
	var out bytes.Buffer
	old := vm.Out
	vm.Out = &out
	defer func() { vm.Out = old }()
	if err := vm.Eval(src); err != nil {
		t.Fatalf("Eval(%q): %v", src, err)
	}
	return out.String()
}

func TestImageDataSpace(t *testing.T) {
	vm := forth.New(nil, nil)
	if err := vm.Eval(`create blob 32 allot blob 32 char * fill here`); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	here := vm.Stack()[0]
	loaded := roundTrip(t, vm)
	if got := evalOutVM(t, loaded, "blob 32 type here ."); !strings.HasPrefix(got, strings.Repeat("*", 32)) {
		t.Errorf("data space did not survive: %q", got)
	}
	if err := loaded.Eval("here"); err != nil {
		t.Fatal(err)
	}
	if got := loaded.Stack(); len(got) == 0 || got[len(got)-1] != here {
		t.Errorf("HERE is %v, want %d", got, here)
	}
	// BASE lives in data space, so it travels with the image.
	if err := vm.Eval("hex"); err != nil {
		t.Fatal(err)
	}
	if got := evalOutVM(t, roundTrip(t, vm), "base @ decimal ."); got != "16 " {
		t.Errorf("BASE did not survive: %q, want %q", got, "16 ")
	}
}

func TestImageAfterForget(t *testing.T) {
	// FORGET drops the word and everything defined after it; an image written
	// afterwards must not bring them back.
	vm := forth.New(nil, nil)
	if err := vm.Eval(": keep 6 7 * ; : a 1 ; : b a . ; forget a"); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	loaded := roundTrip(t, vm)
	if loaded.Lookup("a") != nil || loaded.Lookup("b") != nil {
		t.Error("a forgotten word came back into the dictionary")
	}
	if got := evalOutVM(t, loaded, "keep ."); got != "42 " {
		t.Errorf("keep printed %q, want %q", got, "42 ")
	}
}

func TestImageRedefinitionKeepsBothWords(t *testing.T) {
	vm := forth.New(nil, nil)
	if err := vm.Eval(": v 1 ; : uses-old v ; : v 2 ;"); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	loaded := roundTrip(t, vm)
	if got := evalOutVM(t, loaded, "uses-old . v ."); got != "1 2 " {
		t.Errorf("got %q, want %q", got, "1 2 ")
	}
}

func TestImageErrors(t *testing.T) {
	vm := forth.New(nil, nil)

	// Saving mid-definition is refused.
	if err := vm.Eval(": half-done 1 2"); err != nil {
		t.Fatal(err)
	}
	if err := vm.SaveImage(&bytes.Buffer{}); err == nil ||
		!strings.Contains(err.Error(), "while compiling") {
		t.Errorf("SaveImage while compiling: %v", err)
	}
	vm.Abort()

	var img bytes.Buffer
	if err := vm.SaveImage(&img); err != nil {
		t.Fatalf("SaveImage: %v", err)
	}

	// Loading into a dictionary that has definitions is refused.
	dirty := forth.New(nil, nil)
	if err := dirty.Eval(": mine 1 ;"); err != nil {
		t.Fatal(err)
	}
	if err := dirty.LoadImage(bytes.NewReader(img.Bytes())); err == nil ||
		!strings.Contains(err.Error(), "already has definitions") {
		t.Errorf("LoadImage into a used VM: %v", err)
	}

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", nil, "not a Forth image"},
		{"garbage", []byte("hello there, this is not an image at all"), "not a Forth image"},
		{"truncated", img.Bytes()[:20], "truncated Forth image"},
		{"bad version", patch(img.Bytes(), 8, []byte{9, 0}), "image version 9"},
		{"bad cell size", patch(img.Bytes(), 10, []byte{4}), "4 byte cells"},
		{"bad baseline count", patch(img.Bytes(), 11, []byte{7, 0, 0, 0}), "expects 7 standard words"},
		{"bad baseline hash", patch(img.Bytes(), 15, []byte{1, 2, 3, 4, 5, 6, 7, 8}), "different version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := forth.New(nil, nil).LoadImage(bytes.NewReader(tt.data))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

// patch returns a copy of data with replacement written at offset.
func patch(data []byte, offset int, replacement []byte) []byte {
	out := bytes.Clone(data)
	copy(out[offset:], replacement)
	return out
}

func TestSaveAndLoadImageWords(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "app.img")

	vm := forth.New(nil, nil)
	vm.SetFileSystem(forth.OSFileSystem{})
	src := `: triple 3 * ; 14 value start S" ` + name + `" SAVE-IMAGE`
	if err := vm.Eval(src); err != nil {
		t.Fatalf("SAVE-IMAGE: %v", err)
	}
	if info, err := os.Stat(name); err != nil || info.Size() == 0 {
		t.Fatalf("image file: %v", err)
	}

	loaded := forth.New(nil, nil)
	loaded.SetFileSystem(forth.OSFileSystem{})
	if got := evalOutVM(t, loaded, `S" `+name+`" LOAD-IMAGE start triple .`); got != "42 " {
		t.Errorf("got %q, want %q", got, "42 ")
	}

	// Without a file system the words refuse to run.
	denied := forth.New(nil, nil)
	if err := denied.Eval(`S" x.img" SAVE-IMAGE`); err == nil ||
		!strings.Contains(err.Error(), "no file system") {
		t.Errorf("SAVE-IMAGE without a file system: %v", err)
	}
	if err := denied.Eval(`S" x.img" LOAD-IMAGE`); err == nil ||
		!strings.Contains(err.Error(), "no file system") {
		t.Errorf("LOAD-IMAGE without a file system: %v", err)
	}
}
