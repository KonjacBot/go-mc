package packet

import (
	"bytes"
	"testing"
)

func TestByteArrayReadRejectsNegativeLength(t *testing.T) {
	var b ByteArray
	if _, err := b.ReadFrom(bytes.NewReader([]byte{0xff, 0xff, 0xff, 0xff, 0x0f})); err == nil {
		t.Fatal("expected error for negative byte array length")
	}
}
