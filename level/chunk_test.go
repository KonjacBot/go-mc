package level

import (
	"bytes"
	"testing"

	pk "github.com/KonjacBot/go-mc/net/packet"
)

func TestChunkReadFromUses26_2HeightmapsCodec(t *testing.T) {
	data := []byte{
		1, // heightmaps map size
		0, // heightmap type
		1, // long array length
		0, 0, 0, 0, 0, 0, 0, 42,
		0,                // chunk data
		0,                // block entities
		0, 0, 0, 0, 0, 0, // light masks and arrays
	}

	n, err := EmptyChunk(0).ReadFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(data)) {
		t.Fatalf("read %d bytes, want %d", n, len(data))
	}
}

func TestSectionReadFromUses26_1FluidCount(t *testing.T) {
	data := []byte{
		0, 0, // block count
		0, 7, // fluid count
		0, 0, // block states: single air
		0, 0, // biomes: single plains
	}

	section := EmptyChunk(1).Sections[0]
	n, err := section.ReadFrom(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(data)) {
		t.Fatalf("read %d bytes, want %d", n, len(data))
	}
	if section.FluidCount != 7 {
		t.Fatalf("fluid count = %d, want 7", section.FluidCount)
	}
}

func TestSectionWriteToUses26_1FluidCount(t *testing.T) {
	var buf bytes.Buffer
	section := EmptyChunk(1).Sections[0]
	section.FluidCount = 7
	n, err := section.WriteTo(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 8 {
		t.Fatalf("wrote %d bytes, want 8", n)
	}
	if got, want := buf.Bytes(), []byte{0, 0, 0, 7, 0, 0, 0, 0}; !bytes.Equal(got, want) {
		t.Fatalf("encoded %v, want %v", got, want)
	}
}

func TestEmptyChunkWritesBoundedLightMasks(t *testing.T) {
	const sectionCount = 24
	var buf bytes.Buffer
	if _, err := EmptyChunk(sectionCount).WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() >= 1024 {
		t.Fatalf("empty chunk encoded to %d bytes, want less than 1024", buf.Len())
	}

	decoded := EmptyChunk(sectionCount)
	if _, err := decoded.ReadFrom(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	if decoded.Light == nil {
		t.Fatal("decoded chunk has no light data")
	}

	light := decoded.Light
	if got := len(light.SkyLightMask); got != 1 {
		t.Fatalf("sky light mask has %d words, want 1", got)
	}
	if got := len(light.BlockLightMask); got != 1 {
		t.Fatalf("block light mask has %d words, want 1", got)
	}
	if got := len(light.EmptySkyLightMask); got != 1 {
		t.Fatalf("empty sky light mask has %d words, want 1", got)
	}
	if got := len(light.EmptyBlockLightMask); got != 1 {
		t.Fatalf("empty block light mask has %d words, want 1", got)
	}

	const lightSectionCount = sectionCount + 2
	const allLightSections int64 = 1<<lightSectionCount - 1
	if got := light.SkyLightMask[0]; got != 0 {
		t.Fatalf("sky light mask = %#x, want 0", got)
	}
	if got := light.BlockLightMask[0]; got != 0 {
		t.Fatalf("block light mask = %#x, want 0", got)
	}
	if got := light.EmptySkyLightMask[0]; got != allLightSections {
		t.Fatalf("empty sky light mask = %#x, want %#x", got, allLightSections)
	}
	if got := light.EmptyBlockLightMask[0]; got != allLightSections {
		t.Fatalf("empty block light mask = %#x, want %#x", got, allLightSections)
	}
	if len(light.SkyLight) != 0 || len(light.BlockLight) != 0 {
		t.Fatal("empty chunk unexpectedly contains light arrays")
	}
}

func TestChunkWritesFullTopSkyLightBoundary(t *testing.T) {
	const sectionCount = 24
	chunk := EmptyChunk(sectionCount)
	chunk.Sections[sectionCount-1].SkyLight = bytes.Repeat([]byte{0xff}, 2048)

	var wire bytes.Buffer
	if _, err := chunk.WriteTo(&wire); err != nil {
		t.Fatal(err)
	}
	decoded := EmptyChunk(sectionCount)
	if _, err := decoded.ReadFrom(bytes.NewReader(wire.Bytes())); err != nil {
		t.Fatal(err)
	}

	const topLightIndex = sectionCount + 1
	if !decoded.Light.SkyLightMask.Get(topLightIndex) {
		t.Fatal("top sky-light boundary is not present")
	}
	if decoded.Light.EmptySkyLightMask.Get(topLightIndex) {
		t.Fatal("top sky-light boundary is also marked empty")
	}
	if got := len(decoded.Light.SkyLight); got != 2 {
		t.Fatalf("sky-light arrays = %d, want section plus top boundary", got)
	}
	for index, value := range decoded.Light.SkyLight[1] {
		if value != 0xff {
			t.Fatalf("top sky-light byte %d = %#x, want 0xff", index, value)
		}
	}
}

func TestLightDataRejectsOverlappingMasks(t *testing.T) {
	light := LightData{
		SkyLightMask:      pk.BitSet{1},
		EmptySkyLightMask: pk.BitSet{1},
		SkyLight:          []pk.ByteArray{make([]byte, 2048)},
	}
	if _, err := light.WriteTo(&bytes.Buffer{}); err == nil {
		t.Fatal("expected overlapping light masks to be rejected")
	}
}

func TestLightDataRejectsInvalidArrayLength(t *testing.T) {
	light := LightData{
		SkyLightMask: pk.BitSet{1},
		SkyLight:     []pk.ByteArray{make([]byte, 2047)},
	}
	if _, err := light.WriteTo(&bytes.Buffer{}); err == nil {
		t.Fatal("expected invalid sky light array length to be rejected")
	}
}

func BenchmarkEmptyChunkWrite24Sections(b *testing.B) {
	chunk := EmptyChunk(24)
	var buf bytes.Buffer
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		buf.Reset()
		if _, err := chunk.WriteTo(&buf); err != nil {
			b.Fatal(err)
		}
	}
}
