package level

import (
	"bytes"
	"testing"
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
