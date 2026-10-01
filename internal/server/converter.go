package server

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/admk-studio/timetools.io/internal/tz"
)

type converterData struct {
	From, To, Date, Time, Occurrence string
	Error                            string
	Ambiguous                        bool
	Earlier, Later                   string
	Zones                            []tz.Zone
	Result                           *conversionResult
}

type conversionResult struct {
	From, To                  conversionTime
	Difference, Day, ShareURL string
}

type conversionTime struct {
	Name, Zone, Time, Date, ISO, Offset, Status string
}

// This is a browser tool, including for crawlers and clients without a
// browser user agent. Existing city/API endpoints keep their negotiation.
func (s *Server) handleConverter(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	c := &converterData{
		From: "UTC", To: "America/New_York", Occurrence: "earlier",
		Zones: tz.Zones(),
	}
	for key, field := range map[string]*string{
		"from": &c.From, "to": &c.To, "date": &c.Date,
		"time": &c.Time, "occurrence": &c.Occurrence,
	} {
		if q.Has(key) {
			*field = strings.TrimSpace(q.Get(key))
		}
	}
	d := s.pageBase("Time Zone Converter — "+s.cfg.BaseURL,
		"Convert a date and time between cities and time zones. See the time difference, date changes and daylight saving adjustments, and share your conversion.")
	d.Converter = c
	fail := func(status int, message string) {
		c.Error = message
		s.renderPageStatus(w, status, "converter", d)
	}
	if c.Occurrence != "earlier" && c.Occurrence != "later" {
		fail(http.StatusBadRequest, "Choose the first or second occurrence of the repeated time.")
		return
	}
	from, err := tz.Resolve(c.From)
	if err != nil {
		fail(http.StatusBadRequest, "Choose a valid starting city or time zone, such as London or Europe/London.")
		return
	}
	to, err := tz.Resolve(c.To)
	if err != nil {
		fail(http.StatusBadRequest, "Choose a valid destination city or time zone, such as Tokyo or Asia/Tokyo.")
		return
	}
	fromLoc, err := from.Location()
	if err != nil {
		fail(http.StatusInternalServerError, "The starting time zone is unavailable. Please try another city.")
		return
	}
	toLoc, err := to.Location()
	if err != nil {
		fail(http.StatusInternalServerError, "The destination time zone is unavailable. Please try another city.")
		return
	}
	var instant time.Time
	if q.Get("action") == "now" || (!q.Has("date") && !q.Has("time")) {
		instant = s.now().Truncate(time.Minute)
	} else {
		wall, err := parseConverterTime(c.Date, c.Time)
		if err != nil {
			fail(http.StatusBadRequest, "Enter a valid date and time (years 0001–9999).")
			return
		}
		matches := wallTimeMatches(wall, fromLoc)
		if len(matches) == 0 {
			fail(http.StatusBadRequest, fmt.Sprintf("%s at %s does not exist in %s because the clocks move forward. Choose a time before or after the clock change.", c.Date, c.Time, from.Display))
			return
		}
		instant = matches[0]
		if c.Occurrence == "later" {
			instant = matches[len(matches)-1]
		}
	}
	if q.Get("action") == "swap" {
		from, to = to, from
		fromLoc, toLoc = toLoc, fromLoc
		c.From, c.To = c.To, c.From
	}
	source, target := instant.In(fromLoc), instant.In(toLoc)
	if source.Year() < 1 || source.Year() > 9999 || target.Year() < 1 || target.Year() > 9999 {
		fail(http.StatusBadRequest, "This conversion falls outside the supported years 0001–9999. Choose another date.")
		return
	}
	c.Date, c.Time = source.Format("2006-01-02"), converterClock(source)
	// Re-evaluate after Now or Swap so the selected occurrence always
	// preserves the same instant, even when the new source repeats an hour.
	wall, _ := parseConverterTime(c.Date, c.Time)
	matches := wallTimeMatches(wall, fromLoc)
	c.Occurrence = "earlier"
	if len(matches) > 1 {
		c.Ambiguous = true
		c.Earlier = occurrenceLabel(matches[0])
		c.Later = occurrenceLabel(matches[len(matches)-1])
		if instant.Equal(matches[len(matches)-1]) {
			c.Occurrence = "later"
		}
	}
	_, sourceOffset := source.Zone()
	_, targetOffset := target.Zone()
	difference := "Both places have the same UTC offset at this time."
	if delta := targetOffset - sourceOffset; delta != 0 {
		direction := "ahead of"
		if delta < 0 {
			direction = "behind"
			delta = -delta
		}
		duration := formatDaylight(time.Duration(delta) * time.Second)
		if delta%60 != 0 {
			duration = (time.Duration(delta) * time.Second).String()
		}
		difference = fmt.Sprintf("%s is %s %s %s at this time.", to.Display, duration, direction, from.Display)
	}
	sourceDay := time.Date(source.Year(), source.Month(), source.Day(), 0, 0, 0, 0, time.UTC)
	targetDay := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, time.UTC)
	days := int(targetDay.Sub(sourceDay).Hours() / 24)
	day := "Same day"
	switch {
	case days == 1:
		day = "Next day"
	case days == -1:
		day = "Previous day"
	case days > 1:
		day = fmt.Sprintf("%d days later", days)
	case days < -1:
		day = fmt.Sprintf("%d days earlier", -days)
	}
	share := url.Values{
		"from": {c.From}, "to": {c.To}, "date": {c.Date},
		"time": {c.Time}, "occurrence": {c.Occurrence},
	}
	c.Result = &conversionResult{
		From: conversionView(from, source), To: conversionView(to, target),
		Difference: difference, Day: day,
		ShareURL: "/timezone-converter?" + share.Encode(),
	}
	s.renderPage(w, "converter", d)
}

