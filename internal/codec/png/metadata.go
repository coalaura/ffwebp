package png

import (
	"encoding/binary"
	"io"

	"github.com/coalaura/ffwebp/internal/codec"
)

type metadataReader struct {
	reader      io.Reader
	header      [8]byte
	headerSize  int
	payload     [26]byte
	payloadSize int
	remaining   uint64
	done        bool
	animated    bool
	channels    int
	delays      []codec.FrameDelay
}

func (reader *metadataReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	reader.observe(buffer[:count])

	return count, err
}

func (reader *metadataReader) observe(data []byte) {
	// Keep only a chunk header and the metadata prefix, never compressed pixels.
	// pngn validates chunk lengths, order and CRCs before these values are used.
	for len(data) > 0 && !reader.done {
		if reader.remaining > 0 {
			length := int(min(uint64(len(data)), reader.remaining))

			switch string(reader.header[4:]) {
			case "IHDR", "fcTL":
				reader.payloadSize += copy(reader.payload[reader.payloadSize:], data[:length])
			}

			reader.remaining -= uint64(length)
			data = data[length:]

			if reader.remaining == 0 {
				reader.finishChunk()
				reader.headerSize = 0
			}

			continue
		}

		length := copy(reader.header[reader.headerSize:], data)
		reader.headerSize += length
		data = data[length:]

		if reader.headerSize == len(reader.header) {
			reader.remaining = uint64(binary.BigEndian.Uint32(reader.header[:4])) + 4
			reader.payloadSize = 0
		}
	}
}

func (reader *metadataReader) finishChunk() {
	switch string(reader.header[4:]) {
	case "IHDR":
		switch reader.payload[9] {
		case 0:
			reader.channels = 1
		case 4:
			reader.channels = 2
		}
	case "acTL":
		reader.animated = true
	case "fcTL":
		if binary.BigEndian.Uint32(reader.header[:4]) != 26 {
			return
		}

		delay := codec.FrameDelay{
			Numerator:   uint32(binary.BigEndian.Uint16(reader.payload[20:22])),
			Denominator: uint32(binary.BigEndian.Uint16(reader.payload[22:24])),
		}
		if delay.Denominator == 0 {
			delay.Denominator = 100
		}

		reader.delays = append(reader.delays, delay)
	case "IEND":
		reader.done = true
	}
}
