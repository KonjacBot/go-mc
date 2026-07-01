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
