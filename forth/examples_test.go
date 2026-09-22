package forth_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richardwooding/forth/forth"
)

// TestExamples runs every program in examples/ and checks what it printed, so
// that the examples in the README cannot rot.
func TestExamples(t *testing.T) {
	want := map[string]string{
		"hello.fs":    "Hello, world!\n",
		"fizzbuzz.fs": "1 2 Fizz 4 Buzz Fizz 7 8 Fizz Buzz 11 Fizz 13 14 FizzBuzz 16 17 Fizz 19 Buzz \n",
		"sieve.fs": "primes below 100: 2 3 5 7 11 13 17 19 23 29 31 37 41 43 47 53 59 61 67 71 " +
			"73 79 83 89 97 \nprimes below 1000: 168 \n",
		"fib.fs": "the first 16 Fibonacci numbers:\n" +
			"0 1 1 2 3 5 8 13 21 34 55 89 144 233 377 610 \n" +
			"fib 20, recursively: 6765 \nfib 90, iteratively: 2880067194370816120 \n",
		"case.fs": "Monday\nTuesday\nWednesday\nThursday\nFriday\nSaturday\nSunday\nday 9?\n",
		"defining-words.fs": "squares: 0 1 4 9 16 25 36 49 64 81 \n" +
			"two hours is 7200 seconds\ncounter holds 7 \n",
		"pictured.fs": "   cents    dollars\n    1234    $12.34\n       7    $0.07\n" +
			"   -2500    -$25.00\n12,34 EUR\n-42\n",
		"doubles.fs": "2^64  = 18446744073709551616 \n2^100 = 1267650600228229401496703205376 \n" +
			"25!   = 15511210043330985984000000 \n" +
			"3000000000 * 3000000000 = 9000000000000000000 \n" +
			"-5 as a double: -5 \nand doubled:    -10 \ncompared:       -1 \n",
		"file-io.fs": "notes.txt says:\nthe quick brown fox\njumps over the lazy dog\n" +
			"and it is gone again\n",
	}

	files, err := filepath.Glob(filepath.Join("..", "examples", "*.fs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(want) {
		t.Errorf("found %d examples but expect output for %d", len(files), len(want))
	}
	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			expected, ok := want[name]
			if !ok {
				t.Fatalf("no expected output recorded for %s", name)
			}
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			// The examples that touch files do so in the current directory.
			t.Chdir(t.TempDir())
			var out bytes.Buffer
			vm := forth.New(strings.NewReader(""), &out)
			vm.SetFileSystem(forth.OSFileSystem{})
			if err := vm.EvalNamed(string(src), name); err != nil {
				t.Fatalf("%s: %v\noutput so far:\n%s", name, err, out.String())
			}
			if got := out.String(); got != expected {
				t.Errorf("%s printed\n%q\nwant\n%q", name, got, expected)
			}
			if left := vm.Stack(); len(left) != 0 {
				t.Errorf("%s left %v on the stack", name, left)
			}
		})
	}
}
