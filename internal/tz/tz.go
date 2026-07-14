// Package tz resolves human place names ("tokyo", "nyc", "utc+5:30") to
// IANA time zones, with coordinates for sunrise/sunset math and a scanner
// for upcoming UTC-offset changes.
package tz

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	// The binary must work in a scratch container and on hosts without
	// /usr/share/zoneinfo, so ship the zone database inside it.
	_ "time/tzdata"
)

//go:generate sh -c "go run ../../gen/zones > zones_gen.go"

// Zone is one canonical IANA zone from zone1970.tab. The coordinates are
// those of the zone's principal city.
type Zone struct {
	Name      string
	Countries string
	Lat, Lon  float64
	HasCoords bool
}

// Place is a resolved query: a display name plus the zone it lives in.
// Display can differ from the zone's city ("Miami" resolves to
// America/New_York but should still be called Miami).
type Place struct {
	Display   string
	Zone      string
	Lat, Lon  float64
	HasCoords bool

	fixed *time.Location // set for UTC±HH:MM queries only
}

// Location returns the *time.Location for the place.
func (p Place) Location() (*time.Location, error) {
	if p.fixed != nil {
		return p.fixed, nil
	}
	return loadLocation(p.Zone)
}

// LoadLocation re-parses the zone data on every call, which is wasteful
// under load, so keep the handful of locations we've seen around.
var locCache sync.Map

func loadLocation(name string) (*time.Location, error) {
	if v, ok := locCache.Load(name); ok {
		return v.(*time.Location), nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, err
	}
	locCache.Store(name, loc)
	return loc, nil
}

// NotFoundError is returned by Resolve for unknown places. Suggestions
// holds up to three query strings the caller can offer as corrections.
type NotFoundError struct {
	Query       string
	Suggestions []string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("unknown place %q", e.Query)
}

var (
	buildOnce sync.Once
	index     map[string]Place
	indexKeys []string // sorted, for suggestion scans
)

// Resolve maps a query to a Place. It accepts city names ("tokyo",
// "new york"), aliases and abbreviations ("nyc", "pst"), full IANA names
// in any case ("america/new_york"), and fixed offsets ("utc+5:30").
func Resolve(query string) (Place, error) {
	buildOnce.Do(buildIndex)

	q := strings.TrimSpace(query)
	if p, ok := parseFixedOffset(q); ok {
		return p, nil
	}
	norm := normalize(q)
	if norm == "" {
		return Place{}, &NotFoundError{Query: query}
	}
	if p, ok := index[norm]; ok {
		return p, nil
	}
	// The canonical table only has zones from zone1970.tab; the full tzdb
	// also carries links like Asia/Calcutta. Reconstruct a plausible IANA
	// name and let LoadLocation have the final word.
	if name, ok := guessIANAName(q); ok {
		return Place{Display: cityOf(name), Zone: name}, nil
	}
	return Place{}, &NotFoundError{Query: query, Suggestions: suggest(norm)}
}

// Zones returns the canonical zone table, sorted by name. Callers must
// treat it as read-only.
func Zones() []Zone {
	return zones
}

func buildIndex() {
	index = make(map[string]Place, 3*len(zones))

	add := func(key string, p Place) {
		if key == "" {
			return
		}
		if _, exists := index[key]; !exists {
			index[key] = p
		}
	}

	for _, z := range zones {
		p := Place{Display: cityOf(z.Name), Zone: z.Name, Lat: z.Lat, Lon: z.Lon, HasCoords: true}
		add(normalize(z.Name), p)
		add(normalize(cityOf(z.Name)), p)
	}
	// Aliases override zone-derived keys on purpose: if a curated entry
	// disagrees with a derived one, the curated one is the deliberate pick.
	for key, a := range aliases {
		p := Place{Display: a.Display, Zone: a.Zone, Lat: a.Lat, Lon: a.Lon, HasCoords: a.HasCoords}
		if p.Display == "" {
			p.Display = cityOf(a.Zone)
		}
		if !a.HasCoords {
			if z, ok := zoneByName(a.Zone); ok {
				p.Lat, p.Lon, p.HasCoords = z.Lat, z.Lon, true
			}
		}
		index[normalize(key)] = p
	}

	indexKeys = make([]string, 0, len(index))
	for k := range index {
		indexKeys = append(indexKeys, k)
	}
	sort.Strings(indexKeys)
}

func zoneByName(name string) (Zone, bool) {
	i := sort.Search(len(zones), func(i int) bool { return zones[i].Name >= name })
	if i < len(zones) && zones[i].Name == name {
		return zones[i], true
	}
	return Zone{}, false
}

// cityOf extracts the display city from an IANA name:
// "America/New_York" → "New York".
func cityOf(zoneName string) string {
	city := zoneName[strings.LastIndexByte(zoneName, '/')+1:]
	return strings.ReplaceAll(city, "_", " ")
}

