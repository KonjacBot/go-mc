package level

import (
	"bytes"
	"testing"

	"github.com/KonjacBot/go-mc/nbt"
)

func TestChunkReadFromUsesNetworkNBTHeightmaps(t *testing.T) {
	data := []byte{
		nbt.TagCompound, nbt.TagEnd, // heightmaps
		0,                // chunk data
		0,                // block entities
		0, 0, 0, 0, 0, 0, // light masks and arrays
	}

	if _, err := EmptyChunk(0).ReadFrom(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}
