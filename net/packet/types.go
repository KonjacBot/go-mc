package packet

import (
	"encoding/binary"
	"errors"
	"io"
	"math"

	"github.com/google/uuid"

	"github.com/KonjacBot/go-mc/nbt"
)

// A Field is both FieldEncoder and FieldDecoder
type Field interface {
	FieldEncoder
	FieldDecoder
}

// A FieldEncoder can be encoded as minecraft protocol used.
type FieldEncoder io.WriterTo

// A FieldDecoder can Decode from minecraft protocol
type FieldDecoder io.ReaderFrom

type (
	// Boolean of True is encoded as 0x01, false as 0x00.
	Boolean bool
	// Byte is signed 8-bit integer, two's complement
	Byte int8
	// UnsignedByte is unsigned 8-bit integer
	UnsignedByte uint8
	// Short is signed 16-bit integer, two's complement
	Short int16
	// UnsignedShort is unsigned 16-bit integer
	UnsignedShort uint16
	// Int is signed 32-bit integer, two's complement
	Int int32
	// Long is signed 64-bit integer, two's complement
	Long int64
	// A Float is a single-precision 32-bit IEEE 754 floating point number
	Float float32
	// A Double is a double-precision 64-bit IEEE 754 floating point number
	Double float64
	// String is sequence of Unicode scalar values
	String string

	// Chat is encoded as a String with max length of 32767.
	// Deprecated: Use chat.Message
	Chat = String

	// Identifier is encoded as a String with max length of 32767.
	Identifier = String

	// VarInt is variable-length data encoding a two's complement signed 32-bit integer
	VarInt int32
	// VarLong is variable-length data encoding a two's complement signed 64-bit integer
	VarLong int64

	// Position x as a 26-bit integer, followed by y as a 12-bit integer, followed by z as a 26-bit integer (all signed, two's complement)
	Position struct {
		X, Y, Z int
	}

	// Angle is rotation angle in steps of 1/256 of a full turn
	Angle Byte

	// UUID encoded as an unsigned 128-bit integer
	UUID uuid.UUID

	// ByteArray is []byte with prefix VarInt as length
	ByteArray []byte

	// PluginMessageData is only used in LoginPlugin,and it will read all left bytes
	PluginMessageData []byte

	// BitSet represents Java's BitSet, a list of bits.
	BitSet []int64

	// FixedBitSet is a fixed size BitSet
	FixedBitSet []byte
)

const (
	MaxVarIntLen  = 5
	MaxVarLongLen = 10
)

