package tz

import "time"

// Transition is a UTC-offset change in some zone: a DST switch, or the
// rare permanent offset change a government decrees.
type Transition struct {
	At     time.Time // first instant of the new offset
	Before int       // seconds east of UTC, before the change
	After  int       // seconds east of UTC, after the change
}

// Delta is how far the clocks jump: +1h for a spring-forward, -1h for a
// fall-back.
func (t Transition) Delta() time.Duration {
	return time.Duration(t.After-t.Before) * time.Second
}

// nextTransitionHorizon is how far ahead we look. 15 months covers a full
// DST cycle with room to spare; zones with no change inside it are
// reported as stable.
const nextTransitionHorizon = 456 * 24 * time.Hour

// NextTransition finds the next offset change in loc after 'from'. The Go
// runtime doesn't expose the zone's transition table, so we probe: step
// forward in 6-hour jumps until the offset differs, then binary-search
// the exact second. Transitions are months apart, so a 6-hour step can
// never skip over two of them.
func NextTransition(loc *time.Location, from time.Time) (Transition, bool) {
	const step = 6 * time.Hour

	offsetAt := func(unix int64) int {
		_, off := time.Unix(unix, 0).In(loc).Zone()
		return off
	}

	start := from.Unix()
	end := from.Add(nextTransitionHorizon).Unix()
	before := offsetAt(start)

	stepSecs := int64(step / time.Second)
	var lo, hi int64
	found := false
	for t := start + stepSecs; t <= end; t += stepSecs {
		if offsetAt(t) != before {
			lo, hi = t-stepSecs, t
			found = true
			break
		}
	}
	if !found {
		return Transition{}, false
	}

	// Invariant: offset(lo) == before, offset(hi) != before. Narrow to
	// the exact second the new offset takes effect.
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if offsetAt(mid) != before {
			hi = mid
		} else {
			lo = mid
		}
	}
	return Transition{
		At:     time.Unix(hi, 0).In(loc),
		Before: before,
		After:  offsetAt(hi),
	}, true
}
