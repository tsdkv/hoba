package pbt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoinFlipPrecision(t *testing.T) {
	const seeds = 10000
	for _, pct := range []int{0, 7, 25, 50, 75, 93, 100} {
		gen := CoinFlip(pct)
		heads := 0

		for seed := 0; seed < seeds; seed++ {
			if gen.Example(seed) {
				heads++
			}
		}

		assert.InDelta(t, pct, 100*float64(heads)/seeds, 1.5, "pct=%d", pct)
	}
}

func TestDieRollUniform(t *testing.T) {
	const seeds = 10000
	gen := dieRoll(3)
	counts := [3]int{}

	for seed := 0; seed < seeds; seed++ {
		v := gen.Example(seed)
		if assert.GreaterOrEqual(t, v, 0) && assert.Less(t, v, 3) {
			counts[v]++
		}
	}

	for _, c := range counts {
		assert.InDelta(t, float64(seeds)/3, float64(c), 150, "counts=%v", counts)
	}
}
