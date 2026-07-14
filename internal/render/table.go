package render

import (
	"fmt"
	"strings"
	"time"
)

// Working hours used by the overlap chart. Opinionated on purpose;
// nobody schedules by their real calendar from a curl output.
const (
	workStart = 9  // inclusive
	workEnd   = 18 // exclusive
)

// Table renders the multi-city comparison: one row per city, then a
// working-hours chart aligned to the first city's day. The first city is
// the reference for date and hour differences.
//
//	New York   09:41   Tue, Jul 14   UTC-04:00
//	London     14:41   Tue, Jul 14   UTC+01:00   +5h
//	Tokyo      22:41   Wed, Jul 15   UTC+09:00   +13h
//
//	           0  3  6  9  12 15 18 21
//	New York   ·········█████████······
//	London     ····█████████···········
func Table(cities []City, o Options) string {
	st := styler{on: !o.Plain}
	anchor := cities[0]

	nameW := 0
	for _, c := range cities {
		if n := len([]rune(c.Name)); n > nameW {
			nameW = n
		}
	}
	nameW += 3 // gutter between name and first column

	var b strings.Builder
	b.WriteString("\n")

	timeF := o.timeFormat(false)
	for _, c := range cities {
		row := "  " + st.bold(st.cyan(pad(c.Name, nameW)))
		row += st.bold(pad(c.Now.Format(timeF), 10))
		row += pad(dateCell(c, anchor, o), 16)
		_, off := c.Now.Zone()
		row += pad("UTC"+fmtOffset(off), 12)
		if !c.Now.Equal(anchor.Now) || c.Zone != anchor.Zone {
			row += st.dim(hourDiff(anchor.Now, c.Now))
		}
		b.WriteString(strings.TrimRight(row, " ") + "\n")
	}

	b.WriteString("\n")
	writeChart(&b, cities, o, st, nameW)
	b.WriteString("\n")
	return b.String()
}

// dateCell shows the city's date, flagging cities that are already on a
// different calendar day than the reference city.
func dateCell(c, anchor City, o Options) string {
	cell := c.Now.Format("Mon, Jan 2")
	switch a, b := dayOrdinal(anchor.Now), dayOrdinal(c.Now); {
	case b > a:
		cell += " +1d"
	case b < a:
		cell += " -1d"
	}
	return cell
}

func dayOrdinal(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

// writeChart draws each city's working hours on the first city's
// timeline, so the overlap is visible at a glance. The current hour is
// marked in every row.
func writeChart(b *strings.Builder, cities []City, o Options, st styler, nameW int) {
	anchor := cities[0]
	y, m, d := anchor.Now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, anchor.Now.Location())
	nowHour := anchor.Now.Hour()

	work, off, now := "█", "·", "┃"
	if o.Plain {
		work, off, now = "#", ".", "|"
	}

	b.WriteString("  " + st.dim(pad(fmt.Sprintf("working hours, on %s's clock", anchor.Name), 0)) + "\n")
	axis := make([]byte, 24)
	for i := range axis {
		axis[i] = ' '
	}
	for h := 0; h < 24; h += 3 {
		copy(axis[h:], fmt.Sprintf("%d", h))
	}
	b.WriteString("  " + strings.Repeat(" ", nameW) + st.dim(string(axis)) + "\n")

	for _, c := range cities {
		var cells strings.Builder
		for h := 0; h < 24; h++ {
			local := midnight.Add(time.Duration(h) * time.Hour).In(c.Now.Location())
			working := local.Hour() >= workStart && local.Hour() < workEnd
			switch {
			case h == nowHour:
				cells.WriteString(st.yellow(now))
			case working:
				cells.WriteString(st.green(work))
			default:
				cells.WriteString(st.dim(off))
			}
		}
		b.WriteString("  " + st.cyan(pad(c.Name, nameW)) + cells.String() + "\n")
	}

	if line := overlapLine(cities, midnight, o); line != "" {
		b.WriteString("\n  " + st.dim(line) + "\n")
	}
}

// overlapLine reports when every city is inside working hours, in the
// reference city's clock.
func overlapLine(cities []City, midnight time.Time, o Options) string {
	if len(cities) < 2 {
		return ""
	}
	var ranges []string
	start := -1
	for h := 0; h <= 24; h++ {
		all := h < 24
		if all {
			for _, c := range cities {
				lh := midnight.Add(time.Duration(h) * time.Hour).In(c.Now.Location()).Hour()
				if lh < workStart || lh >= workEnd {
					all = false
					break
				}
			}
		}
		switch {
		case all && start == -1:
			start = h
		case !all && start != -1:
			ranges = append(ranges, fmt.Sprintf("%02d:00-%02d:00", start, h))
			start = -1
		}
	}
	if len(ranges) == 0 {
		return "no shared working hours"
	}
	return fmt.Sprintf("everyone is at work %s, %s time", strings.Join(ranges, " and "), cities[0].Name)
}
