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

// Marshal generate Packet with the ID and Fields
func Marshal[ID ~int32 | int](id ID, fields ...FieldEncoder) (pk Packet) {
	var pb Builder
	for _, v := range fields {
		pb.WriteField(v)
	}
	return pb.Packet(int32(id))
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

	_, err := w.Write(lenBuf[:lenN])
	if err != nil {
		return err
	}
	_, err = w.Write(idBuf[:idN])
	if err != nil {
		return err
	}
	_, err = w.Write(p.Data)
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

func (p *Packet) unpackWithCompression(r io.Reader, threshold int) error {
	var packetLength VarInt
	_, err := packetLength.ReadFrom(r)
	if err != nil {
		return err
	}
	if packetLength < 0 || packetLength > MaxDataLength+MaxVarIntLen*2 {
		return fmt.Errorf("compressed packet error: invalid packet length %d", packetLength)
	}

	lr := &io.LimitedReader{R: r, N: int64(packetLength)}

	var dataLength VarInt
	n2, err := dataLength.ReadFrom(lr)
	if err != nil {
		return err
	}

	var payloadReader io.Reader = lr
	var packetID VarInt
	var payloadLen int

	if dataLength != 0 {
		if int(dataLength) < threshold {
			return fmt.Errorf("compressed packet error: size of %d is below threshold of %d", dataLength, threshold)
		}
		if dataLength > MaxDataLength {
			return fmt.Errorf("compressed packet error: size of %d is larger than protocol maximum of %d", dataLength, MaxDataLength)
		}

		zr, err := zlib.NewReader(lr)
		if err != nil {
			return err
		}
		defer zr.Close()

		payloadReader = zr

		n3, err := packetID.ReadFrom(payloadReader)
		if err != nil {
			return err
		}

		payloadLen = int(dataLength) - int(n3)
		if payloadLen < 0 || payloadLen > MaxDataLength {
			return fmt.Errorf("compressed packet error: invalid payload length %d", payloadLen)
		}
	} else {
		n3, err := packetID.ReadFrom(payloadReader)
		if err != nil {
			return err
		}

		payloadLen = int(packetLength) - int(n2) - int(n3)
		if payloadLen < 0 || payloadLen > MaxDataLength {
			return fmt.Errorf("compressed packet error: invalid payload length %d", payloadLen)
		}
	}

	if cap(p.Data) < payloadLen {
		p.Data = make([]byte, payloadLen)
	} else {
		p.Data = p.Data[:payloadLen]
	}

	p.ID = int32(packetID)
	_, err = io.ReadFull(payloadReader, p.Data)
	if err != nil {
		return err
	}

	if lr.N != 0 && dataLength == 0 {
		return fmt.Errorf("compressed packet error: %d unread bytes left in frame", lr.N)
	}

	return nil
}
