package controllers

import (
	"math"
	"math/rand/v2"
	"time"
)

const percentageScale = 100

func jitterInterval(interval time.Duration, percentage uint) time.Duration {
	if interval <= 0 || percentage == 0 {
		return interval
	}

	//nolint:gosec // Polling jitter does not require cryptographic randomness.
	offset := time.Duration(float64(interval) * (float64(percentage) / percentageScale) * (2*rand.Float64() - 1))
	if offset > 0 && interval > time.Duration(math.MaxInt64)-offset {
		return time.Duration(math.MaxInt64)
	}

	return interval + offset
}
