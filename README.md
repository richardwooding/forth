# forth

A Forth interpreter written in Go: a library (`forth/`) and a command line
interpreter with a REPL (`cmd/forth/`). The dictionary follows the ANS Forth /
Forth-2012 core word set closely, with the double-cell, pictured-output and
file-access words on top. It needs Go 1.27 and has no dependencies outside the
standard library.

## Running it

```sh
go build -o bin/forth ./cmd/forth

bin/forth                      # REPL
bin/forth examples/sieve.fs    # run a program
bin/forth -e '6 7 * . cr'      # evaluate text (repeatable)
bin/forth -i examples/fib.fs   # run a program, then stay in the REPL

bin/forth -e ': greet ." hi" cr ;' -save app.img   # save an image
bin/forth -load app.img -e 'greet'                 # start from that image
```

A session looks like this:

```
$ bin/forth
forth - type BYE to leave, WORDS to list the dictionary
> 6 7 * .
42   ok
> : squared dup * ;
  ok
> 12 squared .
144   ok
> : countdown begin dup . 1- dup 0< until drop ;
  ok
> 5 countdown cr
5 4 3 2 1 0
  ok
> 1 0 /
  division by zero
> bye
```

Definitions may span several lines; the prompt changes to `...` while a
definition is open. An error prints a message and resets the interpreter, the
way `ABORT` does.

## Examples

Each file in `examples/` is a small tour of one topic, and the test suite
checks that they still print what the README says they do.

| File | What it shows |
| --- | --- |
| `hello.fs` | a definition, `."` and `CR` |
| `fizzbuzz.fs` | `?DO ... LOOP`, `IF`, `EXIT` |
| `sieve.fs` | `CREATE`/`ALLOT` data space, `BEGIN ... WHILE ... REPEAT` |
| `fib.fs` | `RECURSE` against an iterative loop |
| `case.fs` | `CASE ... OF ... ENDOF ... ENDCASE` |
| `defining-words.fs` | `CREATE ... DOES>`, arrays and scaled constants |
| `pictured.fs` | `<# # #S HOLD HOLDS SIGN #>` number formatting |
| `doubles.fs` | 128 bit double cell arithmetic, `UM*`, `M*`, `D2*` |
| `file-io.fs` | `CREATE-FILE`, `WRITE-LINE`, `READ-LINE`, `DELETE-FILE` |

## Using it as a library

```go
vm := forth.New(os.Stdin, os.Stdout)     // in and out may be nil
if err := vm.Eval(": tax 20 100 */ ;"); err != nil {
    log.Fatal(err)
}
if err := vm.Eval("250 tax"); err != nil {
    log.Fatal(err)
}
fmt.Println(vm.Stack()) // [50]
```

Useful methods on `*VM`:

- `Eval(src)` / `EvalNamed(src, name)` — interpret text; the name appears in
  error messages as `name:line:`. An error is a `*forth.Error` and resets the
  VM; `BYE` returns `forth.ErrBye`.
- `Stack()`, `Compiling()`, `Abort()`, `Lookup(name)`, `Execute(word)`.
- `SetFileSystem(fs)` — enable the file access words. Use
  `forth.OSFileSystem{}` for real files, or your own `forth.FileSystem` to
  sandbox or fake them. Without one, the file words return an error result and
  touch nothing.
- `SetSourceLoader(fn)` — supply the text for `INCLUDED` and `INCLUDE`
  yourself; otherwise they read through the file system.
- `SaveImage(w)` / `LoadImage(r)` — write the compiled dictionary and data
  space to a stream and read it back into a fresh VM.

A `*VM` is single threaded: give each goroutine its own.

## The dictionary

Word names are case insensitive. `WORDS` lists all 223 of them.

- **Stack** `DUP ?DUP DROP SWAP OVER NIP TUCK ROT -ROT 2DUP 2DROP 2SWAP 2OVER
  DEPTH PICK ROLL`
- **Return stack** `>R R> R@ RDROP 2>R 2R>`
- **Arithmetic** `+ - * / MOD /MOD */ */MOD 1+ 1- 2+ 2- 2* 2/ ABS NEGATE MIN
  MAX FM/MOD SM/REM UM/MOD UM* M* M+`
- **Bitwise** `AND OR XOR INVERT LSHIFT RSHIFT`
- **Comparison** `= <> < > <= >= U< U> 0= 0<> 0< 0> WITHIN TRUE FALSE`
- **Memory** `@ ! +! C@ C! 2@ 2! , C, ALLOT HERE UNUSED ALIGN ALIGNED CELL
  CELL+ CELLS CHAR+ CHARS FILL ERASE BLANK MOVE CMOVE COUNT /STRING BOUNDS PAD
  BASE STATE`
- **Defining** `: ; IMMEDIATE VARIABLE CONSTANT CREATE DOES> VALUE TO >BODY
  2CONSTANT 2VARIABLE FORGET`
