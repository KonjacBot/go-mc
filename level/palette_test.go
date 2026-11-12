package level

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/KonjacBot/go-mc/level/block"
)

func TestPaletteContainer_seq(t *testing.T) {
	container := NewStatesPaletteContainer(4096, 0)
	for i := 0; i < 4096; i++ {
		container.Set(i, BlocksState(i))
	}
	for i := 0; i < 4096; i++ {
		if container.Get(i) != BlocksState(i) {
			t.Errorf("Get Error, got: %v, but expect: %v", container.Get(i), BlocksState(i))
		}
	}
}

func TestPaletteContainer_rand(t *testing.T) {
	data := make([]BlocksState, 4096)
	for i := range data {
		data[i] = BlocksState(rand.Intn(1 << block.BitsPerBlock))
	}
	container := NewStatesPaletteContainer(4096, 0)
	for i, v := range data {
		container.Set(i, v)
	}
	for i, v := range data {
		if v2 := container.Get(i); v != v2 {
			t.Errorf("value not match: got %v, except: %v", v2, v)
		}
	}
}

func TestPaletteContainerReadRejectsEmptyLinearPalette(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(4)
	buf.WriteByte(0)
	buf.Write(make([]byte, calcBitStorageSize(4, 4096)*8))

	container := NewStatesPaletteContainer(4096, 0)
	if _, err := container.ReadFrom(&buf); err == nil {
		t.Fatal("expected error for empty linear palette")
	}
}

func BenchmarkPaletteContainer(b *testing.B) {
	data := make([]BlocksState, 4096)
	for i := range data {
		data[i] = BlocksState(rand.Intn(1 << block.BitsPerBlock))
	}
	rand.Shuffle(len(data), func(i, j int) { data[i], data[j] = data[j], data[i] })
	container := NewStatesPaletteContainer(4096, 0)
	b.ResetTimer()
	b.Run("Set", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			index := i % 4096
			container.Set(index, data[index])
		}
	})
	b.Run("Get", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			index := i % 4096
			container.Get(index)
		}
	})
}

func TestHashPaletteBuildsIndexOnFirstWrite(t *testing.T) {
	values := make([]BlocksState, 20) // 20 entries -> 5 bits -> hash palette
	for i := range values {
		values[i] = BlocksState(100 + i)
	}
	data := make([]uint64, calcBitStorageSize(5, 4096))
	c := NewStatesPaletteContainerWithData(4096, data, values)
	if hp, ok := c.palette.(*hashPalette[BlocksState]); !ok || hp.ids != nil {
		t.Fatalf("palette = %T with ids built eagerly", c.palette)
	}
	if got := c.Get(0); got != 100 {
		t.Fatalf("Get(0) = %d, want 100", got)
	}
	c.Set(1, 119) // existing value: must reuse index 19
	c.Set(2, 500) // new value: appended
	if c.Get(1) != 119 || c.Get(2) != 500 || c.Get(3) != 100 {
		t.Fatalf("after Set: %d %d %d", c.Get(1), c.Get(2), c.Get(3))
	}
	// Adding 500 outgrows the decoded palette, so the container is rebuilt
	// from the values in use; 119 must still appear only once.
	seen := 0
	for _, v := range c.palette.export() {
		if v == 119 {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("119 appears %d times in the palette, want 1", seen)
	}
}

func TestSingleValue(t *testing.T) {
	c := NewStatesPaletteContainer(4096, 7)
	if v, ok := c.SingleValue(); !ok || v != 7 {
		t.Fatalf("SingleValue = %d, %v; want 7, true", v, ok)
	}
	c.Set(0, 8)
	if _, ok := c.SingleValue(); ok {
		t.Fatal("SingleValue reported true after a second value was set")
	}
	if v, ok := NewBiomesPaletteContainer(64, 3).SingleValue(); !ok || v != 3 {
		t.Fatalf("biomes SingleValue = %d, %v; want 3, true", v, ok)
	}
}
