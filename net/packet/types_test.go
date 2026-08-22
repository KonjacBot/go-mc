package packet

import (
	"bytes"
	"testing"
	"unsafe"
)

func TestByteArrayReadRejectsNegativeLength(t *testing.T) {
	var b ByteArray
	if _, err := b.ReadFrom(bytes.NewReader([]byte{0xff, 0xff, 0xff, 0xff, 0x0f})); err == nil {
		t.Fatal("expected error for negative byte array length")
	}
}

func TestBitSetReadRejectsNegativeLength(t *testing.T) {
	var encoded bytes.Buffer
	if _, err := VarInt(-1).WriteTo(&encoded); err != nil {
		t.Fatal(err)
	}

	var bitSet BitSet
	if _, err := bitSet.ReadFrom(&encoded); err == nil {
		t.Fatal("expected error for negative bit set length")
	}
}

func TestBitSetReadRejectsOversizedLength(t *testing.T) {
	var encoded bytes.Buffer
	if _, err := VarInt(MaxDataLength/8 + 1).WriteTo(&encoded); err != nil {
		t.Fatal(err)
	}

	var bitSet BitSet
	if _, err := bitSet.ReadFrom(&encoded); err == nil {
		t.Fatal("expected error for oversized bit set length")
	}
}

func TestArrayReadResizesReusedSlice(t *testing.T) {
	values := []VarInt{1, 2, 3}
	var encoded bytes.Buffer
	if _, err := VarInt(1).WriteTo(&encoded); err != nil {
		t.Fatal(err)
	}
	if _, err := VarInt(9).WriteTo(&encoded); err != nil {
		t.Fatal(err)
	}
	if _, err := Array(&values).ReadFrom(&encoded); err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0] != 9 {
		t.Fatalf("decoded values = %v, want [9]", values)
	}
}

func TestArrayReadRejectsOversizedAllocation(t *testing.T) {
	var encoded bytes.Buffer
	tooMany := MaxDataLength/int(unsafe.Sizeof(VarInt(0))) + 1
	if _, err := VarInt(tooMany).WriteTo(&encoded); err != nil {
		t.Fatal(err)
	}

	var values []VarInt
	if _, err := Array(&values).ReadFrom(&encoded); err == nil {
		t.Fatal("expected error for oversized array allocation")
	}
	if values != nil {
		t.Fatalf("oversized decode allocated %d elements", len(values))
	}
}
