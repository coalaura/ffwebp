package png

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
)

type timingWriter struct {
	writer    io.Writer
	delays    []codec.FrameDelay
	chunk     [38]byte
	size      int
	frame     int
	remaining uint64
}

func (writer *timingWriter) Write(data []byte) (int, error) {
	count := len(data)

	for len(data) > 0 {
		if writer.remaining > 0 {
			length := int(min(uint64(len(data)), writer.remaining))

			err := writer.forward(data[:length])
			if err != nil {
				return 0, err
			}

			writer.remaining -= uint64(length)
			data = data[length:]

			continue
		}

		length := 8

		if writer.size >= 8 && string(writer.chunk[4:8]) == "fcTL" {
			length = len(writer.chunk)
		}

		copied := copy(writer.chunk[writer.size:length], data)
		writer.size += copied
		data = data[copied:]

		if writer.size < length {
			continue
		}

		if length == 8 && string(writer.chunk[4:8]) == "fcTL" {
			if binary.BigEndian.Uint32(writer.chunk[:4]) != 26 || writer.frame >= len(writer.delays) {
				return 0, fmt.Errorf("png: unexpected encoded frame control")
			}

			continue
		}

		if length == 38 {
			delay := writer.delays[writer.frame]

			binary.BigEndian.PutUint16(writer.chunk[28:30], uint16(delay.Numerator))
			binary.BigEndian.PutUint16(writer.chunk[30:32], uint16(delay.Denominator))
			binary.BigEndian.PutUint32(writer.chunk[34:], crc32.ChecksumIEEE(writer.chunk[4:34]))

			writer.frame++
		} else {
			writer.remaining = uint64(binary.BigEndian.Uint32(writer.chunk[:4])) + 4
		}

		err := writer.forward(writer.chunk[:length])
		if err != nil {
			return 0, err
		}

		writer.size = 0
	}

	return count, nil
}

func (writer *timingWriter) forward(data []byte) error {
	count, err := writer.writer.Write(data)
	if err == nil && count != len(data) {
		return io.ErrShortWrite
	}

	return err
}

func (writer *timingWriter) finish() error {
	if writer.frame != len(writer.delays) || writer.size != 0 || writer.remaining != 0 {
		return fmt.Errorf("png: incomplete encoded animation timing")
	}

	return nil
}
