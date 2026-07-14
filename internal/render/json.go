package render

import (
	"encoding/json"
	"time"

	"github.com/admk-studio/timetools.io/internal/solar"
)

// The JSON shape is a public contract; scripts depend on it. Only add
// fields, never rename or remove them.

type jsonResponse struct {
	Generated time.Time      `json:"generated"`
	Locations []jsonLocation `json:"locations"`
}

type jsonLocation struct {
	Name         string          `json:"name"`
	Timezone     string          `json:"timezone"`
	Time         time.Time       `json:"time"`
	Unix         int64           `json:"unix"`
	UTCOffset    string          `json:"utc_offset"`
	Abbreviation string          `json:"abbreviation,omitempty"`
	DST          bool            `json:"dst"`
	Sun          *jsonSun        `json:"sun,omitempty"`
	NextChange   *jsonTransition `json:"next_change,omitempty"`
}

type jsonSun struct {
	Sunrise         *time.Time `json:"sunrise,omitempty"`
	Sunset          *time.Time `json:"sunset,omitempty"`
	DaylightMinutes int        `json:"daylight_minutes"`
	Polar           string     `json:"polar,omitempty"` // "midnight_sun" or "polar_night"
}

type jsonTransition struct {
	At   time.Time `json:"at"`
	From string    `json:"from"` // UTC offset before, "+01:00"
	To   string    `json:"to"`   // UTC offset after, "+02:00"
}

// JSON renders the machine-readable version of one or more cities.
func JSON(cities []City, generated time.Time) ([]byte, error) {
	resp := jsonResponse{
		Generated: generated.UTC().Truncate(time.Second),
		Locations: make([]jsonLocation, 0, len(cities)),
	}
	for _, c := range cities {
		_, offset := c.Now.Zone()
		abbrev := zoneAbbrev(c.Now)
		if abbrev == c.Zone {
			abbrev = "" // fixed offsets and UTC just repeat themselves
		}
		loc := jsonLocation{
			Name:         c.Name,
			Timezone:     c.Zone,
			Time:         c.Now.Truncate(time.Second),
			Unix:         c.Now.Unix(),
			UTCOffset:    fmtOffset(offset),
			Abbreviation: abbrev,
			DST:          c.Now.IsDST(),
		}
		if c.Sun != nil {
			sun := &jsonSun{DaylightMinutes: int(c.Sun.Daylight().Minutes())}
			switch c.Sun.K {
			case solar.Normal:
				rise, set := c.Sun.Rise, c.Sun.Set
				sun.Sunrise, sun.Sunset = &rise, &set
			case solar.MidnightSun:
				sun.Polar = "midnight_sun"
			case solar.PolarNight:
				sun.Polar = "polar_night"
			}
			loc.Sun = sun
		}
		if c.Next != nil {
			loc.NextChange = &jsonTransition{
				At:   c.Next.At.Truncate(time.Second),
				From: fmtOffset(c.Next.Before),
				To:   fmtOffset(c.Next.After),
			}
		}
		resp.Locations = append(resp.Locations, loc)
	}
	return json.MarshalIndent(resp, "", "  ")
}
