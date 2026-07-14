package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/admk-studio/timetools.io/internal/render"
	"github.com/admk-studio/timetools.io/internal/solar"
	"github.com/admk-studio/timetools.io/internal/tz"
)

// maxCities caps a comparison request. Nobody compares more than a
// handful of places at once; more than this is someone probing.
const maxCities = 10

func trimPath(r *http.Request) string {
	return strings.ToLower(strings.Trim(r.URL.Path, "/"))
}

func rawPath(r *http.Request) string {
	return strings.Trim(r.URL.Path, "/")
}

// options derives render options from the query string.
func (s *Server) options(r *http.Request) render.Options {
	q := r.URL.Query()
	return render.Options{
		Plain:      q.Has("plain") || q.Has("ascii") || q.Has("no-color"),
		TwelveHour: q.Has("12"),
		BaseURL:    s.cfg.BaseURL,
		Repo:       s.cfg.RepoURL,
	}
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	switch negotiate(r) {
	case formatHTML:
		s.renderPage(w, "landing", s.landingData())
	case formatJSON:
		// The root as JSON is UTC now; a reasonable thing to ask an API.
		s.respondCities(w, r, []render.City{s.buildCity(mustUTC())}, formatJSON)
	default:
		s.writeText(w, http.StatusOK, render.Help(s.now(), s.options(r)))
	}
}

func (s *Server) handleHelp(w http.ResponseWriter, r *http.Request) {
	s.writeText(w, http.StatusOK, render.Help(s.now(), s.options(r)))
}

// handleZones lists the canonical zones, filtered by ?q= against the
// zone name or an ISO country code. If the query itself resolves as a
// place ("india", "nyc"), that zone leads the results.
func (s *Server) handleZones(w http.ResponseWriter, r *http.Request) {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	code := strings.ToUpper(query)
	var names []string
	for _, z := range tz.Zones() {
		if query == "" || strings.Contains(strings.ToLower(z.Name), query) ||
			containsCode(z.Countries, code) {
			names = append(names, z.Name)
		}
	}
	sort.Strings(names)
	if query != "" {
		if p, err := tz.Resolve(query); err == nil && !slices.Contains(names, p.Zone) {
			names = append([]string{p.Zone}, names...)
		}
	}

	var b strings.Builder
	if len(names) == 0 {
		fmt.Fprintf(&b, "no zones match %q\n", query)
		s.writeText(w, http.StatusNotFound, b.String())
		return
	}
	for _, n := range names {
		b.WriteString(n + "\n")
	}
	fmt.Fprintf(&b, "\n%d zones", len(names))
	if query != "" {
		fmt.Fprintf(&b, " matching %q", query)
	}
	b.WriteString("\n")
	s.writeText(w, http.StatusOK, b.String())
}

func containsCode(csv, code string) bool {
	return slices.Contains(strings.Split(csv, ","), code)
}

func (s *Server) handleCities(w http.ResponseWriter, r *http.Request) {
	places, err := resolvePath(rawPath(r))
	if err != nil {
		s.respondNotFound(w, r, err)
		return
	}
	cities := make([]render.City, len(places))
	for i, p := range places {
		city, err := s.buildCityChecked(p)
		if err != nil {
			// A place that resolves but whose zone won't load means our
			// data is broken, not the user's request.
			s.log.Error("zone failed to load", "zone", p.Zone, "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		cities[i] = city
	}
	s.respondCities(w, r, cities, negotiate(r))
}

// resolvePath turns a URL path into places. A whole path with slashes is
// tried as a single IANA name first ("America/New_York"), then each
// segment as its own place ("nyc/london/tokyo").
func resolvePath(path string) ([]tz.Place, error) {
	if p, err := tz.Resolve(path); err == nil {
		return []tz.Place{p}, nil
	}
	segments := strings.Split(path, "/")
	if len(segments) > maxCities {
		return nil, fmt.Errorf("too many places: %d is more than %d", len(segments), maxCities)
	}
	places := make([]tz.Place, len(segments))
	for i, seg := range segments {
		p, err := tz.Resolve(seg)
		if err != nil {
			return nil, err
		}
		places[i] = p
	}
	return places, nil
}

func (s *Server) respondCities(w http.ResponseWriter, r *http.Request, cities []render.City, f format) {
	opts := s.options(r)
	switch f {
	case formatJSON:
		body, err := render.JSON(cities, s.now())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		s.writeJSON(w, http.StatusOK, body)
	case formatHTML:
		s.renderPage(w, "cities", s.citiesData(cities))
	default:
		if len(cities) == 1 {
			s.writeText(w, http.StatusOK, render.Card(cities[0], opts))
		} else {
			s.writeText(w, http.StatusOK, render.Table(cities, opts))
		}
	}
}

func (s *Server) respondNotFound(w http.ResponseWriter, r *http.Request, err error) {
	var nf *tz.NotFoundError
	query, suggestions := rawPath(r), []string(nil)
	if errors.As(err, &nf) {
		query, suggestions = nf.Query, nf.Suggestions
	}
	switch negotiate(r) {
	case formatJSON:
		s.writeJSON(w, http.StatusNotFound, notFoundJSON(query, suggestions))
	case formatHTML:
		s.renderPageStatus(w, http.StatusNotFound, "error", s.errorData(query, suggestions))
	default:
		s.writeText(w, http.StatusNotFound, render.ErrorText(query, suggestions, s.options(r)))
	}
}

// buildCity assembles the view model for one place at the current time.
func (s *Server) buildCity(p tz.Place) render.City {
	c, _ := s.buildCityChecked(p)
	return c
}

func (s *Server) buildCityChecked(p tz.Place) (render.City, error) {
	loc, err := p.Location()
	if err != nil {
		return render.City{}, err
	}
	now := s.now().In(loc)
	c := render.City{Name: p.Display, Zone: p.Zone, Now: now}
	if p.HasCoords {
		day := solar.Compute(now, p.Lat, p.Lon)
		c.Sun = &day
	}
	if tr, ok := tz.NextTransition(loc, now); ok {
		c.Next = &tr
	}
	return c, nil
}

func mustUTC() tz.Place {
	p, err := tz.Resolve("utc")
	if err != nil {
		panic("utc must always resolve: " + err.Error())
	}
	return p
}

func notFoundJSON(query string, suggestions []string) []byte {
	body, err := json.Marshal(struct {
		Error       string   `json:"error"`
		Query       string   `json:"query"`
		Suggestions []string `json:"suggestions,omitempty"`
	}{"unknown place", query, suggestions})
	if err != nil {
		return []byte(`{"error": "unknown place"}`)
	}
	return body
}
