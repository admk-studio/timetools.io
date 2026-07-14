package render

import (
	"fmt"
	"strings"
	"time"
)

// fmtOffset renders seconds east of UTC as "+09:00" or "-04:30".
func fmtOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	return fmt.Sprintf("%s%02d:%02d", sign, seconds/3600, seconds%3600/60)
}

// fmtDelta renders a clock jump as "1h", "30m" or "1h30m", with sign.
func fmtDelta(d time.Duration) string {
	sign := "+"
	if d < 0 {
		sign = "-"
		d = -d
	}
	return sign + fmtDuration(d)
}

// fmtDuration renders a positive duration as "14h23m", dropping zero
// parts: "14h", "45m".
func fmtDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dh%02dm", h, m)
	}
}

// zoneAbbrev returns the zone's letter abbreviation (EST, JST), or ""
// when the zone only has a numeric name like "+0530" or "-03".
func zoneAbbrev(t time.Time) string {
	name, _ := t.Zone()
	if name == "" || name[0] == '+' || name[0] == '-' {
		return ""
	}
	return name
}

// hourDiff renders the wall-clock difference between two places as seen
// right now: "+13h", "-4h30m", "same time".
func hourDiff(from, to time.Time) string {
	_, offFrom := from.Zone()
	_, offTo := to.Zone()
	d := time.Duration(offTo-offFrom) * time.Second
	if d == 0 {
		return "same time"
	}
	return fmtDelta(d)
}

// pad right-pads s with spaces to width (measured in runes, which is
// enough for the ASCII-ish names we render).
func pad(s string, width int) string {
	if n := width - len([]rune(s)); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
