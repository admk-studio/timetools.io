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
			// Columns are civil hours, not elapsed hours since midnight.
			instant := time.Date(y, m, d, h, 0, 0, 0, anchor.Now.Location())
			if instant.Hour() != h || instant.Minute() != 0 {
				cells.WriteString("-") // this hour was skipped by a clock change
				continue
			}
			local := instant.In(c.Now.Location())
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
	end := midnight.AddDate(0, 0, 1)
	shared := []workInterval{{midnight, end}}
	for _, city := range cities {
		var next []workInterval
		for _, hours := range workingIntervals(midnight, end, city.Now.Location()) {
			for _, current := range shared {
				start, finish := hours.start, hours.end
				if current.start.After(start) {
					start = current.start
				}
				if current.end.Before(finish) {
					finish = current.end
				}
				if start.Before(finish) {
					next = append(next, workInterval{start, finish})
				}
			}
		}
		shared = next
	}
	if len(shared) == 0 {
		return "no shared working hours"
	}
	var ranges []string
	for _, interval := range shared {
		start := interval.start.In(midnight.Location()).Format("15:04")
		finish := interval.end.In(midnight.Location()).Format("15:04")
		if interval.end.Equal(end) {
			finish = "24:00"
		}
		ranges = append(ranges, start+"-"+finish)
	}
	return fmt.Sprintf("everyone is at work %s, %s time", strings.Join(ranges, " and "), cities[0].Name)
}

type workInterval struct{ start, end time.Time }

// Construct each local day's working interval using calendar times. This
// preserves fractional offsets and accounts for clock changes during the day.
func workingIntervals(start, end time.Time, loc *time.Location) []workInterval {
	y, m, d := start.In(loc).Date()
	ey, em, ed := end.In(loc).Date()
	// Iterate calendar dates in UTC so even zones that skip a midnight advance.
	date := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	last := time.Date(ey, em, ed, 0, 0, 0, 0, time.UTC)
	var intervals []workInterval
	for ; !date.After(last); date = date.AddDate(0, 0, 1) {
		y, m, d := date.Date()
		intervals = append(intervals, workInterval{
			time.Date(y, m, d, workStart, 0, 0, 0, loc),
			time.Date(y, m, d, workEnd, 0, 0, 0, loc),
		})
	}
	return intervals
}