- **Compiling** `' ['] EXECUTE POSTPONE LITERAL 2LITERAL RECURSE [ ]`
- **Control flow** `IF ELSE THEN BEGIN UNTIL AGAIN WHILE REPEAT DO ?DO LOOP
  +LOOP LEAVE UNLOOP I J EXIT CASE OF ENDOF ENDCASE`
- **Output** `. U. .R U.R ? .S EMIT CR SPACE SPACES TYPE ." S" C" D. D.R UD.`
- **Pictured output** `<# # #S HOLD HOLDS SIGN #>`
- **Numeric base** `DECIMAL HEX OCTAL BINARY`
- **Double cell** `S>D D>S D+ D- DNEGATE DABS D2* D2/ D0= D0< D= D< DU< DMAX
  DMIN 2ROT`
- **Input and source** `KEY ACCEPT EVALUATE INCLUDE INCLUDED INCLUDE-FILE CHAR
  [CHAR] BL ( \`
- **Files** `OPEN-FILE CREATE-FILE CLOSE-FILE DELETE-FILE RENAME-FILE READ-FILE
  READ-LINE WRITE-FILE WRITE-LINE FILE-SIZE FILE-POSITION REPOSITION-FILE
  FLUSH-FILE R/O W/O R/W BIN FILE-ERROR SAVE-IMAGE LOAD-IMAGE`
- **Other** `WORDS ABORT ABORT" (ABORT") BYE MS MS@ TIME&DATE`

Numbers are read in the current `BASE`, with the usual prefixes: `$ff` is
hexadecimal, `#99` decimal, `%1010` binary and `'A'` is a character.

## Images

An image is a snapshot of everything a program added to the interpreter: the
data space and every word defined on top of the standard dictionary. Compiling
a large program once and starting from the image afterwards is much faster
than re-reading the source, and it is a way to ship a Forth application as one
file.

```sh
bin/forth -save sieve.img examples/sieve.fs   # compile once
bin/forth -load sieve.img -e 'count-primes .' # start from the image
bin/forth -load sieve.img                     # or explore it in the REPL
```

From Forth, `S" app.img" SAVE-IMAGE` and `S" app.img" LOAD-IMAGE` do the same
through the file system; in Go they are `vm.SaveImage(w)` and
`vm.LoadImage(r)`.

What travels: colon definitions with their control flow and compiled strings,
`CONSTANT`, `VARIABLE`, `VALUE`, `CREATE ... DOES>`, `2CONSTANT`, `2VARIABLE`,
immediacy, execution tokens (so a compiled `[']` still points at the same
word), the whole data space including `BASE`, and words that are shadowed by a
later redefinition but still called by older definitions.

What does not: the stacks, the input source, open files, and anything a Go
embedder added as a primitive — an image refers to primitives by their place
in the standard dictionary, and records a fingerprint of it. Loading an image
into an interpreter whose standard dictionary differs is refused rather than
silently misinterpreted, as is loading into a session that already has
definitions of its own.

## How it works

`:` compiles a word into a slice of instructions rather than into data space.
Branches, loops, literals, strings and `EXIT` are instructions of the inner
interpreter (`forth/inner.go`); everything else is a call to another word.
Primitives are Go closures. Data space (`HERE`, `,`, `ALLOT`, `CREATE`) is a
separate 1 MiB byte array with 8 byte cells, so `C@`, strings and `CELLS`
behave as a Forth programmer expects. Forth level errors are raised as a panic
carrying a `*forth.Error` and are caught at the `Eval` boundary.

Deliberate departures from the standard:

- Cells are 64 bit, doubles 128 bit. Addresses are byte offsets into data
  space, and address 0 is never valid, so a stray `0 @` is an error rather than
  a crash.
- `/`, `MOD` and `/MOD` are floored. `FM/MOD` is floored and `SM/REM`
  symmetric, both taking a double dividend, as the standard says.
- The dictionary lives in Go, not in data space, so there is no way to walk it
  with `@`. Execution tokens are small integers; `>BODY` works for words made
  by `CREATE`, `VARIABLE` and `VALUE`.
- `DO` loop frames live on their own stack rather than the return stack, so
  `>R` and `R@` inside a loop are safe. `EXIT` discards the loop frames of the
  word it leaves, and `UNLOOP` is still accepted.
- `'` is state smart: inside a definition it compiles the token, like `[']`.
- `S"` while interpreting returns a transient buffer that is reused after four
  more strings.
- No vocabularies or word lists, no `THROW`/`CATCH`, no floating point, no
  `M*/`, and the input source is not exposed through `>IN`/`SOURCE`.
- Non-standard conveniences: `<= >= 0<> U> RDROP NIP TUCK -ROT 2+ 2- CELL
  BOUNDS BLANK UD. MS@ FILE-ERROR`.

## Tests

```sh
go vet ./...
go test ./...
```

The suite covers the word sets word by word, the error paths, and every
program in `examples/`.
