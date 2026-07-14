package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/admk-studio/timetools.io/internal/solar"
	"github.com/admk-studio/timetools.io/internal/tz"
)

// Card renders the single-city view.
//
//	New York — America/New_York
//
//	09:41:07  Tuesday, 14 July 2026
//	UTC-04:00 · EDT · daylight saving time
//
//	sunrise 05:38 · sunset 20:26 · daylight 14h48m
//	clocks go back 1h on Sun, Nov 1 at 02:00
//
//	UTC 13:41 · unix 1784036467
func Card(c City, o Options) string {
	st := styler{on: !o.Plain}
	var b strings.Builder

	line := func(s string) {
		if s != "" {
			b.WriteString("  " + s + "\n")
		}
	}

	b.WriteString("\n")
	title := st.bold(st.cyan(c.Name))
	if c.Zone != c.Name {
		dash := " — "
		if o.Plain {
			dash = " -- "
		}
		title += dash + st.dim(c.Zone)
	}
	line(title)
	b.WriteString("\n")

	line(st.bold(c.Now.Format(o.timeFormat(true))) + "  " + c.Now.Format("Monday, 2 January 2006"))
	_, offset := c.Now.Zone()
	parts := []string{"UTC" + fmtOffset(offset)}
	// Skip the abbreviation when it just repeats the zone, as it does
	// for UTC and fixed offsets.
	if abbrev := zoneAbbrev(c.Now); abbrev != "" && abbrev != c.Zone {
		parts = append(parts, abbrev)
	}
	parts = append(parts, dstStatus(c))
	line(strings.Join(parts, o.dot()))
	b.WriteString("\n")

	extras := false
	if c.Sun != nil {
		line(sunLine(*c.Sun, o, st))
		extras = true
	}
	if c.Next != nil {
		line(transitionLine(*c.Next, c.Now, o, st))
		extras = true
	}
	if extras {
		b.WriteString("\n")
	}

	utc := c.Now.UTC()
	line(st.dim(fmt.Sprintf("UTC %s%sunix %d", utc.Format("15:04"), o.dot(), utc.Unix())))
	b.WriteString("\n")
	return b.String()
}

func dstStatus(c City) string {
	switch {
	case c.Now.IsDST():
		return "daylight saving time"
	case c.Next != nil:
		return "standard time"
	default:
		return "no seasonal clock changes"
	}
}

func sunLine(day solar.Day, o Options, st styler) string {
	switch day.K {
	case solar.MidnightSun:
		return "midnight sun" + o.dot() + "the sun does not set today"
	case solar.PolarNight:
		return "polar night" + o.dot() + "the sun does not rise today"
	}
	f := o.timeFormat(false)
	return strings.Join([]string{
		st.dim("sunrise ") + day.Rise.Format(f),
		st.dim("sunset ") + day.Set.Format(f),
		st.dim("daylight ") + fmtDuration(day.Daylight()),
	}, o.dot())
}

// transitionLine says what happens in wall-clock terms. The instant is
// shown in the offset people are on *before* the change — "clocks go
// forward on Mar 8 at 02:00" is how everyone says it.
func transitionLine(tr tz.Transition, now time.Time, o Options, st styler) string {
	wall := time.Unix(tr.At.Unix(), 0).In(time.FixedZone("", tr.Before))
	direction := "go forward"
	delta := tr.Delta()
	if delta < 0 {
		direction = "go back"
		delta = -delta
	}
	msg := fmt.Sprintf("clocks %s %s on %s",
		direction,
		fmtDuration(delta),
		wall.Format("Mon, Jan 2 at 15:04"),
	)
	// An imminent change is the one thing on the card worth shouting about.
	if until := tr.At.Sub(now); until < 8*24*time.Hour && until > 0 {
		return st.yellow(msg)
	}
	return st.dim(msg)
}