func (b Boolean) WriteTo(w io.Writer) (int64, error) {
	buf, err := AppendBool(nil, bool(b))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

func (b *Boolean) ReadFrom(r io.Reader) (n int64, err error) {
	n, v, err := readByte(r)
	if err != nil {
		return n, err
	}

	var out bool
	if _, err := DecodeBool([]byte{v}, &out); err != nil {
		return n, err
	}
	*b = Boolean(out)
	return n, nil
}

func (s String) WriteTo(w io.Writer) (int64, error) {
	buf, err := AppendString(nil, string(s))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

func (s *String) ReadFrom(r io.Reader) (n int64, err error) {
	var l VarInt // String length

	nn, err := l.ReadFrom(r)
	if err != nil {
		return nn, err
	}
	n += nn

	bs := make([]byte, l)
	if _, err := io.ReadFull(r, bs); err != nil {
		return n, err
	}
	n += int64(l)

	*s = String(bs)
	return n, nil
}

// readByte read one byte from io.Reader
func readByte(r io.Reader) (int64, byte, error) {
	if r, ok := r.(io.ByteReader); ok {
		v, err := r.ReadByte()
		return 1, v, err
	}
	var v [1]byte
	n, err := r.Read(v[:])
	return int64(n), v[0], err
}

func (b Byte) WriteTo(w io.Writer) (n int64, err error) {
	buf, err := AppendInt8(nil, int8(b))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

func (b *Byte) ReadFrom(r io.Reader) (n int64, err error) {
	n, v, err := readByte(r)
	if err != nil {
		return n, err
	}
	*b = Byte(v)
	return n, nil
}

func (u UnsignedByte) WriteTo(w io.Writer) (n int64, err error) {
	buf, err := AppendUint8(nil, uint8(u))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

func (u *UnsignedByte) ReadFrom(r io.Reader) (n int64, err error) {
	n, v, err := readByte(r)
	if err != nil {
		return n, err
	}
	*u = UnsignedByte(v)
	return n, nil
}

func (s Short) WriteTo(w io.Writer) (int64, error) {
	buf, err := AppendInt16(nil, int16(s))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

func (s *Short) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [2]byte
	if nn, err := io.ReadFull(r, bs[:]); err != nil {
		return int64(nn), err
	} else {
		n += int64(nn)
	}

	*s = Short(binary.BigEndian.Uint16(bs[:]))
	return
}

func (us UnsignedShort) WriteTo(w io.Writer) (int64, error) {
	buf, err := AppendUint16(nil, uint16(us))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

func (us *UnsignedShort) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [2]byte
	if nn, err := io.ReadFull(r, bs[:]); err != nil {
		return int64(nn), err
	} else {
		n += int64(nn)
	}

	*us = UnsignedShort(binary.BigEndian.Uint16(bs[:]))
	return
}

func (i Int) WriteTo(w io.Writer) (int64, error) {
	buf, err := AppendInt32(nil, int32(i))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

func (i *Int) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [4]byte
	if nn, err := io.ReadFull(r, bs[:]); err != nil {
		return int64(nn), err
	} else {
		n += int64(nn)
	}

	*i = Int(binary.BigEndian.Uint32(bs[:]))
	return
}

func (l Long) WriteTo(w io.Writer) (int64, error) {
	buf, err := AppendInt64(nil, int64(l))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

func (l *Long) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [8]byte
	if nn, err := io.ReadFull(r, bs[:]); err != nil {
		return int64(nn), err
	} else {
		n += int64(nn)
	}

	*l = Long(binary.BigEndian.Uint64(bs[:]))
	return
}

func (v VarInt) WriteTo(w io.Writer) (n int64, err error) {
	buf, err := AppendVarInt(nil, int32(v))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

// WriteToBytes encodes the VarInt into buf and returns the number of bytes written.
// If the buffer is too small, WriteToBytes will panic.
func (v VarInt) WriteToBytes(buf []byte) int {
	encoded, _ := AppendVarInt(nil, int32(v))
	return copy(buf, encoded)
}

func (v *VarInt) ReadFrom(r io.Reader) (n int64, err error) {
	var V uint32
	var num int64
	byteReader := CreateByteReader(r)
	for sec := byte(0x80); sec&0x80 != 0; num++ {
		if num > MaxVarIntLen {
			return n, errors.New("VarInt is too big")
		}

		sec, err = byteReader.ReadByte()
		if err != nil {
			return n, err
		}
		n += 1

		V |= uint32(sec&0x7F) << uint32(7*num)
	}

	*v = VarInt(V)
	return
}

// Len returns the number of bytes required to encode the VarInt.
func (v VarInt) Len() int {
	n, _ := SizeVarInt(int32(v))
	return n
}

func (v VarLong) WriteTo(w io.Writer) (n int64, err error) {
	buf, err := AppendVarLong(nil, int64(v))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

// WriteToBytes encodes the VarLong into buf and returns the number of bytes written.
// If the buffer is too small, WriteToBytes will panic.
func (v VarLong) WriteToBytes(buf []byte) int {
	encoded, _ := AppendVarLong(nil, int64(v))
	return copy(buf, encoded)
}

func (v *VarLong) ReadFrom(r io.Reader) (n int64, err error) {
	var V uint64
	var num int64
	byteReader := CreateByteReader(r)
	for sec := byte(0x80); sec&0x80 != 0; num++ {
		if num >= MaxVarLongLen {
			return n, errors.New("VarLong is too big")
		}
		sec, err = byteReader.ReadByte()
		if err != nil {
			return
		}
		n += 1

		V |= uint64(sec&0x7F) << uint64(7*num)
	}

	*v = VarLong(V)
	return
}

// Len returns the number of bytes required to encode the VarLong.
func (v VarLong) Len() int {
	n, _ := SizeVarLong(int64(v))
	return n
}

func (p Position) WriteTo(w io.Writer) (n int64, err error) {
	var b [8]byte
	position := uint64(p.X&0x3FFFFFF)<<38 | uint64((p.Z&0x3FFFFFF)<<12) | uint64(p.Y&0xFFF)
	for i := 7; i >= 0; i-- {
		b[i] = byte(position)
		position >>= 8
	}
	nn, err := w.Write(b[:])
	return int64(nn), err
}

func (p *Position) ReadFrom(r io.Reader) (n int64, err error) {
	var v Long
	nn, err := v.ReadFrom(r)
	if err != nil {
		return nn, err
	}
	n += nn

	x := int(v >> 38)
	y := int(v << 52 >> 52)
	z := int(v << 26 >> 38)

	p.X, p.Y, p.Z = x, y, z
	return
}

// ToDeg convert Angle to Degree
func (a Angle) ToDeg() float64 {
	return 360 * float64(a) / 256
}

// ToRad convert Angle to Radian
func (a Angle) ToRad() float64 {
	return 2 * math.Pi * float64(a) / 256
}

func (a Angle) WriteTo(w io.Writer) (int64, error) {
	return Byte(a).WriteTo(w)
}

func (a *Angle) ReadFrom(r io.Reader) (int64, error) {
	return (*Byte)(a).ReadFrom(r)
}

func (f Float) WriteTo(w io.Writer) (n int64, err error) {
	buf, err := AppendFloat32(nil, float32(f))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

func (f *Float) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [4]byte
	nn, err := io.ReadFull(r, bs[:])
	if err != nil {
		return int64(nn), err
	}
	var out float32
	_, err = DecodeFloat32(bs[:], &out)
	*f = Float(out)
	return int64(nn), err
}

func (d Double) WriteTo(w io.Writer) (n int64, err error) {
	buf, err := AppendFloat64(nil, float64(d))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

func (d *Double) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [8]byte
	nn, err := io.ReadFull(r, bs[:])
	if err != nil {
		return int64(nn), err
	}
	var out float64
	_, err = DecodeFloat64(bs[:], &out)
	*d = Double(out)
	return int64(nn), err
}

// NBT is used to write or read Named Binary Tag data from/to packets.
//
// When using as [FieldDecoder], unknown fields are not allowed by default.
// For allow unknown fields, using [NBTField] instead.
func NBT(v any) Field {
	return NBTField{V: v}
}

// NBTField is a NBT warpper for convert any type into a Field, allowing to be pack or unpack with network packets.
//
// After v1.20.2, the "network format" is used. The root tag nolonger have a tag name anymore.
type NBTField struct {
	V any

	DisallowUnknownFields bool
}

func (n NBTField) WriteTo(w io.Writer) (int64, error) {
	if n.V == nil {
		n, err := w.Write([]byte{nbt.TagEnd})
		return int64(n), err
	}
	// nbt Encode method does not count written bytes,
	// so we warp the writer to count it.
	cw := countingWriter{w: w}
	enc := nbt.NewEncoder(&cw)
	enc.NetworkFormat(true)
	err := enc.Encode(n.V, "")
	return cw.n, err
}

func (n NBTField) ReadFrom(r io.Reader) (int64, error) {
	// LimitReader is used to count reader length
	cr := countingReader{r: r}
	dec := nbt.NewDecoder(&cr)
	dec.NetworkFormat(true)
	if n.DisallowUnknownFields {
		dec.DisallowUnknownFields()
	}
	_, err := dec.Decode(n.V)
	if err != nil {
		if !errors.Is(err, nbt.ErrEND) {
			return cr.n, err
		}
		err = nil
	}
	return cr.n, nil
}

// countingWriter is a wrapper of io.Writer to externally count written bytes
type countingWriter struct {
	n int64
	w io.Writer
}

func (c *countingWriter) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	c.n += int64(n)
	return
}

// countingReader is a wrapper of io.Reader to externally count read bytes
type countingReader struct {
	n int64
	r io.Reader
}

func (c *countingReader) Read(p []byte) (n int, err error) {
	n, err = c.r.Read(p)
	c.n += int64(n)
	return
}

func (b ByteArray) WriteTo(w io.Writer) (n int64, err error) {
	buf, err := AppendByteArray(nil, []byte(b))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

func (b *ByteArray) ReadFrom(r io.Reader) (n int64, err error) {
	var Len VarInt
	n1, err := Len.ReadFrom(r)
	if err != nil {
		return n1, err
	}
	if cap(*b) < int(Len) {
		*b = make(ByteArray, Len)
	} else {
		*b = (*b)[:Len]
	}
	n2, err := io.ReadFull(r, *b)
	return n1 + int64(n2), err
}

func (u UUID) WriteTo(w io.Writer) (n int64, err error) {
	buf, err := AppendUUID(nil, uuid.UUID(u))
	if err != nil {
		return 0, err
	}
	nn, err := w.Write(buf)
	return int64(nn), err
}

func (u *UUID) ReadFrom(r io.Reader) (n int64, err error) {
	var bs [16]byte
	nn, err := io.ReadFull(r, bs[:])
	if err != nil {
		return int64(nn), err
	}
	var out uuid.UUID
	_, err = DecodeUUID(bs[:], &out)
	*u = UUID(out)
	return int64(nn), err
}

func (p PluginMessageData) WriteTo(w io.Writer) (n int64, err error) {
	nn, err := w.Write(p)
	return int64(nn), err
}

func (p *PluginMessageData) ReadFrom(r io.Reader) (n int64, err error) {
	*p, err = io.ReadAll(r)
	return int64(len(*p)), err
}

func (b BitSet) WriteTo(w io.Writer) (n int64, err error) {
	n, err = VarInt(len(b)).WriteTo(w)
	if err != nil {
		return
	}
	for i := range b {
		n2, err := Long(b[i]).WriteTo(w)
		if err != nil {
			return n + n2, err
		}
		n += n2
	}
	return
}

func (b *BitSet) ReadFrom(r io.Reader) (n int64, err error) {
	var Len VarInt
	n, err = Len.ReadFrom(r)
	if err != nil {
		return
	}
	if int(Len) > cap(*b) {
		*b = make([]int64, Len)
	} else {
		*b = (*b)[:Len]
	}
	for i := 0; i < int(Len); i++ {
		n2, err := ((*Long)(&(*b)[i])).ReadFrom(r)
		if err != nil {
			return n + n2, err
		}
		n += n2
	}
	return
}

func (b BitSet) Get(index int) bool {
	return (b[index/64] & (1 << (index % 64))) != 0
}

func (b BitSet) Set(index int, value bool) {
	if value {
		b[index/64] |= 1 << (index % 64)
	} else {
		b[index/64] &= ^(1 << (index % 64))
	}
}

func (b BitSet) Len() int {
	return len(b) * 64
}

// NewFixedBitSet make a [FixedBitSet] which can store n bits at least.
// If n <= 0, return nil
func NewFixedBitSet(n int64) FixedBitSet {
	if n < 0 {
		return nil
	}
	return make(FixedBitSet, (n+7)/8)
}

func (f FixedBitSet) WriteTo(w io.Writer) (n int64, err error) {
	n2, err := w.Write(f)
	return int64(n2), err
}

func (f FixedBitSet) ReadFrom(r io.Reader) (n int64, err error) {
	n2, err := r.Read(f)
	return int64(n2), err
}

func (f FixedBitSet) Get(index int) bool {
	return (f[index/8] & (1 << (index % 8))) != 0
}

func (f FixedBitSet) Set(index int, value bool) {
	if value {
		f[index/8] |= 1 << (index % 8)
	} else {
		f[index/8] &= ^(1 << (index % 8))
	}
}

func (f FixedBitSet) Len() int {
	return len(f) * 8
}

func sizeVarIntMust(v int32) int {
	n, _ := SizeVarInt(v)
	return n
}
