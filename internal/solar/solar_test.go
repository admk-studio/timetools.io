package solar

import (
	"testing"
	"time"
	_ "time/tzdata"
)

// Reference values from NOAA's solar calculator. The algorithm is only
// good to a couple of minutes, so allow a 10-minute window.
func TestCompute(t *testing.T) {
	const tolerance = 10 * time.Minute

	tests := []struct {
		name      string
		zone      string
		date      string // YYYY-MM-DD, local
		lat, lon  float64
		rise, set string // HH:MM, local
	}{
		{"New York summer solstice", "America/New_York", "2024-06-20", 40.71, -74.01, "05:25", "20:31"},
		{"New York winter solstice", "America/New_York", "2024-12-21", 40.71, -74.01, "07:17", "16:32"},
		{"London winter", "Europe/London", "2024-12-21", 51.51, -0.13, "08:04", "15:53"},
		{"London summer", "Europe/London", "2024-06-20", 51.51, -0.13, "04:43", "21:21"},
		{"Singapore equator", "Asia/Singapore", "2024-03-20", 1.35, 103.82, "07:10", "19:16"},
		{"Sydney southern winter", "Australia/Sydney", "2024-06-20", -33.87, 151.21, "07:00", "16:54"},
		{"Tokyo spring", "Asia/Tokyo", "2024-04-01", 35.68, 139.69, "05:28", "18:02"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := time.LoadLocation(tt.zone)
			if err != nil {
				t.Fatal(err)
			}
			date, err := time.ParseInLocation("2006-01-02", tt.date, loc)
			if err != nil {
				t.Fatal(err)
			}
			day := Compute(date, tt.lat, tt.lon)
			if day.K != Normal {
				t.Fatalf("kind = %v, want Normal", day.K)
			}
			checkClose(t, "sunrise", day.Rise, tt.date+" "+tt.rise, loc, tolerance)
			checkClose(t, "sunset", day.Set, tt.date+" "+tt.set, loc, tolerance)
		})
	}
}

func checkClose(t *testing.T, what string, got time.Time, want string, loc *time.Location, tol time.Duration) {
	t.Helper()
	w, err := time.ParseInLocation("2006-01-02 15:04", want, loc)
	if err != nil {
		t.Fatal(err)
	}
	diff := got.Sub(w)
	if diff < 0 {
		diff = -diff
	}
	if diff > tol {
		t.Errorf("%s = %s, want %s ±%s", what, got.Format("15:04:05"), want, tol)
	}
}

func TestPolar(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		t.Fatal(err)
	}
	tromso := func(date string) Day {
		d, err := time.ParseInLocation("2006-01-02", date, loc)
		if err != nil {
			t.Fatal(err)
		}
		return Compute(d, 69.65, 18.96)
	}

	if d := tromso("2024-12-21"); d.K != PolarNight {
		t.Errorf("Tromsø in December: kind = %v, want PolarNight", d.K)
	} else if d.Daylight() != 0 {
		t.Errorf("polar night daylight = %v, want 0", d.Daylight())
	}

	if d := tromso("2024-06-21"); d.K != MidnightSun {
		t.Errorf("Tromsø in June: kind = %v, want MidnightSun", d.K)
	} else if d.Daylight() != 24*time.Hour {
		t.Errorf("midnight sun daylight = %v, want 24h", d.Daylight())
	}
}

func TestDaylight(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Singapore")
	if err != nil {
		t.Fatal(err)
	}
	d := Compute(time.Date(2024, 3, 20, 0, 0, 0, 0, loc), 1.35, 103.82)
	// Equinox at the equator: very close to 12 hours of daylight.
	if dl := d.Daylight(); dl < 11*time.Hour+50*time.Minute || dl > 12*time.Hour+20*time.Minute {
		t.Errorf("equatorial equinox daylight = %v, want ~12h", dl)
	}
}
