// Package solar computes sunrise and sunset with the NOAA sunrise
// equation. Plain spherical astronomy — accurate to a couple of minutes,
// which is all a clock display needs — and no dependencies.
package solar

import (
	"math"
	"time"
)

// Kind classifies the day at a given latitude.
type Kind int

const (
	Normal      Kind = iota
	MidnightSun      // the sun never sets
	PolarNight       // the sun never rises
)

// Day holds the sun times for one calendar day at one spot. Rise and Set
// are only meaningful when K is Normal.
type Day struct {
	Rise, Set time.Time
	K         Kind
}

// Daylight is the time the sun spends above the horizon.
func (d Day) Daylight() time.Duration {
	switch d.K {
	case MidnightSun:
		return 24 * time.Hour
	case PolarNight:
		return 0
	}
	return d.Set.Sub(d.Rise)
}

const (
	j2000         = 2451545.0 // Julian date of 2000-01-01 12:00 UTC
	unixEpochJD   = 2440587.5 // Julian date of 1970-01-01 00:00 UTC
	obliquity     = 23.4397   // Earth's axial tilt, degrees
	sunAltitude   = -0.833    // horizon altitude for rise/set: refraction + solar radius
	degToRad      = math.Pi / 180
	radToDeg      = 180 / math.Pi
	secondsPerDay = 86400.0
)

// Compute returns sunrise and sunset for the calendar day of t (in t's
// location) at latitude/longitude in degrees, east and north positive.
func Compute(t time.Time, lat, lon float64) Day {
	// Anchor on local solar noon so "the day of t" means the day the
	// observer's calendar shows, whatever their UTC offset is.
	y, m, d := t.Date()
	localNoon := time.Date(y, m, d, 12, 0, 0, 0, t.Location())
	jdNoon := float64(localNoon.Unix())/secondsPerDay + unixEpochJD

	// Whole solar days since the J2000 epoch, corrected for how far the
	// observer sits from the prime meridian.
	n := math.Round(jdNoon - j2000 + lon/360)
	solarTime := n - lon/360

	// Solar mean anomaly and the equation-of-center correction.
	meanAnomaly := math.Mod(357.5291+0.98560028*solarTime, 360)
	mRad := meanAnomaly * degToRad
	center := 1.9148*math.Sin(mRad) + 0.0200*math.Sin(2*mRad) + 0.0003*math.Sin(3*mRad)

	// Ecliptic longitude of the sun; 102.9372 is the argument of perihelion.
	eclipticLon := math.Mod(meanAnomaly+center+180+102.9372, 360)
	lRad := eclipticLon * degToRad

	transit := j2000 + solarTime + 0.0053*math.Sin(mRad) - 0.0069*math.Sin(2*lRad)

	declination := math.Asin(math.Sin(lRad) * math.Sin(obliquity*degToRad))

	// Hour angle at which the sun crosses our effective horizon.
	latRad := lat * degToRad
	cosHA := (math.Sin(sunAltitude*degToRad) - math.Sin(latRad)*math.Sin(declination)) /
		(math.Cos(latRad) * math.Cos(declination))

	switch {
	case cosHA > 1:
		return Day{K: PolarNight}
	case cosHA < -1:
		return Day{K: MidnightSun}
	}

	hourAngle := math.Acos(cosHA) * radToDeg
	rise := jdToTime(transit-hourAngle/360, t.Location())
	set := jdToTime(transit+hourAngle/360, t.Location())
	return Day{Rise: rise, Set: set, K: Normal}
}

func jdToTime(jd float64, loc *time.Location) time.Time {
	unix := (jd - unixEpochJD) * secondsPerDay
	return time.Unix(int64(math.Round(unix)), 0).In(loc)
}
