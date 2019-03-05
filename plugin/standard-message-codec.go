package plugin

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"

	"github.com/pkg/errors"
)

const (
	standardMessageType_null         = 0
	standardMessageType_true         = 1
	standardMessageType_false        = 2
	standardMessageType_int32        = 3
	standardMessageType_int64        = 4
	standardMessageType_bigint       = 5
	standardMessageType_float64      = 6
	standardMessageType_string       = 7
	standardMessageType_byteSlice    = 8
	standardMessageType_int32Slice   = 9
	standardMessageType_int64Slice   = 10
	standardMessageType_float64Slice = 11
	standardMessageType_list         = 12
	standardMessageType_map          = 13
)

// StandardMessageCodec implements a MessageCodec using the Flutter standard
// binary encoding.
//
// This codec tries to stay compatible with the corresponding
// StandardMessageCodec on the Dart side.
// See: https://docs.flutter.io/flutter/services/StandardMessageCodec-class.html
//
// Supported messages are acyclic values of these forms:
//
//     nil
//     bool
//     byte
//     int32, int64
//     float64
//     *big.Int
//     string
//     []byte, []int32, []int64, []float64
//     []interface{} of supported values
//     map[interface{}]interface{} with supported keys and values
//
// On the Dart side, these values are represented as follows:
//
//     null: null
//     bool: bool
//     byte, int8, int16, int32, int64: int
//     float32, float64: double
//     string: String
//     []byte: Uint8List
//     []int32: Int32List
//     []int64: Int64List
//     []float64: Float64List
//     []interface{}: List
//     map[interface{}]interface{}: Map
//
// *big.Int's are represented in Dart as strings with the
// hexadecimal representation of the integer's value.
//
type StandardMessageCodec struct{}

var _ MessageCodec = StandardMessageCodec{} // compile-time type check

// EncodeMessage encodes message to bytes using the Flutter standard message encoding.
// message is expected to be comprised of supported types. See `type StandardMessageCodec`.
func (s StandardMessageCodec) EncodeMessage(message interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := s.writeValue(&buf, message)
	if err != nil {
		// TODO: Should return MessageTypeError when that was the cause.
		return nil, errors.Wrap(err, "failed to encode message")
	}
	return buf.Bytes(), nil
}

// DecodeMessage decodes binary data into a standard message
func (s StandardMessageCodec) DecodeMessage(data []byte) (message interface{}, err error) {
	buf := bytes.NewBuffer(data)
	message, err = s.readValue(buf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode data to message")
	}
	return message, nil
}

// writeSize writes an int representing a size to the specified stream.
// Uses an expanding code of 1 to 5 bytes to optimize for small values.
func (s StandardMessageCodec) writeSize(buf *bytes.Buffer, value int) error {
	if value < 0 {
		return errors.New("invalid size: negative")
	}
	var err error
	if value < 254 {
		// write as byte
		err = buf.WriteByte(byte(value))
		if err != nil {
			return err
		}
	} else if value <= 0xffff {
		// write as int16
		err = buf.WriteByte(254)
		if err != nil {
			return err
		}
		err = s.writeInt16(buf, int16(value))
		if err != nil {
			return err
		}
	} else {
		// write as int32
		err = buf.WriteByte(255)
		if err != nil {
			return err
		}
		err = s.writeInt32(buf, int32(value))
		if err != nil {
			return err
		}
	}
	return nil
}

// writeInt16 encodes an int16 to the writer.
// 2 bytes. char in Java.
func (s StandardMessageCodec) writeInt16(w io.Writer, value int16) error {
	return binary.Write(w, endian, value)
}

// writeInt32 encodes an int32 to the writer.
// 4 bytes. int in Java.
func (s StandardMessageCodec) writeInt32(w io.Writer, value int32) error {
	return binary.Write(w, endian, value)
}

// writeInt64 encodes an int64 to the writer.
// 8 bytes. long in Java.
func (s StandardMessageCodec) writeInt64(w io.Writer, value int64) error {
	return binary.Write(w, endian, value)
}

// writeFloat64 encodes a float64 to the writer.
// 8 bytes. double in Java.
func (s StandardMessageCodec) writeFloat64(w io.Writer, value float64) error {
	// TODO: binary correct format?
	return binary.Write(w, endian, value)
}

// writeBytes encodes a slice of bytes to the writer.
// First the length is written, then the actual bytes.
func (s StandardMessageCodec) writeBytes(buf *bytes.Buffer, value []byte) error {
	err := s.writeSize(buf, len(value))
	if err != nil {
		return err
	}
	_, err = buf.Write(value)
	if err != nil {
		return err
	}
	return nil
}