func parseConverterTime(date, clock string) (time.Time, error) {
	layout := "2006-01-02T15:04"
	if len(clock) == 8 {
		layout += ":05"
	}
	input := date + "T" + clock
	wall, err := time.Parse(layout, input)
	if err != nil || wall.Year() < 1 || wall.Format(layout) != input {
		return time.Time{}, fmt.Errorf("invalid date and time")
	}
	return wall, nil
}

// Treat wall as calendar fields, not an instant. Visit every zone interval
// within 48 hours of those fields and round-trip each possible UTC offset.
// This catches gaps and folds (including half-hour changes and skipped
// dates), instead of accepting time.Date's arbitrary choice at a transition.
func wallTimeMatches(wall time.Time, loc *time.Location) []time.Time {
	var matches []time.Time
	limit := wall.Add(48 * time.Hour)
	for cursor := wall.Add(-48 * time.Hour).In(loc); !cursor.After(limit); {
		_, offset := cursor.Zone()
		candidate := wall.Add(-time.Duration(offset) * time.Second).In(loc)
		if candidate.Format("2006-01-02T15:04:05") == wall.Format("2006-01-02T15:04:05") &&
			!slices.ContainsFunc(matches, candidate.Equal) {
			matches = append(matches, candidate)
		}
		_, end := cursor.ZoneBounds()
		if end.IsZero() {
			break
		}
		if !end.After(cursor) {
			// A POSIX extension can end at the last day of a year. Advance
			// into the next day so it can supply the next year's interval.
			end = cursor.Add(24 * time.Hour)
		}
		cursor = end.In(loc)
	}
	slices.SortFunc(matches, func(a, b time.Time) int { return a.Compare(b) })
	return matches
}

func converterClock(t time.Time) string {
	if t.Second() != 0 {
		return t.Format("15:04:05")
	}
	return t.Format("15:04")
}

func occurrenceLabel(t time.Time) string {
	name, offset := t.Zone()
	return name + " · " + converterOffset(offset)
}

func converterOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign, seconds = "-", -seconds
	}
	offset := fmt.Sprintf("UTC%s%02d:%02d", sign, seconds/3600, seconds%3600/60)
	if seconds%60 != 0 {
		offset += fmt.Sprintf(":%02d", seconds%60)
	}
	return offset
}

func conversionView(place tz.Place, t time.Time) conversionTime {
	_, offset := t.Zone()
	iso := t.Format(time.RFC3339)
	if offset%60 != 0 {
		// RFC3339 cannot express historical offsets with seconds. Keep
		// the machine-readable datetime exact by expressing it in UTC.
		iso = t.UTC().Format(time.RFC3339)
	}
	status := "Daylight saving is not active"
	if t.IsDST() {
		status = "Daylight saving is active"
	}
	return conversionTime{
		Name: place.Display, Zone: place.Zone, Time: converterClock(t),
		Date: t.Format("Monday, 2 January 2006"), ISO: iso,
		Offset: converterOffset(offset), Status: status,
	}
}
