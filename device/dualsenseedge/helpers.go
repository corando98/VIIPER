package dualsenseedge

import "math"

func GyroDpsToRaw(dps float64) int16 {
	return clampI16(math.Round(dps * GyroCountsPerDps))
}

func GyroRawToDps(raw int16) float64 {
	return float64(raw) / GyroCountsPerDps
}

func AccelMS2ToRaw(ms2 float64) int16 {
	return clampI16(math.Round(ms2 * AccelCountsPerMS2))
}

func AccelRawToMS2(raw int16) float64 {
	return float64(raw) / AccelCountsPerMS2
}

func DefaultAccelRaw() (x, y, z int16) {
	return DefaultAccelXRaw, DefaultAccelYRaw, DefaultAccelZRaw
}

func clampI16(v float64) int16 {
	if v > math.MaxInt16 {
		return math.MaxInt16
	}
	if v < math.MinInt16 {
		return math.MinInt16
	}
	return int16(v)
}

func encodeTouchCoords(b []byte, x, y uint16) {
	if x > TouchpadMaxX {
		x = TouchpadMaxX
	}
	if y > TouchpadMaxY {
		y = TouchpadMaxY
	}
	b[0] = uint8(x & 0xFF)
	b[1] = uint8((x>>8)&0x0F) | uint8((y&0x0F)<<4)
	b[2] = uint8(y >> 4)
}