// writeString encodes a string to the writer.
// First the length is written, then the actual bytes.
func (s StandardMessageCodec) writeString(buf *bytes.Buffer, value string) error {
	err := s.writeSize(buf, len(value))
	if err != nil {
		return err
	}
	_, err = buf.WriteString(value)
	if err != nil {
		return err
	}
	return nil
}

// writeAlignment writes a number of padding bytes to the specified stream to ensure that
// the next value is aligned to a whole multiple of the specified alignment.
// An example usage with alignment = 8 is to ensure doubles are word-aligned
// in the stream.
func (s StandardMessageCodec) writeAlignment(buf *bytes.Buffer, alignment int) error {
	mod := buf.Len() % alignment
	if mod != 0 {
		_, err := buf.Write(make([]byte, alignment-mod))
		if err != nil {
			return err
		}
	}
	return nil
}

// writeValue writes a type discriminator byte followed by the encoded value.
func (s StandardMessageCodec) writeValue(buf *bytes.Buffer, value interface{}) error {
	if value == nil {
		return buf.WriteByte(standardMessageType_null)
	}

	var err error
	switch typedValue := value.(type) {
	case bool:
		if typedValue {
			return buf.WriteByte(standardMessageType_true)
		}
		return buf.WriteByte(standardMessageType_false)

	case int32:
		err = buf.WriteByte(standardMessageType_int32)
		if err != nil {
			return err
		}
		return s.writeInt32(buf, typedValue)

	case int64:
		err = buf.WriteByte(standardMessageType_int64)
		if err != nil {
			return err
		}
		return s.writeInt64(buf, typedValue)

	case float64:
		err = buf.WriteByte(standardMessageType_float64)
		if err != nil {
			return err
		}
		err = s.writeAlignment(buf, 8) // TODO: add to writeFloat64?
		if err != nil {
			return err
		}
		return s.writeFloat64(buf, typedValue)

	case *big.Int:
		err = buf.WriteByte(standardMessageType_bigint)
		if err != nil {
			return err
		}
		return s.writeString(buf, typedValue.Text(16))

	case string:
		err = buf.WriteByte(standardMessageType_string)
		if err != nil {
			return err
		}
		return s.writeString(buf, typedValue)

	case []byte:
		err = buf.WriteByte(standardMessageType_byteSlice)
		if err != nil {
			return err
		}
		return s.writeBytes(buf, typedValue)

	case []int32:
		err = buf.WriteByte(standardMessageType_int32Slice)
		if err != nil {
			return err
		}
		// TODO: wrap as writeInt32Slice ?
		err = s.writeSize(buf, len(typedValue))
		if err != nil {
			return err
		}
		err = s.writeAlignment(buf, 4)
		if err != nil {
			return err
		}
		return binary.Write(buf, endian, typedValue)

	case []int64:
		err = buf.WriteByte(standardMessageType_int64Slice)
		if err != nil {
			return err
		}
		// TODO: wrap as writeInt64Slice ?
		err = s.writeSize(buf, len(typedValue))
		if err != nil {
			return err
		}
		err = s.writeAlignment(buf, 8)
		if err != nil {
			return err
		}
		return binary.Write(buf, endian, typedValue)

	case []float64:
		err = buf.WriteByte(standardMessageType_float64Slice)
		if err != nil {
			return err
		}
		// TODO: wrap as writeFloat64Slice ?
		err = s.writeSize(buf, len(typedValue))
		if err != nil {
			return err
		}
		err = s.writeAlignment(buf, 8)
		if err != nil {
			return err
		}
		return binary.Write(buf, endian, typedValue)

	case []interface{}:
		err = buf.WriteByte(standardMessageType_list)
		if err != nil {
			return err
		}
		// TODO: wrap as writeList ?
		err = s.writeSize(buf, len(typedValue))
		if err != nil {
			return err
		}
		for _, v := range typedValue {
			err = s.writeValue(buf, v)
			if err != nil {
				return err
			}
		}
		return nil

	case map[interface{}]interface{}:
		err = buf.WriteByte(standardMessageType_map)
		if err != nil {
			return err
		}
		// TODO: wrap as writeMap ?
		err = s.writeSize(buf, len(typedValue))
		if err != nil {
			return err
		}
		for k, v := range typedValue {
			err = s.writeValue(buf, k)
			if err != nil {
				return err
			}
			err = s.writeValue(buf, v)
			if err != nil {
				return err
			}
		}
		return nil

	default:
		return MessageTypeError{fmt.Sprintf("type %T is not supported by StandardMessageCodec", value)}
	}
	// no return statement because each case must return
}

func (s StandardMessageCodec) readSize(buf *bytes.Buffer) (size int, err error) {
	b, err := buf.ReadByte()
	if err != nil {
		return 0, err
	}
	if b < 254 {
		return int(b), nil
	} else if b == 254 {
		v, err := s.readInt16(buf)
		return int(v), err
	}
	v, err := s.readInt32(buf)
	return int(v), err
}

