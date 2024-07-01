package internal

import (
	"time"
)

type (
	Clock interface {
		Now() time.Time
	}

	UTCClock struct{}

	FixedClock struct {
		now time.Time
	}
)

func (clk *UTCClock) Now() time.Time {
	return time.Now().UTC()
}

func NewFixedClock(now time.Time) FixedClock {
	return FixedClock{now: now}
}

func (clk FixedClock) Now() time.Time {
	return clk.now
}
