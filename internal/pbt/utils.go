package pbt

import "pgregory.net/rapid"

var tenBits = rapid.SliceOfN(rapid.Bool(), 10, 10)

func foldBits(bs []bool) int {
	v := 0
	for _, b := range bs {
		v <<= 1
		if b {
			v |= 1
		}
	}
	return v
}

// CoinFlip reports true with pct% probability (±0.05pp). Bool is the only
// unbiased primitive rapid has, so pct is quantized into 10 fair bits.
// Shrinks toward true.
func CoinFlip(pct int) *rapid.Generator[bool] {
	thr := (pct*(1<<10) + 50) / 100
	return rapid.Map(tenBits, func(bs []bool) bool { return foldBits(bs) < thr })
}

// dieRoll draws 0..n-1 without IntRange's bias toward 0; 1024 mod n < n
// keeps the skew under 0.1pp for small n.
func dieRoll(n int) *rapid.Generator[int] {
	return rapid.Map(tenBits, func(bs []bool) int { return foldBits(bs) % n })
}