func (s StandardMessageCodec) readInt16(r io.Reader) (value int16, err error) {
	err = binary.Read(r, endian, &value)
	return value, err
}

func (s StandardMessageCodec) readInt32(r io.Reader) (value int32, err error) {
	err = binary.Read(r, endian, &value)
	return value, err
}

func (s StandardMessageCodec) readInt64(r io.Reader) (value int64, err error) {
	err = binary.Read(r, endian, &value)
	return value, err
}

func (s StandardMessageCodec) readFloat64(r io.Reader) (value float64, err error) {
	err = binary.Read(r, endian, &value)
	return value, err
}

func (s StandardMessageCodec) readBytes(buf *bytes.Buffer) (value []byte, err error) {
	length, err := s.readSize(buf)
	if err != nil {
		return nil, errors.Wrap(err, "reading size")
	}
	value = buf.Next(length)
	if len(value) != length {
		return nil, errors.New("message corrupted: not enough bytes in buffer")
	}
	return value, nil
}

func (s StandardMessageCodec) readAlignment(buf *bytes.Buffer, originalSize int, alignment int) {
	position := originalSize - buf.Len()
	mod := position % alignment
	if mod != 0 {
		buf.Next(alignment - mod)
	}
}

func (s StandardMessageCodec) readValue(buf *bytes.Buffer) (value interface{}, err error) {
	originalSize := len(buf.Bytes())
	return s.readValueAligned(buf, originalSize)
}

func (s StandardMessageCodec) readValueAligned(buf *bytes.Buffer, originalSize int) (value interface{}, err error) {
	valueType, err := buf.ReadByte()
	if err != nil {
		return nil, errors.Wrap(err, "reading value type")
	}

	switch valueType {
	case standardMessageType_null:
		return nil, nil

	case standardMessageType_true:
		return true, nil

	case standardMessageType_false:
		return false, nil

	case standardMessageType_int32:
		return s.readInt32(buf)

	case standardMessageType_int64:
		return s.readInt64(buf)

	case standardMessageType_float64:
		s.readAlignment(buf, originalSize, 8)
		return s.readFloat64(buf)

	case standardMessageType_bigint:
		bs, err := s.readBytes(buf)
		if err != nil {
			return nil, errors.Wrap(err, "reading byte slice for bigint")
		}
		bigint, ok := new(big.Int).SetString(string(bs), 16)
		if !ok {
			return nil, errors.New("invalid binary encoding for bigint")
		}
		return bigint, nil

	case standardMessageType_string:
		bs, err := s.readBytes(buf)
		if err != nil {
			return nil, errors.Wrap(err, "reading byte slice for string")
		}
		return string(bs), nil

	case standardMessageType_byteSlice:
		bs, err := s.readBytes(buf)
		if err != nil {
			return nil, errors.Wrap(err, "reading byte slice")
		}
		return bs, nil

	case standardMessageType_int32Slice:
		length, err := s.readSize(buf)
		if err != nil {
			return nil, err
		}
		s.readAlignment(buf, originalSize, 4)
		value := make([]int32, length)
		err = binary.Read(buf, endian, value)
		if err != nil {
			return nil, err
		}
		return value, nil

	case standardMessageType_int64Slice:
		length, err := s.readSize(buf)
		if err != nil {
			return nil, err
		}
		s.readAlignment(buf, originalSize, 8)
		value := make([]int64, length)
		err = binary.Read(buf, endian, value)
		if err != nil {
			return nil, err
		}
		return value, nil

	case standardMessageType_float64Slice:
		length, err := s.readSize(buf)
		if err != nil {
			return nil, err
		}
		s.readAlignment(buf, originalSize, 8)
		value := make([]float64, length)
		err = binary.Read(buf, endian, value)
		if err != nil {
			return nil, err
		}
		return value, nil

	case standardMessageType_list:
		length, err := s.readSize(buf)
		if err != nil {
			return nil, err
		}
		list := make([]interface{}, 0, length)
		for i := 0; i < length; i++ {
			value, err := s.readValueAligned(buf, originalSize)
			if err != nil {
				return nil, err
			}
			list = append(list, value)
		}
		return list, nil

	case standardMessageType_map:
		length, err := s.readSize(buf)
		if err != nil {
			return nil, err
		}
		m := make(map[interface{}]interface{})
		for i := 0; i < length; i++ {
			key, err := s.readValueAligned(buf, originalSize)
			if err != nil {
				return nil, err
			}
			value, err := s.readValueAligned(buf, originalSize)
			if err != nil {
				return nil, err
			}
			m[key] = value
		}
		return m, nil

	default:
		return nil, errors.New("invalid message value type")
	}
}
