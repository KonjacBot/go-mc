package packet

import (
	"bytes"
	"io"
	"runtime"
	"testing"
)

// varIntBytes encodes v as a VarInt.
func varIntBytes(v int32) []byte {
	var buf bytes.Buffer
	_, _ = VarInt(v).WriteTo(&buf)
	return buf.Bytes()
}

// allocatedBy reports the bytes f allocated on the heap.
func allocatedBy(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestLengthPrefixedFieldsRejectBadLengths(t *testing.T) {
	decoders := map[string]func() FieldDecoder{
		"String":    func() FieldDecoder { return new(String) },
		"ByteArray": func() FieldDecoder { return new(ByteArray) },
		"IDSet":     func() FieldDecoder { return new(IDSet) },
		"BitSet":    func() FieldDecoder { return new(BitSet) },
		"Array":     func() FieldDecoder { var v []VarInt; return Array(&v) },
	}
	lengths := map[string]int32{
		"negative":              -1,
		"beyond-remaining":      64,         // only a few bytes follow
		"huge-beyond-remaining": 0x7fffffff, // 5 bytes that used to allocate 2 GB
	}
	for name, newDecoder := range decoders {
		for lname, length := range lengths {
			t.Run(name+"/"+lname, func(t *testing.T) {
				// IDSet stores length+1, so -1 there is the "tag name" marker 0;
				// use -2 to encode a negative count.
				if name == "IDSet" && length == -1 {
					length = -2
				}
				input := append(varIntBytes(length), 1, 2, 3)
				var err error
				alloc := allocatedBy(func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("panicked: %v", r)
						}
					}()
					_, err = newDecoder().ReadFrom(bytes.NewReader(input))
				})
				if err == nil {
					t.Fatal("expected an error")
				}
				if alloc > 1<<20 {
					t.Fatalf("allocated %d bytes before rejecting the length", alloc)
				}
			})
		}
	}
}

func TestStringRoundTrip(t *testing.T) {
	for _, want := range []String{"", "a", "minecraft:stone", String(bytes.Repeat([]byte("龍"), 1000))} {
		var buf bytes.Buffer
		if _, err := want.WriteTo(&buf); err != nil {
			t.Fatal(err)
		}
		var got String
		if _, err := got.ReadFrom(&buf); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

// oneByteReader returns at most one byte per Read, like a slow socket.
type oneByteReader struct{ r io.Reader }

func (o oneByteReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return o.r.Read(p)
}

func TestFixedBitSetReadsAcrossShortReads(t *testing.T) {
	f := NewFixedBitSet(32)
	if _, err := f.ReadFrom(oneByteReader{bytes.NewReader([]byte{1, 2, 3, 4})}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f, []byte{1, 2, 3, 4}) {
		t.Fatalf("got %v", []byte(f))
	}
}

// TestUnpackBackToBackFrames reads a mixed stream of compressed, below-threshold
// and empty packets from one reader: every frame must be consumed exactly, or
// the following packet is decoded from the wrong offset.
func TestUnpackBackToBackFrames(t *testing.T) {
	packets := []Packet{
		{ID: 1, Data: joinGamePayload},
		{ID: 2, Data: []byte("tiny")},
		{ID: 3, Data: bytes.Repeat([]byte("abc"), 500)},
		{ID: 4, Data: nil},
		{ID: 5, Data: bytes.Repeat([]byte{0}, 300)},
		{ID: 6, Data: joinGamePayload[:257]},
	}
	for _, threshold := range []int{-1, 0, 256} {
		var stream bytes.Buffer
		for i := range packets {
			if err := packets[i].Pack(&stream, threshold); err != nil {
				t.Fatal(err)
			}
		}
		r := oneByteReader{&stream}
		var got Packet // reused, as Conn callers do
		for _, want := range packets {
			if err := got.UnPack(r, threshold); err != nil {
				t.Fatalf("threshold %d, packet %d: %v", threshold, want.ID, err)
			}
			if got.ID != want.ID || !bytes.Equal(got.Data, want.Data) {
				t.Fatalf("threshold %d: got packet %d (%d bytes), want %d (%d bytes)",
					threshold, got.ID, len(got.Data), want.ID, len(want.Data))
			}
		}
		if stream.Len() != 0 {
			t.Fatalf("threshold %d: %d bytes left unread", threshold, stream.Len())
		}
	}
}

func FuzzUnpack(f *testing.F) {
	for _, threshold := range []int{-1, 256} {
		for _, p := range []Packet{{ID: 1, Data: []byte("hi")}, {ID: 2, Data: joinGamePayload[:600]}} {
			var buf bytes.Buffer
			if err := p.Pack(&buf, threshold); err != nil {
				f.Fatal(err)
			}
			f.Add(buf.Bytes(), threshold < 0)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte, uncompressed bool) {
		threshold := 256
		if uncompressed {
			threshold = -1
		}
		var p Packet
		r := bytes.NewReader(data)
		for range 4 {
			if p.UnPack(r, threshold) != nil {
				return
			}
		}
	})
}

func FuzzFieldDecoders(f *testing.F) {
	f.Add([]byte{3, 'a', 'b', 'c'})
	f.Add(append(varIntBytes(-1), 1, 2))
	f.Add(append(varIntBytes(0x7fffffff), 0))
	f.Add([]byte{0, 2, 'm', 'c'})
	f.Fuzz(func(t *testing.T, data []byte) {
		var (
			s   String
			ba  ByteArray
			ids IDSet
			bs  BitSet
			arr []VarInt
			id  Identifier
		)
		for _, d := range []FieldDecoder{&s, &ba, &ids, &bs, Array(&arr), &id} {
			_, _ = d.ReadFrom(bytes.NewReader(data))
		}
	})
}
