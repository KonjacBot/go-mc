package packet

import (
	"bytes"
	_ "embed"
	"testing"
)

//go:embed joingame_test.bin
var joinGamePayload []byte

// packFrames packs p count times back to back with the given threshold.
func packFrames(tb testing.TB, p Packet, threshold, count int) []byte {
	tb.Helper()
	var buf bytes.Buffer
	for range count {
		if err := p.Pack(&buf, threshold); err != nil {
			tb.Fatal(err)
		}
	}
	return buf.Bytes()
}

func benchmarkUnpack(b *testing.B, payload []byte, threshold int) {
	const frames = 64
	stream := packFrames(b, Packet{ID: 0x2b, Data: payload}, threshold, frames)
	r := bytes.NewReader(stream)
	var p Packet

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := range b.N {
		if i%frames == 0 {
			r.Reset(stream)
		}
		if err := p.UnPack(r, threshold); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnpack(b *testing.B) {
	small := bytes.Repeat([]byte("minecraft "), 30) // 300 B, compressible
	b.Run("uncompressed/1KB", func(b *testing.B) { benchmarkUnpack(b, make([]byte, 1024), -1) })
	b.Run("below-threshold/100B", func(b *testing.B) { benchmarkUnpack(b, small[:100], 256) })
	b.Run("compressed/300B", func(b *testing.B) { benchmarkUnpack(b, small, 256) })
	b.Run("compressed/joingame-30KB", func(b *testing.B) { benchmarkUnpack(b, joinGamePayload, 256) })
}

func BenchmarkStringReadFrom(b *testing.B) {
	var buf bytes.Buffer
	if _, err := String("minecraft:overworld_block_state").WriteTo(&buf); err != nil {
		b.Fatal(err)
	}
	encoded := buf.Bytes()
	r := bytes.NewReader(encoded)
	var s String

	b.ReportAllocs()
	for range b.N {
		r.Reset(encoded)
		if _, err := s.ReadFrom(r); err != nil {
			b.Fatal(err)
		}
	}
}
