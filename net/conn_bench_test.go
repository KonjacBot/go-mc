package net

import (
	"bytes"
	"crypto/aes"
	"net"
	"testing"

	"github.com/KonjacBot/go-mc/net/CFB8"
	pk "github.com/KonjacBot/go-mc/net/packet"
)

// streamConn is a net.Conn that serves a fixed byte stream and counts Read
// calls, which stand in for syscalls on a real socket.
type streamConn struct {
	net.Conn
	r     *bytes.Reader
	reads int
}

func (c *streamConn) Read(p []byte) (int, error) {
	c.reads++
	return c.r.Read(p)
}

func (c *streamConn) Write(p []byte) (int, error) { return len(p), nil }

var benchKey = []byte("0123456789abcdef")

// encryptedStream frames packets with the threshold and encrypts them the
// way a server does after login.
func encryptedStream(tb testing.TB, packets []pk.Packet, threshold int) []byte {
	tb.Helper()
	var plain bytes.Buffer
	for i := range packets {
		if err := packets[i].Pack(&plain, threshold); err != nil {
			tb.Fatal(err)
		}
	}
	block, err := aes.NewCipher(benchKey)
	if err != nil {
		tb.Fatal(err)
	}
	out := make([]byte, plain.Len())
	CFB8.NewCFB8Encrypt(block, benchKey).XORKeyStream(out, plain.Bytes())
	return out
}

func newEncryptedConn(tb testing.TB, stream []byte, threshold int) (*Conn, *streamConn) {
	tb.Helper()
	sc := &streamConn{r: bytes.NewReader(stream)}
	c := WrapConn(sc)
	block, err := aes.NewCipher(benchKey)
	if err != nil {
		tb.Fatal(err)
	}
	c.SetCipher(CFB8.NewCFB8Encrypt(block, benchKey), CFB8.NewCFB8Decrypt(block, benchKey))
	c.SetThreshold(threshold)
	return c, sc
}

// mixedPackets approximates play traffic: many small packets (movement,
// keepalive), some medium ones and an occasional large chunk-sized one.
func mixedPackets() []pk.Packet {
	sizes := []int{12, 24, 40, 12, 90, 300, 24, 1500, 12, 40, 24, 30000}
	packets := make([]pk.Packet, 0, len(sizes)*8)
	for range 8 {
		for i, n := range sizes {
			data := bytes.Repeat([]byte{byte(i), 'm', 'c', byte(n)}, n/4+1)[:n]
			packets = append(packets, pk.Packet{ID: int32(i), Data: data})
		}
	}
	return packets
}

func BenchmarkConnReadPacket(b *testing.B) {
	const threshold = 256
	packets := mixedPackets()
	var total int
	for _, p := range packets {
		total += len(p.Data)
	}
	stream := encryptedStream(b, packets, threshold)

	c, sc := newEncryptedConn(b, stream, threshold)
	var p pk.Packet
	reads := 0

	b.ReportAllocs()
	b.SetBytes(int64(total / len(packets)))
	b.ResetTimer()
	for i := range b.N {
		if i > 0 && i%len(packets) == 0 {
			b.StopTimer()
			reads += sc.reads
			c, sc = newEncryptedConn(b, stream, threshold)
			b.StartTimer()
		}
		if err := c.ReadPacket(&p); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	reads += sc.reads
	b.ReportMetric(float64(reads)/float64(b.N), "reads/packet")
}
