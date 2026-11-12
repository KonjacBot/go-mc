package packet

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"net"
	"sync"
)

const MaxDataLength = 0x200000

// Packet define a net data package
type Packet struct {
	ID   int32
	Data []byte
}

// PreparedPacket is an immutable, fully framed packet that can be reused
// across connections using the same compression threshold. Preparing large,
// static packets once avoids repeating zlib work for every recipient.
type PreparedPacket struct {
	wire      []byte
	threshold int
}

// Marshal generate Packet with the ID and Fields
func Marshal[ID ~int32 | int](id ID, fields ...FieldEncoder) (pk Packet) {
	var pb Builder
	for _, v := range fields {
		pb.WriteField(v)
	}
	return pb.Packet(int32(id))
}

// Prepare frames and, when required, compresses a packet for repeated writes.
func (p Packet) Prepare(threshold int) (PreparedPacket, error) {
	var wire bytes.Buffer
	if err := p.Pack(&wire, threshold); err != nil {
		return PreparedPacket{}, err
	}
	return PreparedPacket{
		wire:      wire.Bytes(),
		threshold: threshold,
	}, nil
}

// Threshold reports the compression threshold used while preparing the packet.
func (p PreparedPacket) Threshold() int {
	return p.threshold
}

// Len reports the framed packet size in bytes.
func (p PreparedPacket) Len() int {
	return len(p.wire)
}

// WriteTo writes the complete prepared frame, including when the destination
// performs a legal short write.
func (p PreparedPacket) WriteTo(w io.Writer) (n int64, err error) {
	for len(p.wire) > 0 {
		written, writeErr := w.Write(p.wire)
		n += int64(written)
		p.wire = p.wire[written:]
		if writeErr != nil {
			return n, writeErr
		}
		if written == 0 {
			return n, io.ErrShortWrite
		}
	}
	return n, nil
}

// Scan decode the packet and fill data into fields
func (p Packet) Scan(fields ...FieldDecoder) error {
	r := bytes.NewReader(p.Data)
	for i, v := range fields {
		_, err := v.ReadFrom(r)
		if err != nil {
			return fmt.Errorf("scanning packet field[%d] error: %w", i, err)
		}
	}
	return nil
}

var (
	bufPool  = sync.Pool{New: func() any { return new(bytes.Buffer) }}
	zlibPool = sync.Pool{New: func() any { return zlib.NewWriter(io.Discard) }}
)

// Pack 打包一个数据包
func (p *Packet) Pack(w io.Writer, threshold int) error {
	if threshold >= 0 {
		return p.packWithCompression(w, threshold)
	} else {
		return p.packWithoutCompression(w)
	}
}

func (p *Packet) packWithoutCompression(w io.Writer) error {
	var lenBuf [MaxVarIntLen]byte
	var idBuf [MaxVarIntLen]byte

	idN := VarInt(p.ID).WriteToBytes(idBuf[:])
	pktLen := VarInt(idN + len(p.Data))
	lenN := pktLen.WriteToBytes(lenBuf[:])

	if vw, ok := w.(interface{ Writev([][]byte) (int, error) }); ok {
		_, err := vw.Writev([][]byte{
			lenBuf[:lenN],
			idBuf[:idN],
			p.Data,
		})
		return err
	}

	if nw, ok := w.(*net.TCPConn); ok {
		bufs := net.Buffers{lenBuf[:lenN], idBuf[:idN], p.Data}
		_, err := bufs.WriteTo(nw)
		return err
	}

	// One Write per packet: on an encrypted connection every Write is a
	// separate syscall, so writing the three parts separately tripled them.
	buff := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buff)
	buff.Reset()
	buff.Write(lenBuf[:lenN])
	buff.Write(idBuf[:idN])
	buff.Write(p.Data)
	_, err := w.Write(buff.Bytes())
	return err
}

func (p *Packet) packWithCompression(w io.Writer, threshold int) error {
	buff := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buff)
	buff.Reset()

	PacketID := VarInt(p.ID)
	if len(p.Data) < threshold {
		DataLength := VarInt(0) // uncompressed mark
		PacketLength := VarInt(DataLength.Len() + PacketID.Len() + len(p.Data))
		_, _ = PacketLength.WriteTo(buff)
		_, _ = DataLength.WriteTo(buff)
		_, _ = PacketID.WriteTo(buff)
		_, _ = buff.Write(p.Data)
	} else {
		DataLength := VarInt(PacketID.Len() + len(p.Data))

		var zeroPad [MaxVarIntLen]byte
		buff.Write(zeroPad[:])
		_, _ = DataLength.WriteTo(buff)
		if err := compressPacket(buff, p.ID, p.Data); err != nil {
			return err
		}

		PacketLength := VarInt(buff.Len() - MaxVarIntLen)
		packetLengthLen := PacketLength.Len()
		buff.Next(MaxVarIntLen - packetLengthLen)
		PacketLength.WriteToBytes(buff.Bytes()[:packetLengthLen])
	}

	_, err := w.Write(buff.Bytes())
	return err
}

func compressPacket(w io.Writer, packetID int32, data []byte) error {
	zw := zlibPool.Get().(*zlib.Writer)
	defer zlibPool.Put(zw)
	zw.Reset(w)

	_, _ = VarInt(packetID).WriteTo(zw)
	if _, err := zw.Write(data); err != nil {
		return err
	}
	return zw.Close()
}

