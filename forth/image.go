package forth

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"io"
	"strings"
)

// An image is a snapshot of everything a program added to a VM: the data
// space and the words defined on top of the standard dictionary. Primitives
// are Go code and are never written out; an image refers to them by their
// position in the standard dictionary instead, and refuses to load into a VM
// whose standard dictionary differs.
const (
	imageMagic   = "FORTHIMG"
	imageVersion = uint16(1)
)

// flagImmediate is the only flag stored with a saved word.
const flagImmediate uint8 = 1

// baselineHash fingerprints the standard dictionary, so that an image cannot
// be loaded into a VM whose primitives have moved.
func (vm *VM) baselineHash() uint64 {
	h := fnv.New64a()
	for _, w := range vm.order[:vm.baseline] {
		io.WriteString(h, strings.ToUpper(w.Name))
		io.WriteString(h, "\n")
	}
	return h.Sum64()
}

// SaveImage writes the data space and every word defined after New to w. The
// stacks, the input source and the pictured output buffer are not part of an
// image.
func (vm *VM) SaveImage(w io.Writer) error {
	if vm.Compiling() || vm.current != nil {
		return &Error{Msg: "cannot save an image while compiling"}
	}

	// Number every word that compiled code can refer to.
	index := make(map[*Word]int, len(vm.order))
	for i, word := range vm.order {
		index[word] = i
	}
	all := vm.order[vm.baseline:]

	// Every word an image contains must be describable: primitives added by
	// the embedder are Go code, and every call must land on a word that the
	// image or the standard dictionary has.
	for _, word := range all {
		if word.kind == kindPrim {
			return &Error{Msg: fmt.Sprintf("cannot save %s: primitives added after New are Go code", word.Name)}
		}
		for _, code := range [][]instr{word.code, word.does} {
			for _, in := range code {
				if in.word == nil {
					continue
				}
				if _, ok := index[in.word]; !ok {
					return &Error{Msg: fmt.Sprintf("cannot save %s: it calls %s, which is no longer in the dictionary",
						word.Name, in.word.Name)}
				}
			}
		}
	}

	// ref encodes a word reference: 0 none, 1 standard dictionary, 2 image.
	ref := func(e *encoder, target *Word) {
		switch {
		case target == nil:
			e.u8(0)
			e.u32(0)
		case index[target] < vm.baseline:
			e.u8(1)
			e.u32(uint32(index[target]))
		default:
			e.u8(2)
			e.u32(uint32(index[target] - vm.baseline))
		}
	}

	e := &encoder{}
	e.raw([]byte(imageMagic))
	e.u16(imageVersion)
	e.u8(CellSize)
	e.u32(uint32(vm.baseline))
	e.u64(vm.baselineHash())
	e.u64(uint64(vm.here))
	e.raw(vm.mem[:vm.here])
	e.u32(uint32(len(vm.xts)))
	e.u32(uint32(len(all)))
	for _, word := range all {
		flags := uint8(0)
		if word.Immediate {
			flags |= flagImmediate
		}
		e.u32(uint32(word.xt))
		e.u8(uint8(word.kind))
		e.u8(flags)
		e.str(word.Name)
		e.u64(uint64(word.data))
		e.u16(uint16(len(word.vals)))
		for _, v := range word.vals {
			e.u64(uint64(v))
		}
		for _, code := range [][]instr{word.code, word.does} {
			e.u32(uint32(len(code)))
			for _, in := range code {
				e.u8(uint8(in.op))
				e.u32(uint32(in.dest))
				e.u64(uint64(in.val))
				e.u64(uint64(in.len))
				ref(e, in.word)
			}
		}
	}
	if _, err := w.Write(e.bytes()); err != nil {
		return err
	}
	return nil
}