// normalize reduces a query to lowercase letters and digits so that
// "New_York", "new york" and "NewYork" all collide. Common Latin accents
// fold to ASCII so "São Paulo" and "Zürich" work too.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r > 127:
			if folded, ok := accentFold[r]; ok {
				b.WriteString(folded)
			}
		}
	}
	return b.String()
}

var accentFold = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a", 'ă': "a", 'ą': "a",
	'ç': "c", 'ć': "c", 'č': "c",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ė': "e", 'ę': "e", 'ě': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ī': "i", 'ı': "i",
	'ñ': "n", 'ń': "n", 'ň': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ū': "u", 'ů': "u",
	'ý': "y", 'ÿ': "y",
	'ś': "s", 'š': "s", 'ş': "s", 'ș': "s",
	'ź': "z", 'ż': "z", 'ž': "z",
	'ğ': "g", 'ł': "l", 'đ': "d", 'ț': "t",
	'ß': "ss", 'æ': "ae", 'œ': "oe",
}

// guessIANAName rebuilds tzdb capitalization from a casual query:
// "asia/calcutta" → "Asia/Calcutta", "australia/lord howe" →
// "Australia/Lord_Howe". Only names LoadLocation accepts are returned.
func guessIANAName(q string) (string, bool) {
	if !strings.Contains(q, "/") {
		return "", false
	}
	segments := strings.Split(q, "/")
	for i, seg := range segments {
		words := strings.FieldsFunc(seg, func(r rune) bool { return r == ' ' || r == '_' })
		for j, w := range words {
			words[j] = titleASCII(w)
		}
		segments[i] = strings.Join(words, "_")
	}
	name := strings.Join(segments, "/")
	if _, err := loadLocation(name); err != nil {
		return "", false
	}
	return name, true
}

// parseFixedOffset handles "utc+5", "gmt-3", "utc+05:30" style queries.
func parseFixedOffset(q string) (Place, bool) {
	s := strings.ToLower(strings.TrimSpace(q))
	switch {
	case strings.HasPrefix(s, "utc"), strings.HasPrefix(s, "gmt"):
		s = s[3:]
	default:
		return Place{}, false
	}
	if s == "" {
		return Place{}, false // bare "utc" resolves through the alias table
	}
	sign := 1
	switch s[0] {
	case '+':
	case '-':
		sign = -1
	default:
		return Place{}, false
	}
	s = s[1:]

	hh, mm := s, "0"
	if h, m, ok := strings.Cut(s, ":"); ok {
		hh, mm = h, m
	} else if len(s) == 4 { // "0530"
		hh, mm = s[:2], s[2:]
	}
	hours, err := strconv.Atoi(hh)
	if err != nil || len(hh) > 2 {
		return Place{}, false
	}
	minutes, err := strconv.Atoi(mm)
	if err != nil || minutes > 59 {
		return Place{}, false
	}
	offset := sign * (hours*3600 + minutes*60)
	// Real-world offsets stop at UTC-12 and UTC+14.
	if offset < -12*3600 || offset > 14*3600 {
		return Place{}, false
	}
	if offset == 0 {
		return Place{Display: "UTC", Zone: "UTC", fixed: time.UTC}, true
	}
	name := fmt.Sprintf("UTC%+03d:%02d", sign*hours, minutes)
	return Place{Display: name, Zone: name, fixed: time.FixedZone(name, offset)}, true
}

// suggest returns up to three known queries within a small edit distance,
// closest first.
func suggest(norm string) []string {
	maxDist := 2
	if len(norm) > 8 {
		maxDist = 3
	}
	type cand struct {
		key  string
		dist int
	}
	var cands []cand
	for _, k := range indexKeys {
		// A length gap larger than the budget can't be within distance.
		if abs(len(k)-len(norm)) > maxDist {
			continue
		}
		if d := levenshtein(norm, k); d <= maxDist {
			cands = append(cands, cand{k, d})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		return cands[i].key < cands[j].key
	})
	var out []string
	seen := map[string]bool{}
	for _, c := range cands {
		token := suggestionToken(index[c.key])
		if !seen[token] {
			seen[token] = true
			out = append(out, token)
		}
		if len(out) == 3 {
			break
		}
	}
	return out
}

// suggestionToken renders a place as something the user can put straight
// back into a URL path.
func suggestionToken(p Place) string {
	return strings.ToLower(strings.ReplaceAll(p.Display, " ", "_"))
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, min(curr[j-1]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// titleASCII uppercases the first letter of an ASCII word; tzdb names
// never contain anything beyond ASCII.
func titleASCII(w string) string {
	w = strings.ToLower(w)
	if w == "" {
		return w
	}
	if w[0] >= 'a' && w[0] <= 'z' {
		return string(w[0]-32) + w[1:]
	}
	return w
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
