package net

import (
	"bytes"
	"crypto/aes"
	"io"
	"testing"

	"github.com/KonjacBot/go-mc/net/CFB8"
	pk "github.com/KonjacBot/go-mc/net/packet"
)

// TestSetCipherAfterBufferedRead covers the login sequence: the server's last
// plaintext packet and its first encrypted packets often arrive in one TCP
// read, so the read buffer already holds ciphertext when SetCipher is called.
// That ciphertext must still go through the decrypter.
func TestSetCipherAfterBufferedRead(t *testing.T) {
	const threshold = 256
	plainFirst := pk.Packet{ID: 0x01, Data: []byte("encryption request")}
	encrypted := []pk.Packet{
		{ID: 0x02, Data: []byte("login success")},
		{ID: 0x03, Data: bytes.Repeat([]byte("chunk"), 400)}, // compressed
		{ID: 0x04, Data: []byte{}},
	}

	var stream bytes.Buffer
	if err := plainFirst.Pack(&stream, threshold); err != nil {
		t.Fatal(err)
	}
	stream.Write(encryptedStream(t, encrypted, threshold))

	sc := &streamConn{r: bytes.NewReader(stream.Bytes())}
	c := WrapConn(sc)
	c.SetThreshold(threshold)

	var p pk.Packet
	if err := c.ReadPacket(&p); err != nil {
		t.Fatal(err)
	}
	if p.ID != plainFirst.ID || !bytes.Equal(p.Data, plainFirst.Data) {
		t.Fatalf("plaintext packet: got %d %q", p.ID, p.Data)
	}
	if sc.r.Len() != 0 {
		t.Fatalf("test setup: expected the whole stream to be buffered, %d bytes left", sc.r.Len())
	}

	block, err := aes.NewCipher(benchKey)
	if err != nil {
		t.Fatal(err)
	}
	c.SetCipher(CFB8.NewCFB8Encrypt(block, benchKey), CFB8.NewCFB8Decrypt(block, benchKey))
	for _, want := range encrypted {
		if err := c.ReadPacket(&p); err != nil {
			t.Fatalf("packet %d: %v", want.ID, err)
		}
		if p.ID != want.ID || !bytes.Equal(p.Data, want.Data) {
			t.Fatalf("got packet %d (%d bytes), want %d (%d bytes)", p.ID, len(p.Data), want.ID, len(want.Data))
		}
	}
	if err := c.ReadPacket(&p); err != io.EOF {
		t.Fatalf("after the last packet: got %v, want EOF", err)
	}
}

func TestCipherReaderReadByteMatchesRead(t *testing.T) {
	block, err := aes.NewCipher(benchKey)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("the quick brown fox jumps over the lazy dog")
	ct := make([]byte, len(plain))
	CFB8.NewCFB8Encrypt(block, benchKey).XORKeyStream(ct, plain)

	// Mix ReadByte and Read, over a source with and without ReadByte.
	for _, src := range []io.Reader{bytes.NewReader(ct), struct{ io.Reader }{bytes.NewReader(ct)}} {
		r := newCipherReader(CFB8.NewCFB8Decrypt(block, benchKey), src)
		var got []byte
		for len(got) < len(plain) {
			if len(got)%3 == 0 {
				b, err := r.ReadByte()
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, b)
				continue
			}
			buf := make([]byte, 5)
			n, err := r.Read(buf)
			got = append(got, buf[:n]...)
			if err != nil && err != io.EOF {
				t.Fatal(err)
			}
		}
		if !bytes.Equal(got[:len(plain)], plain) {
			t.Fatalf("got %q", got)
		}
	}
}

func TestBuffered(t *testing.T) {
	var stream bytes.Buffer
	for i := range 3 {
		p := pk.Packet{ID: int32(i), Data: []byte("abc")}
		if err := p.Pack(&stream, -1); err != nil {
			t.Fatal(err)
		}
	}
	total := stream.Len()
	c := WrapConn(&streamConn{r: bytes.NewReader(stream.Bytes())})
	if c.Buffered() != 0 {
		t.Fatalf("Buffered before any read = %d", c.Buffered())
	}
	var p pk.Packet
	if err := c.ReadPacket(&p); err != nil {
		t.Fatal(err)
	}
	if got, want := c.Buffered(), total*2/3; got != want {
		t.Fatalf("Buffered after one packet = %d, want %d", got, want)
	}
	if (&Conn{}).Buffered() != 0 {
		t.Fatal("Buffered on an unwrapped Conn should be 0")
	}
}
