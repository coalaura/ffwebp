package jpeg

import (
	"bytes"
	"fmt"

	"github.com/coalaura/jpeg/classic"
)

const (
	iccSignature  = "ICC_PROFILE\x00"
	iccHeaderSize = len(iccSignature) + 2
	iccPartSize   = 65533 - iccHeaderSize
)

func readICC(markers []classic.Marker) ([]byte, error) {
	var (
		parts [255][]byte
		count int
		total int
		found int
	)

	for _, marker := range markers {
		if marker.ID != 0xe2 || !bytes.HasPrefix(marker.Data, []byte(iccSignature)) {
			continue
		}

		if len(marker.Data) <= iccHeaderSize {
			return nil, fmt.Errorf("jpeg: truncated ICC_PROFILE APP2 segment")
		}

		sequence := int(marker.Data[len(iccSignature)])
		declared := int(marker.Data[len(iccSignature)+1])

		if sequence == 0 || declared == 0 || sequence > declared {
			return nil, fmt.Errorf("jpeg: invalid ICC_PROFILE segment sequence %d/%d", sequence, declared)
		}

		if count != 0 && count != declared {
			return nil, fmt.Errorf("jpeg: contradictory ICC_PROFILE segment counts %d and %d", count, declared)
		}

		if parts[sequence-1] != nil {
			return nil, fmt.Errorf("jpeg: duplicate ICC_PROFILE segment %d", sequence)
		}

		count = declared
		parts[sequence-1] = marker.Data[iccHeaderSize:]
		total += len(parts[sequence-1])
		found++
	}

	if found != count {
		return nil, fmt.Errorf("jpeg: incomplete ICC_PROFILE segment set: got %d of %d", found, count)
	}

	if count == 0 {
		return nil, nil
	}

	profile := make([]byte, 0, total)

	for _, part := range parts[:count] {
		profile = append(profile, part...)
	}

	return profile, nil
}

func profileMarkers(profile []byte) ([]classic.Marker, error) {
	if len(profile) == 0 {
		return nil, nil
	}

	count := (len(profile)-1)/iccPartSize + 1
	if count > 255 || len(profile) > (4<<20)-count*(iccHeaderSize+3) {
		return nil, fmt.Errorf("jpeg: ICC profile exceeds the classic backend's 4 MiB metadata limit; use PNG or JPEG XL output to preserve it")
	}

	markers := make([]classic.Marker, count)

	for index := range markers {
		length := min(len(profile), iccPartSize)
		data := make([]byte, iccHeaderSize+length)

		copy(data, iccSignature)
		data[len(iccSignature)] = byte(index + 1)
		data[len(iccSignature)+1] = byte(count)
		copy(data[iccHeaderSize:], profile[:length])

		markers[index] = classic.Marker{ID: 0xe2, Data: data}

		profile = profile[length:]
	}

	return markers, nil
}