// UnPack in-place decompression a packet
func (p *Packet) UnPack(r io.Reader, threshold int) error {
	if threshold >= 0 {
		return p.unpackWithCompression(r, threshold)
	} else {
		return p.unpackWithoutCompression(r)
	}
}

func (p *Packet) unpackWithoutCompression(r io.Reader) error {
	var Length VarInt
	_, err := Length.ReadFrom(r)
	if err != nil {
		return err
	}

	var PacketID VarInt
	n, err := PacketID.ReadFrom(r)
	if err != nil {
		return err
	}
	p.ID = int32(PacketID)

	lengthOfData := int(Length) - int(n)
	if lengthOfData < 0 || lengthOfData > MaxDataLength {
		return fmt.Errorf("uncompressed packet error: length is %d", lengthOfData)
	}
	if cap(p.Data) < lengthOfData {
		p.Data = make([]byte, lengthOfData)
	} else {
		p.Data = p.Data[:lengthOfData]
	}
	_, err = io.ReadFull(r, p.Data)
	if err != nil {
		return err
	}
	return nil
}

// inflater holds the per-frame state of compressed decoding so it can be
// reused: the raw frame buffer, a reader over it and a zlib reader. Creating a
// zlib reader allocates its ~40 KB window, which used to dominate the cost of
// reading a compressed packet.
type inflater struct {
	frame []byte
	br    bytes.Reader
	zr    io.ReadCloser // always also a zlib.Resetter
}

// maxPooledFrame caps the frame buffer kept in the pool, so one huge packet
// does not pin megabytes of memory for the life of the process.
const maxPooledFrame = 256 << 10

var inflaterPool = sync.Pool{New: func() any { return new(inflater) }}

// zlibReader returns a zlib reader positioned at the start of inf.br.
func (inf *inflater) zlibReader() (io.Reader, error) {
	if inf.zr == nil {
		zr, err := zlib.NewReader(&inf.br)
		if err != nil {
			return nil, err
		}
		inf.zr = zr
		return zr, nil
	}
	if err := inf.zr.(zlib.Resetter).Reset(&inf.br, nil); err != nil {
		return nil, err
	}
	return inf.zr, nil
}

func (inf *inflater) release() {
	if cap(inf.frame) > maxPooledFrame {
		inf.frame = nil
	}
	inf.br.Reset(nil)
	inflaterPool.Put(inf)
}

func (p *Packet) unpackWithCompression(r io.Reader, threshold int) error {
	var packetLength VarInt
	_, err := packetLength.ReadFrom(r)
	if err != nil {
		return err
	}
	if packetLength < 0 || packetLength > MaxDataLength+MaxVarIntLen*2 {
		return fmt.Errorf("compressed packet error: invalid packet length %d", packetLength)
	}

	// Read the whole frame first. This always consumes exactly packetLength
	// bytes from r, including the zlib checksum the decompressor may not ask
	// for, so the next frame starts where it should. It also lets the
	// decompressor read from memory through io.ByteReader, which avoids the
	// bufio.Reader that flate would otherwise allocate per packet.
	inf := inflaterPool.Get().(*inflater)
	defer inf.release()
	if cap(inf.frame) < int(packetLength) {
		inf.frame = make([]byte, packetLength)
	} else {
		inf.frame = inf.frame[:packetLength]
	}
	if _, err := io.ReadFull(r, inf.frame); err != nil {
		return err
	}
	inf.br.Reset(inf.frame)

	var dataLength VarInt
	if _, err := dataLength.ReadFrom(&inf.br); err != nil {
		return err
	}

	var packetID VarInt
	if dataLength == 0 {
		// Below the threshold: the rest of the frame is the raw packet.
		if _, err := packetID.ReadFrom(&inf.br); err != nil {
			return err
		}
		payloadLen := inf.br.Len()
		if payloadLen > MaxDataLength {
			return fmt.Errorf("compressed packet error: invalid payload length %d", payloadLen)
		}
		p.ID = int32(packetID)
		p.Data = append(p.Data[:0], inf.frame[len(inf.frame)-payloadLen:]...)
		return nil
	}

	if int(dataLength) < threshold {
		return fmt.Errorf("compressed packet error: size of %d is below threshold of %d", dataLength, threshold)
	}
	if dataLength > MaxDataLength {
		return fmt.Errorf("compressed packet error: size of %d is larger than protocol maximum of %d", dataLength, MaxDataLength)
	}

	zr, err := inf.zlibReader()
	if err != nil {
		return err
	}
	n3, err := packetID.ReadFrom(zr)
	if err != nil {
		return err
	}
	payloadLen := int(dataLength) - int(n3)
	if payloadLen < 0 || payloadLen > MaxDataLength {
		return fmt.Errorf("compressed packet error: invalid payload length %d", payloadLen)
	}

	if cap(p.Data) < payloadLen {
		p.Data = make([]byte, payloadLen)
	} else {
		p.Data = p.Data[:payloadLen]
	}
	p.ID = int32(packetID)
	_, err = io.ReadFull(zr, p.Data)
	return err
}
