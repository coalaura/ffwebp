package codec

import (
	"bytes"
	"encoding/binary"
	"math"
)

func EquivalentICC(first, second []byte) bool {
	return bytes.Equal(first, second) || StandardSRGB(first) && StandardSRGB(second)
}

// StandardSRGB recognizes matrix/TRC sRGB (or its neutral gray axis), not
// descriptions or the RGB header alone. Unknown/LUT profiles remain profiled.
func StandardSRGB(profile []byte) bool {
	if len(profile) == 0 {
		return true
	}

	if len(profile) < 132 || string(profile[36:40]) != "acsp" || string(profile[20:24]) != "XYZ " || int64(binary.BigEndian.Uint32(profile)) != int64(len(profile)) {
		return false
	}

	count := int64(binary.BigEndian.Uint32(profile[128:]))
	if count > int64((len(profile)-132)/12) {
		return false
	}

	for index := range int(count) {
		entry := profile[132+index*12:]

		signature := string(entry[:4])
		if signature[:3] == "A2B" || signature[:3] == "B2A" || signature[:3] == "D2B" || signature[:3] == "B2D" {
			return false
		}

		offset := uint64(binary.BigEndian.Uint32(entry[4:]))
		size := uint64(binary.BigEndian.Uint32(entry[8:]))

		if offset < uint64(132+count*12) || offset+size > uint64(len(profile)) {
			return false
		}
	}

	white := iccTag(profile, "wtpt")
	d50 := iccXYZ(white, [3]float64{0.9642, 1, 0.8249})

	if string(profile[16:20]) == "GRAY" {
		// libjxl emits a D65 gray white point; ICC gray profiles can also use
		// the adapted D50 neutral axis. Both share sRGB's neutral samples.
		d65 := iccXYZ(white, [3]float64{0.95046, 1, 1.08906})
		return (d50 || d65) && srgbTRC(iccTag(profile, "kTRC"))
	}

	if !d50 || string(profile[16:20]) != "RGB " {
		return false
	}

	red := iccXYZ(iccTag(profile, "rXYZ"), [3]float64{0.43607, 0.22250, 0.01393})
	green := iccXYZ(iccTag(profile, "gXYZ"), [3]float64{0.38515, 0.71687, 0.09708})
	blue := iccXYZ(iccTag(profile, "bXYZ"), [3]float64{0.14307, 0.06063, 0.71410})
	curves := srgbTRC(iccTag(profile, "rTRC")) && srgbTRC(iccTag(profile, "gTRC")) && srgbTRC(iccTag(profile, "bTRC"))

	return red && green && blue && curves
}

func iccTag(profile []byte, signature string) []byte {
	count := int(binary.BigEndian.Uint32(profile[128:]))

	var found []byte

	for index := range count {
		entry := profile[132+index*12:]
		if string(entry[:4]) != signature {
			continue
		}

		if found != nil {
			return nil
		}

		offset := int(binary.BigEndian.Uint32(entry[4:]))
		size := int(binary.BigEndian.Uint32(entry[8:]))
		found = profile[offset : offset+size]
	}

	return found
}

func iccFixed(data []byte) float64 {
	return float64(int32(binary.BigEndian.Uint32(data))) / 65536
}

func iccXYZ(tag []byte, expected [3]float64) bool {
	if len(tag) != 20 || string(tag[:4]) != "XYZ " {
		return false
	}

	for index, value := range expected {
		if math.Abs(iccFixed(tag[8+index*4:])-value) > 0.0005 {
			return false
		}
	}

	return true
}

func srgbTRC(tag []byte) bool {
	if len(tag) < 12 {
		return false
	}

	var (
		parameters [7]float64
		count      int
	)

	function := -1

	switch string(tag[:4]) {
	case "para":
		function = int(binary.BigEndian.Uint16(tag[8:]))
		lengths := [...]int{1, 3, 4, 5, 7}

		if function >= len(lengths) || len(tag) != 12+lengths[function]*4 {
			return false
		}

		for index := range lengths[function] {
			parameters[index] = iccFixed(tag[12+index*4:])
		}
	case "curv":
		count = int(binary.BigEndian.Uint32(tag[8:]))
		if count < 2 || int64(count)*2+12 != int64(len(tag)) {
			return false
		}
	default:
		return false
	}

	for index := range 1025 {
		input := float64(index) / 1024
		expected := input / 12.92

		if input > 0.04045 {
			expected = math.Pow((input+0.055)/1.055, 2.4)
		}

		var actual float64

		if function < 0 {
			position := input * float64(count-1)
			lower := min(int(position), count-2)
			first := float64(binary.BigEndian.Uint16(tag[12+lower*2:])) / 65535
			second := float64(binary.BigEndian.Uint16(tag[14+lower*2:])) / 65535
			actual = first + (second-first)*(position-float64(lower))
		} else {
			gamma := parameters[0]
			scale := parameters[1]
			offset := parameters[2]

			switch function {
			case 0:
				actual = math.Pow(input, gamma)
			case 1, 2:
				actual = math.Pow(max(0, scale*input+offset), gamma) + parameters[3]
			case 3, 4:
				if input >= parameters[4] {
					actual = math.Pow(scale*input+offset, gamma) + parameters[5]
				} else {
					actual = parameters[3]*input + parameters[6]
				}
			}
		}

		if math.IsNaN(actual) || math.Abs(actual-expected) > 0.0001 {
			return false
		}
	}

	return true
}
