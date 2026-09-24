package forth

import (
	"errors"
	"io"
	"os"
	"time"
)

// FileMode says how a file should be opened. It is the Go level counterpart of
// a Forth file access method.
type FileMode uint

// The bits of a FileMode.
const (
	ModeRead FileMode = 1 << iota
	ModeWrite
	ModeCreate
	ModeTruncate
	ModeBinary
)

// File is an open file as the file access words use it.
type File interface {
	io.Reader
	io.Writer
	io.Seeker
	io.Closer
	Sync() error
	Size() (int64, error)
}

// FileSystem is the host file system behind the file access words. A VM
// without one answers every file word with an error ior, so that programs
// cannot touch the host by accident.
type FileSystem interface {
	Open(name string, mode FileMode) (File, error)
	Delete(name string) error
	Rename(from, to string) error
}

// OSFileSystem is the FileSystem of the machine the interpreter runs on.
type OSFileSystem struct{}

// Open opens name, creating and truncating it if the mode asks for that.
func (OSFileSystem) Open(name string, mode FileMode) (File, error) {
	flag := os.O_RDONLY
	switch {
	case mode&ModeRead != 0 && mode&ModeWrite != 0:
		flag = os.O_RDWR
	case mode&ModeWrite != 0:
		flag = os.O_WRONLY
	}
	if mode&ModeCreate != 0 {
		flag |= os.O_CREATE
	}
	if mode&ModeTruncate != 0 {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(name, flag, 0o666)
	if err != nil {
		return nil, err
	}
	return osFile{f}, nil
}

// Delete removes name.
func (OSFileSystem) Delete(name string) error { return os.Remove(name) }

// Rename renames from to to.
func (OSFileSystem) Rename(from, to string) error { return os.Rename(from, to) }

type osFile struct{ *os.File }

func (f osFile) Size() (int64, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// SetFileSystem installs the file system used by the file access words and, if
// no source loader is set, by INCLUDED.
func (vm *VM) SetFileSystem(fs FileSystem) { vm.fs = fs }

// errNoFileSystem is reported when file words are used on a VM that has no
// file system.
var errNoFileSystem = errors.New("no file system installed")

// loadSource reads a source file for INCLUDED and INCLUDE.
func (vm *VM) loadSource(name string) (string, error) {
	if vm.source != nil {
		return vm.source(name)
	}
	if vm.fs == nil {
		return "", errNoFileSystem
	}
	f, err := vm.fs.Open(name, ModeRead)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ior returns the Forth I/O result for err: zero on success, -1 otherwise.
// The message is kept for FILE-ERROR.
func (vm *VM) ior(err error) Cell {
	if err == nil {
		return 0
	}
	vm.lastIOErr = err.Error()
	return -1
}

// file looks up an open file by its Forth file identifier.
func (vm *VM) file(fid Cell) File {
	f := vm.files[fid]
	if f == nil {
		vm.throw("invalid file identifier %d", fid)
	}
	return f
}

// famMode converts a Forth file access method to a FileMode.
func (vm *VM) famMode(fam Cell) FileMode {
	mode := FileMode(0)
	switch fam & 3 {
	case 1:
		mode = ModeRead
	case 2:
		mode = ModeWrite
	case 3:
		mode = ModeRead | ModeWrite
	default:
		vm.throw("invalid file access method %d", fam)
	}
	if fam&4 != 0 {
		mode |= ModeBinary
	}
	return mode
}

// popName pops a string from the stack, for the words that take a file name.
func (vm *VM) popName() string {
	n := vm.pop()
	addr := vm.pop()
	return vm.readString(addr, n)
}

func (vm *VM) installSystem() {
	vm.prim("R/O", func(vm *VM) { vm.push(1) })
	vm.prim("W/O", func(vm *VM) { vm.push(2) })
	vm.prim("R/W", func(vm *VM) { vm.push(3) })
	vm.prim("BIN", func(vm *VM) { vm.push(vm.pop() | 4) })

	open := func(vm *VM, extra FileMode) {
		fam := vm.pop()
		name := vm.popName()
		mode := vm.famMode(fam) | extra
		if vm.fs == nil {
			vm.push(0)
			vm.push(vm.ior(errNoFileSystem))
			return
		}
		f, err := vm.fs.Open(name, mode)
		if err != nil {
			vm.push(0)
			vm.push(vm.ior(err))
			return
		}
		vm.nextFid++
		if vm.files == nil {
			vm.files = make(map[Cell]File)
		}
		vm.files[vm.nextFid] = f
		vm.push(vm.nextFid)
		vm.push(0)
	}
	vm.prim("OPEN-FILE", func(vm *VM) { open(vm, 0) })
	vm.prim("CREATE-FILE", func(vm *VM) { open(vm, ModeCreate|ModeTruncate) })
	vm.prim("CLOSE-FILE", func(vm *VM) {
		fid := vm.pop()
		f := vm.file(fid)
		delete(vm.files, fid)
		vm.push(vm.ior(f.Close()))
	})
	vm.prim("DELETE-FILE", func(vm *VM) {
		name := vm.popName()
		if vm.fs == nil {
			vm.push(vm.ior(errNoFileSystem))
			return
		}
		vm.push(vm.ior(vm.fs.Delete(name)))
	})
	vm.prim("RENAME-FILE", func(vm *VM) {
		to := vm.popName()
		from := vm.popName()
		if vm.fs == nil {
			vm.push(vm.ior(errNoFileSystem))
			return
		}
		vm.push(vm.ior(vm.fs.Rename(from, to)))
	})
	vm.prim("READ-FILE", func(vm *VM) {
		f := vm.file(vm.pop())
		n := vm.pop()
		addr := vm.pop()
		vm.checkAddr(addr, n)
		got, err := io.ReadFull(f, vm.mem[addr:addr+n])
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			err = nil
		}
		vm.push(Cell(got))
		vm.push(vm.ior(err))
	})
	vm.prim("READ-LINE", func(vm *VM) {
		f := vm.file(vm.pop())
		limit := vm.pop()
		addr := vm.pop()
		vm.checkAddr(addr, limit)
		// The file position is shared with the other file words, so read one
		// byte at a time rather than buffering ahead.
		var n Cell
		var buf [1]byte
		var err error
		eof := true
		for n < limit {
			_, err = f.Read(buf[:])
			if err != nil {
				break
			}
			eof = false
			if buf[0] == '\n' {
				break
			}
			if buf[0] != '\r' {
				vm.mem[addr+n] = buf[0]
				n++
			}
		}
		if errors.Is(err, io.EOF) {
			err = nil
		}
		vm.push(n)
		vm.pushBool(!eof)
		vm.push(vm.ior(err))
	})
	vm.prim("WRITE-FILE", func(vm *VM) {
		f := vm.file(vm.pop())
		n := vm.pop()
		addr := vm.pop()
		vm.checkAddr(addr, n)
		_, err := f.Write(vm.mem[addr : addr+n])
		vm.push(vm.ior(err))
	})
	vm.prim("WRITE-LINE", func(vm *VM) {
		f := vm.file(vm.pop())
		n := vm.pop()
		addr := vm.pop()
		vm.checkAddr(addr, n)
		_, err := f.Write(vm.mem[addr : addr+n])
		if err == nil {
			_, err = io.WriteString(f, "\n")
		}
		vm.push(vm.ior(err))
	})
	vm.prim("FILE-SIZE", func(vm *VM) {
		f := vm.file(vm.pop())
		size, err := f.Size()
		vm.pushD(uint64(size), 0)
		vm.push(vm.ior(err))
	})
	vm.prim("FILE-POSITION", func(vm *VM) {
		f := vm.file(vm.pop())
		pos, err := f.Seek(0, io.SeekCurrent)
		vm.pushD(uint64(pos), 0)
		vm.push(vm.ior(err))
	})
	vm.prim("REPOSITION-FILE", func(vm *VM) {
		f := vm.file(vm.pop())
		lo, _ := vm.popD()
		_, err := f.Seek(int64(lo), io.SeekStart)
		vm.push(vm.ior(err))
	})
	vm.prim("FLUSH-FILE", func(vm *VM) {
		f := vm.file(vm.pop())
		vm.push(vm.ior(f.Sync()))
	})
	vm.prim("INCLUDE-FILE", func(vm *VM) {
		fid := vm.pop()
		f := vm.file(fid)
		b, err := io.ReadAll(f)
		if err != nil {
			vm.throw("INCLUDE-FILE: %v", err)
		}
		vm.pushInput(string(b), "file "+vm.format(fid))
		defer vm.popInput()
		vm.interpret()
	})
	vm.prim("SAVE-IMAGE", func(vm *VM) {
		name := vm.popName()
		if vm.fs == nil {
			vm.throw("SAVE-IMAGE: %v", errNoFileSystem)
		}
		f, err := vm.fs.Open(name, ModeWrite|ModeCreate|ModeTruncate)
		if err != nil {
			vm.throw("SAVE-IMAGE: %v", err)
		}
		defer f.Close()
		if err := vm.SaveImage(f); err != nil {
			vm.throw("SAVE-IMAGE: %v", err)
		}
	})
	vm.prim("LOAD-IMAGE", func(vm *VM) {
		name := vm.popName()
		if vm.fs == nil {
			vm.throw("LOAD-IMAGE: %v", errNoFileSystem)
		}
		f, err := vm.fs.Open(name, ModeRead)
		if err != nil {
			vm.throw("LOAD-IMAGE: %v", err)
		}
		defer f.Close()
		if err := vm.LoadImage(f); err != nil {
			vm.throw("LOAD-IMAGE: %v", err)
		}
	})
	vm.prim("FILE-ERROR", func(vm *VM) {
		addr, n := vm.transient(vm.lastIOErr)
		vm.push(addr)
		vm.push(n)
	})

	vm.prim("MS", func(vm *VM) {
		if ms := vm.pop(); ms > 0 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
	})
	vm.prim("TIME&DATE", func(vm *VM) {
		now := time.Now()
		vm.push(Cell(now.Second()))
		vm.push(Cell(now.Minute()))
		vm.push(Cell(now.Hour()))
		vm.push(Cell(now.Day()))
		vm.push(Cell(now.Month()))
		vm.push(Cell(now.Year()))
	})
	vm.prim("MS@", func(vm *VM) { vm.push(Cell(time.Now().UnixMilli())) })
}
