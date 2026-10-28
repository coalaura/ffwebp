package tiff

import (
	"encoding/binary"
	"io"
)

const (
	tagCompression         = 259
	tagPhotometric         = 262
	compressionGroup3      = 3
	compressionGroup4      = 4
	photometricBlackIsZero = 1
)

// Inspect only the first IFD, matching Decode. The decoder has already validated it.
func invertFaxPolarity(reader io.ReaderAt) (bool, error) {
	var header [8]byte

	_, err := reader.ReadAt(header[:], 0)
	if err != nil {
		return false, err
	}

	var order binary.ByteOrder = binary.LittleEndian

	if header[0] == 'M' {
		order = binary.BigEndian
	}

	offset := int64(order.Uint32(header[4:]))

	var count [2]byte

	_, err = reader.ReadAt(count[:], offset)
	if err != nil {
		return false, err
	}

	entries := int(order.Uint16(count[:]))

	var (
		entry       [12]byte
		compression uint16
		photometric uint16
	)

	for index := range entries {
		_, err = reader.ReadAt(entry[:], offset+2+int64(index)*12)
		if err != nil {
			return false, err
		}

		// Both fields are single SHORT values stored directly in the directory entry.
		if order.Uint16(entry[2:4]) != 3 || order.Uint32(entry[4:8]) != 1 {
			continue
		}

		switch order.Uint16(entry[:2]) {
		case tagCompression:
			compression = order.Uint16(entry[8:10])
		case tagPhotometric:
			photometric = order.Uint16(entry[8:10])
		}
	}

	fax := compression == compressionGroup3 || compression == compressionGroup4

	return fax && photometric == photometricBlackIsZero, nil
}
