package render

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/admk-studio/timetools.io/internal/solar"
	"github.com/admk-studio/timetools.io/internal/tz"
)

// A fixed instant keeps every rendering test deterministic:
// 2024-07-16 13:41:07 UTC, a Tuesday.
var testInstant = time.Date(2024, 7, 16, 13, 41, 7, 0, time.UTC)

func mustCity(t *testing.T, query string) City {
	t.Helper()
	p, err := tz.Resolve(query)
	if err != nil {
		t.Fatal(err)
	}
	loc, err := p.Location()
	if err != nil {
		t.Fatal(err)
	}
	now := testInstant.In(loc)
	c := City{Name: p.Display, Zone: p.Zone, Now: now}
	if p.HasCoords {
		day := solar.Compute(now, p.Lat, p.Lon)
		c.Sun = &day
	}
	if tr, ok := tz.NextTransition(loc, testInstant); ok {
		c.Next = &tr
	}
	return c
}

func TestCard(t *testing.T) {
	got := Card(mustCity(t, "nyc"), Options{Plain: true})

	for _, want := range []string{
		"New York -- America/New_York",
		"09:41:07",
		"Tuesday, 16 July 2024",
		"UTC-04:00",
		"EDT",
		"daylight saving time",
		// Accuracy is asserted with tolerances in the solar package;
		// here we only care that the line is present and formatted.
		"sunrise 05:38",
		"sunset 20:25",
		"clocks go back 1h on Sun, Nov 3 at 02:00",
		"unix 1721137267",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("card missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Error("plain card contains ANSI escapes")
	}
}

func TestCardColor(t *testing.T) {
	got := Card(mustCity(t, "tokyo"), Options{})
	if !strings.Contains(got, "\x1b[") {
		t.Error("colored card has no ANSI escapes")
	}
	if !strings.Contains(got, "no seasonal clock changes") {
		t.Errorf("Tokyo card should say it has no clock changes:\n%s", got)
	}
}

func TestCardTwelveHour(t *testing.T) {
	got := Card(mustCity(t, "nyc"), Options{Plain: true, TwelveHour: true})
	if !strings.Contains(got, "9:41:07 AM") {
		t.Errorf("12-hour card missing AM time:\n%s", got)
	}
}

func TestTable(t *testing.T) {
	cities := []City{mustCity(t, "nyc"), mustCity(t, "london"), mustCity(t, "tokyo")}
	got := Table(cities, Options{Plain: true})

	for _, want := range []string{
		"New York",
		"London",
		"Tokyo",
		"09:41", "14:41", "22:41",
		"UTC-04:00", "UTC+01:00", "UTC+09:00",
		"+5h", "+13h",
		"working hours, on New York's clock",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("table missing %q:\n%s", want, got)
		}
	}
	// At 13:41 UTC, NY (09:41), London (14:41) are both inside 09-18;
	// Tokyo (22:41) is not, so the shared window excludes it.
	if !strings.Contains(got, "no shared working hours") {
		t.Errorf("expected no shared working hours with Tokyo in the mix:\n%s", got)
	}
}

func TestTableOverlap(t *testing.T) {
	got := Table([]City{mustCity(t, "nyc"), mustCity(t, "london")}, Options{Plain: true})
	// London is UTC+1, NY is UTC-4: five hours apart, so the shared
	// window on New York's clock is 09:00-13:00.
	if !strings.Contains(got, "everyone is at work 09:00-13:00, New York time") {
		t.Errorf("wrong overlap line:\n%s", got)
	}
}

func TestTableDayMarkers(t *testing.T) {
	// At 13:41 UTC it is already the 17th in Auckland (UTC+12).
	got := Table([]City{mustCity(t, "nyc"), mustCity(t, "auckland")}, Options{Plain: true})
	if !strings.Contains(got, "+1d") {
		t.Errorf("Auckland should be flagged as next day:\n%s", got)
	}
}

func TestJSON(t *testing.T) {
	data, err := JSON([]City{mustCity(t, "nyc"), mustCity(t, "utc+5:30")}, testInstant)
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Generated time.Time `json:"generated"`
		Locations []struct {
			Name         string    `json:"name"`
			Timezone     string    `json:"timezone"`
			Unix         int64     `json:"unix"`
			UTCOffset    string    `json:"utc_offset"`
			Abbreviation string    `json:"abbreviation"`
			DST          bool      `json:"dst"`
			Sun          *struct{} `json:"sun"`
			NextChange   *struct{} `json:"next_change"`
		} `json:"locations"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, data)
	}
	if len(resp.Locations) != 2 {
		t.Fatalf("got %d locations, want 2", len(resp.Locations))
	}
	ny := resp.Locations[0]
	if ny.Timezone != "America/New_York" || ny.UTCOffset != "-04:00" || !ny.DST {
		t.Errorf("bad New York entry: %+v", ny)
	}
	if ny.Unix != testInstant.Unix() {
		t.Errorf("unix = %d, want %d", ny.Unix, testInstant.Unix())
	}
	if ny.Sun == nil || ny.NextChange == nil {
		t.Error("New York should have sun and next_change")
	}
	fixed := resp.Locations[1]
	if fixed.UTCOffset != "+05:30" {
		t.Errorf("fixed offset = %q, want +05:30", fixed.UTCOffset)
	}
	if fixed.Sun != nil {
		t.Error("fixed offsets have no coordinates, so no sun")
	}
}

func TestHelp(t *testing.T) {
	got := Help(testInstant, Options{Plain: true, BaseURL: "timetools.io", Repo: "github.com/x/y"})
	for _, want := range []string{
		"timetools.io",
		"curl timetools.io/tokyo",
		"unix 1721137267",
		"github.com/x/y",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("help missing %q:\n%s", want, got)
		}
	}
}

func TestErrorText(t *testing.T) {
	got := ErrorText("tokio", []string{"tokyo"}, Options{Plain: true, BaseURL: "timetools.io"})
	if !strings.Contains(got, "tokio") || !strings.Contains(got, "curl timetools.io/tokyo") {
		t.Errorf("error text missing pieces:\n%s", got)
	}
}

func TestFormatHelpers(t *testing.T) {
	if got := fmtOffset(5*3600 + 30*60); got != "+05:30" {
		t.Errorf("fmtOffset = %q", got)
	}
	if got := fmtOffset(-4 * 3600); got != "-04:00" {
		t.Errorf("fmtOffset = %q", got)
	}
	if got := fmtDuration(14*time.Hour + 23*time.Minute); got != "14h23m" {
		t.Errorf("fmtDuration = %q", got)
	}
	if got := fmtDuration(45 * time.Minute); got != "45m" {
		t.Errorf("fmtDuration = %q", got)
	}
	if got := fmtDelta(-time.Hour); got != "-1h" {
		t.Errorf("fmtDelta = %q", got)
	}
}
