package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type Marshalable interface {
	MarshalBinary() []byte
	UnmarshalBinary(data []byte) error
}

type binWriter struct {
	bytes.Buffer
}

func (w *binWriter) putU16(v uint16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	w.Write(b[:])
}

func (w *binWriter) putU32(v uint32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	w.Write(b[:])
}

func (w *binWriter) putU64(v uint64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	w.Write(b[:])
}

func (w *binWriter) putStr(s string) {
	w.WriteString(s)
}

func (w *binWriter) putBytes(b []byte) {
	w.Write(b)
}

type binReader struct {
	data []byte
	pos  int
	err  error
}

func (r *binReader) u16() uint16 {
	if r.err != nil {
		return 0
	}
	if r.pos+2 > len(r.data) {
		r.err = fmt.Errorf("overflow at %d reading uint16", r.pos)
		return 0
	}
	v := binary.BigEndian.Uint16(r.data[r.pos:])
	r.pos += 2
	return v
}

func (r *binReader) u32() uint32 {
	if r.err != nil {
		return 0
	}
	if r.pos+4 > len(r.data) {
		r.err = fmt.Errorf("overflow at %d reading uint32", r.pos)
		return 0
	}
	v := binary.BigEndian.Uint32(r.data[r.pos:])
	r.pos += 4
	return v
}

func (r *binReader) u64() uint64 {
	if r.err != nil {
		return 0
	}
	if r.pos+8 > len(r.data) {
		r.err = fmt.Errorf("overflow at %d reading uint64", r.pos)
		return 0
	}
	v := binary.BigEndian.Uint64(r.data[r.pos:])
	r.pos += 8
	return v
}

func (r *binReader) str(n int) string {
	if r.err != nil {
		return ""
	}
	if n < 0 || r.pos+n > len(r.data) {
		r.err = fmt.Errorf("overflow at %d reading %d bytes", r.pos, n)
		return ""
	}
	s := string(r.data[r.pos : r.pos+n])
	r.pos += n
	return s
}

func (r *binReader) readBytes(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || r.pos+n > len(r.data) {
		r.err = fmt.Errorf("overflow at %d reading %d bytes", r.pos, n)
		return nil
	}
	b := make([]byte, n)
	copy(b, r.data[r.pos:r.pos+n])
	r.pos += n
	return b
}
