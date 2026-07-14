package tz

import (
	"errors"
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		query       string
		wantZone    string
		wantDisplay string
	}{
		// Plain city names that are zone segments.
		{"tokyo", "Asia/Tokyo", "Tokyo"},
		{"Tokyo", "Asia/Tokyo", "Tokyo"},
		{"london", "Europe/London", "London"},
		{"new york", "America/New_York", "New York"},
		{"new_york", "America/New_York", "New York"},
		{"NewYork", "America/New_York", "New York"},
		{"buenos aires", "America/Argentina/Buenos_Aires", "Buenos Aires"},
		{"ho chi minh", "Asia/Ho_Chi_Minh", "Ho Chi Minh"},

		// Full IANA names, any case.
		{"America/New_York", "America/New_York", "New York"},
		{"america/new_york", "America/New_York", "New York"},
		{"Asia/Tokyo", "Asia/Tokyo", "Tokyo"},

		// Accent folding.
		{"são paulo", "America/Sao_Paulo", "Sao Paulo"},
		{"zürich", "Europe/Zurich", "Zurich"},
		{"bogotá", "America/Bogota", "Bogota"},

		// Aliases keep the asked-about city as display name.
		{"nyc", "America/New_York", "New York"},
		{"miami", "America/New_York", "Miami"},
		{"sf", "America/Los_Angeles", "San Francisco"},
		{"beijing", "Asia/Shanghai", "Beijing"},
		{"mumbai", "Asia/Kolkata", "Mumbai"},
		{"stockholm", "Europe/Berlin", "Stockholm"},
		{"amsterdam", "Europe/Brussels", "Amsterdam"},
		{"reykjavik", "Africa/Abidjan", "Reykjavík"},
		{"kuala lumpur", "Asia/Singapore", "Kuala Lumpur"},

		// Time zone abbreviations.
		{"est", "America/New_York", "New York"},
		{"pst", "America/Los_Angeles", "Los Angeles"},
		{"ist", "Asia/Kolkata", "India"},
		{"jst", "Asia/Tokyo", "Tokyo"},

		// UTC spellings.
		{"utc", "UTC", "UTC"},
		{"gmt", "UTC", "UTC"},
		{"UTC", "UTC", "UTC"},
	}
	for _, tt := range tests {
		p, err := Resolve(tt.query)
		if err != nil {
			t.Errorf("Resolve(%q): unexpected error: %v", tt.query, err)
			continue
		}
		if p.Zone != tt.wantZone {
			t.Errorf("Resolve(%q).Zone = %q, want %q", tt.query, p.Zone, tt.wantZone)
		}
		if p.Display != tt.wantDisplay {
			t.Errorf("Resolve(%q).Display = %q, want %q", tt.query, p.Display, tt.wantDisplay)
		}
	}
}

func TestResolveLocationsLoad(t *testing.T) {
	// Every canonical zone must load with the embedded tzdata.
	for _, z := range Zones() {
		if _, err := loadLocation(z.Name); err != nil {
			t.Errorf("zone %s does not load: %v", z.Name, err)
		}
	}
	// Every alias must point at a loadable zone.
	for key, a := range aliases {
		if _, err := loadLocation(a.Zone); err != nil {
			t.Errorf("alias %q points at unloadable zone %s: %v", key, a.Zone, err)
		}
	}
}

func TestResolveTZDBLinks(t *testing.T) {
	// Links like Asia/Calcutta are not in zone1970.tab but must still
	// resolve via the LoadLocation fallback.
	p, err := Resolve("asia/calcutta")
	if err != nil {
		t.Fatalf("Resolve(asia/calcutta): %v", err)
	}
	if p.Zone != "Asia/Calcutta" {
		t.Errorf("Zone = %q, want Asia/Calcutta", p.Zone)
	}
	if p.HasCoords {
		t.Error("link zones resolved by guess should not claim coordinates")
	}
}

