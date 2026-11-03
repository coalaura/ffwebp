package codec

import (
	"fmt"
	"math"
	"math/big"
	"time"
)

func (animation *Animation) fractionFrameAt(target time.Duration) int {
	// Arbitrary APNG denominators need not have a machine-sized common multiple.
	// Reuse the rational scratch space across frames rather than rounding to nanoseconds.
	var (
		elapsed  big.Rat
		delay    big.Rat
		position big.Rat
	)

	position.SetFrac64(int64(target), int64(time.Second))

	for index, fraction := range animation.DelayFractions {
		if fraction.Denominator == 0 {
			return len(animation.Frames) - 1
		}

		delay.SetFrac64(int64(fraction.Numerator), int64(fraction.Denominator))
		elapsed.Add(&elapsed, &delay)

		if elapsed.Cmp(&position) > 0 {
			return index
		}
	}

	return len(animation.Frames) - 1
}

func (animation *Animation) fractionDelaysIn(units uint64) ([]int, error) {
	if units == 0 || len(animation.DelayFractions) != len(animation.Frames) {
		return nil, fmt.Errorf("invalid animation timing")
	}

	delays := make([]int, len(animation.Frames))
	previous := 0

	var (
		elapsed   big.Rat
		delay     big.Rat
		numerator big.Int
		boundary  big.Int
		remainder big.Int
		scale     big.Int
	)

	scale.SetUint64(units)

	for index, fraction := range animation.DelayFractions {
		if fraction.Denominator == 0 {
			return nil, fmt.Errorf("frame %d has an invalid delay denominator", index)
		}

		delay.SetFrac64(int64(fraction.Numerator), int64(fraction.Denominator))

		elapsed.Add(&elapsed, &delay)
		numerator.Mul(elapsed.Num(), &scale)
		boundary.QuoRem(&numerator, elapsed.Denom(), &remainder)
		remainder.Lsh(&remainder, 1)

		if remainder.Cmp(elapsed.Denom()) >= 0 {
			remainder.SetInt64(1)
			boundary.Add(&boundary, &remainder)
		}

		if !boundary.IsInt64() || boundary.Int64() > int64(math.MaxInt) {
			return nil, fmt.Errorf("animation duration exceeds the destination timeline")
		}

		current := int(boundary.Int64())
		if fraction.Numerator > 0 && current == previous {
			return nil, fmt.Errorf("visible frame %d cannot be represented at %d timing units per second; use APNG or JPEG XL output", index, units)
		}

		delays[index] = current - previous
		previous = current
	}

	return delays, nil
}