// LoadImage reads an image written by SaveImage. The VM must still be as New
// left it, with no words of its own, and its data space is replaced by the
// one in the image.
func (vm *VM) LoadImage(r io.Reader) error {
	if len(vm.order) != vm.baseline {
		return &Error{Msg: "cannot load an image into a dictionary that already has definitions"}
	}
	if vm.Compiling() || vm.current != nil {
		return &Error{Msg: "cannot load an image while compiling"}
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	d := &decoder{buf: data}

	if got := string(d.raw(len(imageMagic))); got != imageMagic {
		return &Error{Msg: "not a Forth image"}
	}
	version, cellSize := d.u16(), d.u8()
	baseCount, baseHash := int(d.u32()), d.u64()
	here := int64(d.u64())
	if d.err != nil { // the header itself is incomplete
		return d.err
	}
	if version != imageVersion {
		return &Error{Msg: fmt.Sprintf("image version %d, want %d", version, imageVersion)}
	}
	if cellSize != CellSize {
		return &Error{Msg: fmt.Sprintf("image has %d byte cells, want %d", cellSize, CellSize)}
	}
	if baseCount != vm.baseline {
		return &Error{Msg: fmt.Sprintf("image expects %d standard words, this interpreter has %d", baseCount, vm.baseline)}
	}
	if baseHash != vm.baselineHash() {
		return &Error{Msg: "image was written by a different version of the interpreter"}
	}
	if here < CellSize || here > int64(len(vm.mem)) {
		return &Error{Msg: fmt.Sprintf("image data space of %d bytes does not fit", here)}
	}
	mem := d.raw(int(here))
	xtCount := int(d.u32())
	count := int(d.u32())
	if d.err != nil {
		return d.err
	}

	// Decode every word before resolving references, because code may call a
	// word that is defined later in the image.
	type pending struct {
		word       *Word
		code, does []wordRef
	}
	words := make([]*Word, count)
	refs := make([]pending, count)
	for i := range count {
		xt := Cell(d.u32())
		kind := wordKind(d.u8())
		flags := d.u8()
		name := d.str()
		w := &Word{
			Name:      name,
			Immediate: flags&flagImmediate != 0,
			kind:      kind,
			data:      Cell(d.u64()),
			xt:        xt,
		}
		if kind > kindValue {
			return &Error{Msg: fmt.Sprintf("image word %s has an unknown kind %d", name, kind)}
		}
		if n := int(d.u16()); n > 0 {
			w.vals = make([]Cell, n)
			for j := range w.vals {
				w.vals[j] = Cell(d.u64())
			}
		}
		w.code, refs[i].code = d.code()
		w.does, refs[i].does = d.code()
		if len(w.does) == 0 {
			w.does = nil
		}
		refs[i].word = w
		words[i] = w
		if d.err != nil {
			return d.err
		}
		vm.defineAt(w, xt)
	}
	resolve := func(target wordRef) (*Word, error) {
		switch target.tag {
		case 0:
			return nil, nil
		case 1:
			if target.index >= vm.baseline {
				return nil, &Error{Msg: "image refers to a standard word that does not exist"}
			}
			return vm.order[target.index], nil
		case 2:
			if target.index >= len(words) {
				return nil, &Error{Msg: "image refers to a word that it does not contain"}
			}
			return words[target.index], nil
		}
		return nil, &Error{Msg: "image contains a damaged word reference"}
	}
	for _, p := range refs {
		for _, part := range []struct {
			code []instr
			refs []wordRef
		}{{p.word.code, p.code}, {p.word.does, p.does}} {
			for i := range part.code {
				target, err := resolve(part.refs[i])
				if err != nil {
					return err
				}
				part.code[i].word = target
			}
		}
	}
	if d.err != nil {
		return d.err
	}

	// Everything decoded, so commit.
	clear(vm.mem)
	copy(vm.mem, mem)
	vm.here = Cell(here)
	for len(vm.xts) < xtCount {
		vm.xts = append(vm.xts, nil)
	}
	vm.holdPtr = vm.holdBuf + holdSize
	vm.strOff = 0
	return nil
}

// defineAt enters a word into the dictionary keeping the execution token it
// had when the image was written, so that tokens compiled by ' and ['] stay
// valid.
func (vm *VM) defineAt(w *Word, xt Cell) {
	for Cell(len(vm.xts)) < xt {
		vm.xts = append(vm.xts, nil)
	}
	if xt >= 1 {
		vm.xts[xt-1] = w
	}
	w.xt = xt
	vm.words[strings.ToUpper(w.Name)] = w
	vm.order = append(vm.order, w)
}

// wordRef is an undecoded reference to a word.
type wordRef struct {
	tag   uint8
	index int
}

// --- encoding --------------------------------------------------------------

type encoder struct{ b []byte }

func (e *encoder) bytes() []byte { return e.b }
func (e *encoder) raw(p []byte)  { e.b = append(e.b, p...) }
func (e *encoder) u8(v uint8)    { e.b = append(e.b, v) }
func (e *encoder) u16(v uint16)  { e.b = binary.LittleEndian.AppendUint16(e.b, v) }
func (e *encoder) u32(v uint32)  { e.b = binary.LittleEndian.AppendUint32(e.b, v) }
func (e *encoder) u64(v uint64)  { e.b = binary.LittleEndian.AppendUint64(e.b, v) }
func (e *encoder) str(s string) {
	e.u16(uint16(len(s)))
	e.raw([]byte(s))
}

type decoder struct {
	buf []byte
	pos int
	err error
}

// short records that the image ended too early and returns zero values from
// then on.
func (d *decoder) short() bool {
	if d.err == nil {
		d.err = &Error{Msg: "truncated Forth image"}
	}
	return true
}

func (d *decoder) raw(n int) []byte {
	if n < 0 || d.pos+n > len(d.buf) {
		d.short()
		return make([]byte, max(n, 0))
	}
	p := d.buf[d.pos : d.pos+n]
	d.pos += n
	return p
}

func (d *decoder) u8() uint8 {
	p := d.raw(1)
	return p[0]
}

func (d *decoder) u16() uint16 { return binary.LittleEndian.Uint16(d.raw(2)) }
func (d *decoder) u32() uint32 { return binary.LittleEndian.Uint32(d.raw(4)) }
func (d *decoder) u64() uint64 { return binary.LittleEndian.Uint64(d.raw(8)) }
func (d *decoder) str() string { return string(d.raw(int(d.u16()))) }

// code reads a code list, leaving the word references undecoded.
func (d *decoder) code() ([]instr, []wordRef) {
	n := int(d.u32())
	if d.err != nil || n == 0 {
		return nil, nil
	}
	if n > len(d.buf)-d.pos {
		d.short()
		return nil, nil
	}
	code := make([]instr, n)
	refs := make([]wordRef, n)
	for i := range code {
		code[i] = instr{op: opcode(d.u8()), dest: int(d.u32())}
		code[i].val = Cell(d.u64())
		code[i].len = Cell(d.u64())
		refs[i] = wordRef{tag: d.u8(), index: int(d.u32())}
	}
	return code, refs
}
