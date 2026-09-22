// Command forth is a Forth interpreter: it runs the script files named on the
// command line and then, if asked or if there is nothing else to do, offers a
// read-eval-print loop.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/richardwooding/forth/forth"
)

func main() {
	out := bufio.NewWriter(os.Stdout)
	err := run(out)
	out.Flush()
	if err != nil && !errors.Is(err, forth.ErrBye) {
		fmt.Fprintln(os.Stderr, "forth:", err)
		os.Exit(1)
	}
}

// evalFlag collects repeated -e options.
type evalFlag []string

func (e *evalFlag) String() string     { return strings.Join(*e, " ") }
func (e *evalFlag) Set(s string) error { *e = append(*e, s); return nil }

func run(out *bufio.Writer) error {
	var evals evalFlag
	flag.Var(&evals, "e", "evaluate `text` before any files (repeatable)")
	interactive := flag.Bool("i", false, "enter the REPL after running files")
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), "usage: forth [-e text] [-i] [file.fs ...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	vm := forth.New(os.Stdin, out)
	vm.SetSourceLoader(loadFile)
	vm.SetFileSystem(forth.OSFileSystem{})

	for _, text := range evals {
		if err := vm.EvalNamed(text, "-e"); err != nil {
			return err
		}
	}
	for _, name := range flag.Args() {
		src, err := loadFile(name)
		if err != nil {
			return err
		}
		if err := vm.EvalNamed(src, name); err != nil {
			return err
		}
	}
	if *interactive || (len(evals) == 0 && flag.NArg() == 0) {
		return repl(vm, out)
	}
	return nil
}

func loadFile(name string) (string, error) {
	b, err := os.ReadFile(filepath.Clean(name))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func repl(vm *forth.VM, out *bufio.Writer) error {
	fmt.Fprintln(out, "forth - type BYE to leave, WORDS to list the dictionary")
	// Read through vm.In, so that KEY and ACCEPT see the same buffered input.
	for {
		if vm.Compiling() {
			fmt.Fprint(out, "... ")
		} else {
			fmt.Fprint(out, "> ")
		}
		out.Flush()
		line, readErr := vm.In.ReadString('\n')
		if line == "" && readErr != nil {
			fmt.Fprintln(out)
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
		err := vm.Eval(strings.TrimRight(line, "\r\n"))
		switch {
		case errors.Is(err, forth.ErrBye):
			fmt.Fprintln(out)
			return nil
		case err != nil:
			fmt.Fprintf(out, "  %v\n", err)
		case !vm.Compiling():
			fmt.Fprintln(out, "  ok")
		}
	}
}
