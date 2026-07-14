// Package render turns resolved places into terminal text, JSON, and the
// bits of prose the service shows users.
package render

import (
	"time"

	"github.com/admk-studio/timetools.io/internal/solar"
	"github.com/admk-studio/timetools.io/internal/tz"
)

// City is the view model for one place: everything already computed, so
// renderers only format.
type City struct {
	Name string    // display name, e.g. "New York"
	Zone string    // IANA name, e.g. "America/New_York"
	Now  time.Time // current time, in the city's location
	Sun  *solar.Day
	Next *tz.Transition // nil when no offset change is coming
}

// Options controls output shape. Plain disables ANSI colors and swaps
// the glyphs for pure ASCII, for pipes and ancient terminals.
type Options struct {
	Plain      bool
	TwelveHour bool
	BaseURL    string // hostname shown in examples, e.g. "timetools.io"
	Repo       string // source link shown in the help footer
}

func (o Options) timeFormat(seconds bool) string {
	switch {
	case o.TwelveHour && seconds:
		return "3:04:05 PM"
	case o.TwelveHour:
		return "3:04 PM"
	case seconds:
		return "15:04:05"
	default:
		return "15:04"
	}
}

// dot is the field separator: a middle dot, or a pipe when Plain.
func (o Options) dot() string {
	if o.Plain {
		return " | "
	}
	return " · "
}
