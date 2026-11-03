package jpegxl

import (
	"fmt"
	"math"

	"github.com/coalaura/ffwebp/internal/codec"
)

func exactTiming(delays []codec.FrameDelay) (uint32, uint32, []uint32, error) {
	numerator := uint64(1)
	denominator := uint64(0)

	// The greatest common divisor of reduced fractions is gcd(numerators)/lcm(denominators).
	// Using that tick length minimizes both the clock fields and every frame duration.
	for index, delay := range delays {
		if delay.Denominator == 0 {
			return 0, 0, nil, fmt.Errorf("jpegxl: frame %d has an invalid delay denominator", index)
		}

		if delay.Numerator == 0 {
			continue
		}

		divisor := gcd(uint64(delay.Numerator), uint64(delay.Denominator))
		frameNumerator := uint64(delay.Numerator) / divisor
		frameDenominator := uint64(delay.Denominator) / divisor
		factor := frameDenominator / gcd(numerator, frameDenominator)

		if numerator > math.MaxUint32/factor {
			return 0, 0, nil, timingRangeError()
		}

		numerator *= factor
		denominator = gcd(denominator, frameNumerator)
	}

	if denominator == 0 {
		denominator = 1
	}

	if denominator > math.MaxUint32 {
		return 0, 0, nil, timingRangeError()
	}

	durations := make([]uint32, len(delays))

	for index, delay := range delays {
		if delay.Numerator == 0 {
			continue
		}

		divisor := gcd(uint64(delay.Numerator), uint64(delay.Denominator))
		frameNumerator := uint64(delay.Numerator) / divisor
		frameDenominator := uint64(delay.Denominator) / divisor

		factor := numerator / frameDenominator
		ticks := frameNumerator / denominator

		if ticks > math.MaxUint32/factor {
			return 0, 0, nil, timingRangeError()
		}

		durations[index] = uint32(ticks * factor)
	}

	return uint32(numerator), uint32(denominator), durations, nil
}

func gcd(first, second uint64) uint64 {
	for second != 0 {
		remainder := first % second
		first = second
		second = remainder
	}

	return first
}

func timingRangeError() error {
	return fmt.Errorf("jpegxl: exact frame timing exceeds the 32-bit clock or duration fields; keep APNG output or explicitly retime the animation before converting")
}