func TestResolveFixedOffset(t *testing.T) {
	tests := []struct {
		query      string
		wantOffset int // seconds
		wantName   string
	}{
		{"utc+5", 5 * 3600, "UTC+05:00"},
		{"UTC+5:30", 5*3600 + 30*60, "UTC+05:30"},
		{"utc+0530", 5*3600 + 30*60, "UTC+05:30"},
		{"gmt-3", -3 * 3600, "UTC-03:00"},
		{"utc+14", 14 * 3600, "UTC+14:00"},
		{"utc-12", -12 * 3600, "UTC-12:00"},
		{"utc+0", 0, "UTC"},
	}
	for _, tt := range tests {
		p, err := Resolve(tt.query)
		if err != nil {
			t.Errorf("Resolve(%q): %v", tt.query, err)
			continue
		}
		loc, err := p.Location()
		if err != nil {
			t.Errorf("Resolve(%q).Location(): %v", tt.query, err)
			continue
		}
		_, off := time.Now().In(loc).Zone()
		if off != tt.wantOffset {
			t.Errorf("Resolve(%q) offset = %d, want %d", tt.query, off, tt.wantOffset)
		}
		if p.Zone != tt.wantName {
			t.Errorf("Resolve(%q).Zone = %q, want %q", tt.query, p.Zone, tt.wantName)
		}
	}

	// Out-of-range and malformed offsets must not resolve.
	for _, q := range []string{"utc+15", "utc-13", "utc+5:75", "utc+abc"} {
		if _, err := Resolve(q); err == nil {
			t.Errorf("Resolve(%q) should fail", q)
		}
	}
}

func TestResolveNotFound(t *testing.T) {
	_, err := Resolve("tokio")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("expected NotFoundError, got %v", err)
	}
	found := false
	for _, s := range nf.Suggestions {
		if s == "tokyo" {
			found = true
		}
	}
	if !found {
		t.Errorf("suggestions for 'tokio' = %v, want to include 'tokyo'", nf.Suggestions)
	}

	if _, err := Resolve("xxqqzz"); err == nil {
		t.Error("Resolve(xxqqzz) should fail")
	}
	if _, err := Resolve(""); err == nil {
		t.Error("Resolve of empty string should fail")
	}
	if _, err := Resolve("!!!"); err == nil {
		t.Error("Resolve of punctuation should fail")
	}
}

func TestNextTransition(t *testing.T) {
	// Fixed historical instants: tzdata ships the past, so these results
	// can never drift as rules change.
	nyc, err := loadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	// From Jan 1 2024, New York's next change was the spring-forward on
	// March 10 2024 at 03:00 EDT (07:00 UTC).
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	tr, ok := NextTransition(nyc, from)
	if !ok {
		t.Fatal("expected a transition for New York")
	}
	want := time.Date(2024, 3, 10, 7, 0, 0, 0, time.UTC)
	if !tr.At.Equal(want) {
		t.Errorf("transition at %v, want %v", tr.At.UTC(), want)
	}
	if tr.Before != -5*3600 || tr.After != -4*3600 {
		t.Errorf("offsets = %d → %d, want -18000 → -14400", tr.Before, tr.After)
	}
	if tr.Delta() != time.Hour {
		t.Errorf("delta = %v, want 1h", tr.Delta())
	}

	// Tokyo has no DST: no transition inside the horizon.
	tokyo, err := loadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := NextTransition(tokyo, from); ok {
		t.Error("Tokyo should have no upcoming transition")
	}

	// Southern hemisphere: from July 2024, Sydney's next change was DST
	// starting October 6 2024 at 03:00 AEDT (Oct 5 16:00 UTC).
	sydney, err := loadLocation("Australia/Sydney")
	if err != nil {
		t.Fatal(err)
	}
	tr, ok = NextTransition(sydney, time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("expected a transition for Sydney")
	}
	want = time.Date(2024, 10, 5, 16, 0, 0, 0, time.UTC)
	if !tr.At.Equal(want) {
		t.Errorf("Sydney transition at %v, want %v", tr.At.UTC(), want)
	}
}

func TestSuggestionTokenUsable(t *testing.T) {
	// Whatever we suggest must itself resolve; a suggestion that 404s is
	// worse than none.
	for _, q := range []string{"tokio", "landon", "new yark", "berln"} {
		_, err := Resolve(q)
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			continue
		}
		for _, s := range nf.Suggestions {
			if _, err := Resolve(s); err != nil {
				t.Errorf("suggestion %q for %q does not resolve: %v", s, q, err)
			}
		}
	}
}
