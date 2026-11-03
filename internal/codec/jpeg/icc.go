package jpeg

import (
	"fmt"

	"github.com/coalaura/jpeg/classic"
)

const (
	iccSignature  = "ICC_PROFILE\x00"
	iccHeaderSize = len(iccSignature) + 2
	iccPartSize   = 65533 - iccHeaderSize
)

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
