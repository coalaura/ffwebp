package ico

import "encoding/binary"

// Windows uses the alpha channel of 32-bit icons/cursors when it is populated.
// go-ico v1.0.0 also applies the legacy AND mask, erasing partially opaque pixels.
// Clear only those redundant masks in our owned input buffer before decoding.
func preferAlpha(data []byte) {
	if len(data) < 6 {
		return
	}

	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count > (len(data)-6)/16 {
		return
	}

	for index := range count {
		entry := data[6+index*16 : 22+index*16]

		offset := uint64(binary.LittleEndian.Uint32(entry[12:16]))
		size := uint64(binary.LittleEndian.Uint32(entry[8:12]))

		if offset < uint64(6+16*count) || size < 40 || offset+size > uint64(len(data)) {
			continue
		}

		preferBitmapAlpha(data[offset : offset+size])
	}
}

func preferBitmapAlpha(bitmap []byte) {
	headerSize := uint64(binary.LittleEndian.Uint32(bitmap[:4]))
	bits := binary.LittleEndian.Uint16(bitmap[14:16])
	compression := binary.LittleEndian.Uint32(bitmap[16:20])
	paletteSize := binary.LittleEndian.Uint32(bitmap[32:36])

	if headerSize != 40 || bits != 32 || compression != 0 || paletteSize != 0 {
		return
	}

	width := int64(int32(binary.LittleEndian.Uint32(bitmap[4:8])))

	height := int64(int32(binary.LittleEndian.Uint32(bitmap[8:12])))
	if height < 0 {
		height = -height
	}

	if width <= 0 || height == 0 || height%2 != 0 {
		return
	}

	height /= 2
	pixelBytes := uint64(width) * uint64(height) * 4

	maskBytes := uint64((width+31)/32*4) * uint64(height)
	maskStart := headerSize + pixelBytes

	if maskStart+maskBytes > uint64(len(bitmap)) {
		return
	}

	for offset := headerSize + 3; offset < maskStart; offset += 4 {
		if bitmap[offset] != 0 {
			clear(bitmap[maskStart : maskStart+maskBytes])

			return
		}
	}
}
