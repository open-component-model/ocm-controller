package controllers

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJitterInterval(t *testing.T) {
	for _, tt := range []struct {
		name       string
		interval   time.Duration
		percentage uint
		minimum    time.Duration
		maximum    time.Duration
	}{
		{name: "disabled", interval: time.Minute, minimum: time.Minute, maximum: time.Minute},
		{name: "five percent", interval: time.Minute, percentage: 5, minimum: 57 * time.Second, maximum: 63 * time.Second},
		{name: "largest percentage", interval: time.Second, percentage: 99, minimum: 10 * time.Millisecond, maximum: 1990 * time.Millisecond},
		{name: "zero interval", percentage: 5},
		{name: "negative interval", interval: -time.Second, percentage: 5, minimum: -time.Second, maximum: -time.Second},
		{name: "smallest interval", interval: time.Nanosecond, percentage: 99, minimum: time.Nanosecond, maximum: time.Nanosecond},
		{name: "largest interval", interval: time.Duration(math.MaxInt64), percentage: 99, minimum: 1, maximum: time.Duration(math.MaxInt64)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for range 100 {
				actual := jitterInterval(tt.interval, tt.percentage)
				require.GreaterOrEqual(t, actual, tt.minimum)
				require.LessOrEqual(t, actual, tt.maximum)
			}
		})
	}
}

func TestJitterIntervalConcurrent(t *testing.T) {
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			for range 100 {
				actual := jitterInterval(time.Minute, 5)
				if actual < 57*time.Second || actual > 63*time.Second {
					t.Errorf("jittered interval out of range: %s", actual)
				}
			}
		})
	}
	workers.Wait()
}

func TestJitterIntervalSpreadsPolls(t *testing.T) {
	var earlier, later bool
	for range 128 {
		actual := jitterInterval(time.Minute, 5)
		earlier = earlier || actual < time.Minute
		later = later || actual > time.Minute
	}
	require.True(t, earlier, "jitter should schedule some polls before the base interval")
	require.True(t, later, "jitter should schedule some polls after the base interval")
}
