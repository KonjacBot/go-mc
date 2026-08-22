package chat

import (
	"bytes"
	"errors"
	"io"
	"strconv"

	"github.com/KonjacBot/go-mc/nbt"
	pk "github.com/KonjacBot/go-mc/net/packet"
)

// ReadFrom decode Message in a Text component
func (m *Message) ReadFrom(r io.Reader) (int64, error) {
	return pk.NBT(m).ReadFrom(r)
}

// WriteTo encode Message into a Text component
func (m Message) WriteTo(w io.Writer) (int64, error) {
	return pk.NBT(&m).WriteTo(w)
}

func (m Message) TagType() byte {
	return nbt.TagCompound
}

func (m Message) MarshalNBT(w io.Writer) error {
	var encoded bytes.Buffer
	encoder := nbt.NewEncoder(&encoded)
	encoder.NetworkFormat(true)
	var err error
	if m.Translate != "" {
		err = encoder.Encode(translateMsg(m), "")
	} else {
		err = encoder.Encode(rawMsgStruct(m), "")
	}
	if err != nil {
		return err
	}
	data := encoded.Bytes()
	if len(data) == 0 || data[0] != m.TagType() {
		return errors.New("chat message did not encode as an NBT compound")
	}
	_, err = w.Write(data[1:])
	return err
}

func (m *Message) UnmarshalNBT(tagType byte, r nbt.DecoderReader) error {
	// Re-combine the tagType into the reader, and create a nbt decoder
	tagReader := bytes.NewReader([]byte{tagType})
	decoder := nbt.NewDecoder(io.MultiReader(tagReader, r))
	decoder.NetworkFormat(true) // TagType directlly followed the body

	switch tagType {
	case nbt.TagString:
		_, err := decoder.Decode(&m.Text)
		return err
	case nbt.TagCompound:
		_, err := decoder.Decode((*rawMsgStruct)(m))
		return err
	case nbt.TagList:
		_, err := decoder.Decode(&m.Extra)
		return err
	default:
		return errors.New("unknown chat message type: '" + strconv.FormatUint(uint64(tagType), 16) + "'")
	}
}

func (t *TranslateArgs) UnmarshalNBT(tagType byte, r nbt.DecoderReader) error {
	tagPrefix := []byte{tagType}
	var listType byte
	if tagType == nbt.TagList {
		var err error
		listType, err = r.ReadByte()
		if err != nil {
			return err
		}
		tagPrefix = append(tagPrefix, listType)
	}
	tagReader := bytes.NewReader(tagPrefix)
	decoder := nbt.NewDecoder(io.MultiReader(tagReader, r))
	decoder.NetworkFormat(true) // TagType directlly followed the body

	switch tagType {
	case nbt.TagList:
		// In 26.2, a translation argument can be a primitive (number, boolean,
		// or string) or a component. NBT lists are homogeneous, so inspect the
		// element type before deciding whether to decode the values as Message.
		if listType == nbt.TagList || listType == nbt.TagCompound {
			var value []Message
			if _, err := decoder.Decode(&value); err != nil {
				return err
			}
			for _, v := range value {
				*t = append(*t, v)
			}
			return nil
		}
		switch listType {
		case nbt.TagEnd, nbt.TagByte, nbt.TagShort, nbt.TagInt, nbt.TagLong, nbt.TagFloat, nbt.TagDouble, nbt.TagString:
		default:
			return errors.New("unknown translation arg list type: '" + strconv.FormatUint(uint64(listType), 16) + "'")
		}

		var value []any
		if _, err := decoder.Decode(&value); err != nil {
			return err
		}
		*t = append(*t, value...)
		return nil
	case nbt.TagByteArray:
		var value []int8
		if _, err := decoder.Decode(&value); err != nil {
			return err
		}
		for _, v := range value {
			*t = append(*t, strconv.FormatInt(int64(v), 10))
		}
		return nil
	case nbt.TagIntArray:
		var value []int32
		if _, err := decoder.Decode(&value); err != nil {
			return err
		}
		for _, v := range value {
			*t = append(*t, strconv.FormatInt(int64(v), 10))
		}
		return nil
	case nbt.TagLongArray:
		var value []int64
		if _, err := decoder.Decode(&value); err != nil {
			return err
		}
		for _, v := range value {
			*t = append(*t, strconv.FormatInt(int64(v), 10))
		}
		return nil
	default:
		return errors.New("unknown translation args type: '" + strconv.FormatUint(uint64(tagType), 16) + "'")
	}
}
