package server

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/admk-studio/timetools.io/internal/render"
	"github.com/admk-studio/timetools.io/internal/solar"
)

//go:embed templates/*.html
var templateFS embed.FS

func parseTemplates() (*template.Template, error) {
	return template.ParseFS(templateFS, "templates/*.html")
}

// pageData feeds every HTML page; unused fields stay zero.
type pageData struct {
	Title       string
	Desc        string
	BaseURL     string
	RepoName    string // "github.com/x/y", as shown
	RepoHref    string // with scheme, for the href
	LinkText    string
	LinkURL     string
	Version     string
	Cities      []cityView
	Query       string
	Suggestions []string
}

// cityView is one city pre-formatted for the template. TZ is set when
// the browser's Intl API can handle the zone name; otherwise the clock
// falls back to OffsetSecs.
type cityView struct {
	Name       string
	Zone       string
	TZ         string
	OffsetSecs int
	Time       string
	Date       string
	Offset     string
	Abbrev     string
	Status     string
	Sunrise    string
	Sunset     string
	Daylight   string
	Change     string
}

func (s *Server) pageBase(title, desc string) pageData {
	href := s.cfg.RepoURL
	if href != "" && !strings.Contains(href, "://") {
		href = "https://" + href
	}
	return pageData{
		Title:    title,
		Desc:     desc,
		BaseURL:  s.cfg.BaseURL,
		RepoName: s.cfg.RepoURL,
		RepoHref: href,
		LinkText: s.cfg.LinkText,
		LinkURL:  s.cfg.LinkURL,
		Version:  s.cfg.Version,
	}
}

func (s *Server) landingData() pageData {
	return s.pageBase(s.cfg.BaseURL+" — time from your terminal",
		"A curl-able world clock: current time, time zones, DST changes and sunrise for any city.")
}

func (s *Server) citiesData(cities []render.City) pageData {
	names := make([]string, len(cities))
	views := make([]cityView, len(cities))
	for i, c := range cities {
		names[i] = c.Name
		views[i] = newCityView(c)
	}
	d := s.pageBase(strings.Join(names, ", ")+" — "+s.cfg.BaseURL,
		"Current local time in "+strings.Join(names, ", ")+".")
	d.Cities = views
	return d
}

func (s *Server) errorData(query string, suggestions []string) pageData {
	d := s.pageBase("not found — "+s.cfg.BaseURL, "")
	d.Query = query
	d.Suggestions = suggestions
	return d
}

func newCityView(c render.City) cityView {
	_, offset := c.Now.Zone()
	v := cityView{
		Name:       c.Name,
		Zone:       c.Zone,
		OffsetSecs: offset,
		Time:       c.Now.Format("15:04:05"),
		Date:       c.Now.Format("Monday, 2 January 2006"),
		Offset:     "UTC" + fmtOffsetHTML(offset),
	}
	// Browsers' Intl API understands IANA names and plain "UTC", but not
	// our synthetic fixed-offset names.
	if strings.Contains(c.Zone, "/") || c.Zone == "UTC" {
		v.TZ = c.Zone
	}
	if name, _ := c.Now.Zone(); name != "" && name[0] != '+' && name[0] != '-' && name != c.Zone {
		v.Abbrev = name
	}
	switch {
	case c.Now.IsDST():
		v.Status = "daylight saving time"
	case c.Next != nil:
		v.Status = "standard time"
	}
	if c.Sun != nil {
		switch c.Sun.K {
		case solar.Normal:
			v.Sunrise = c.Sun.Rise.Format("15:04")
			v.Sunset = c.Sun.Set.Format("15:04")
			v.Daylight = formatDaylight(c.Sun.Daylight())
		case solar.MidnightSun:
			v.Daylight = "midnight sun — no sunset today"
		case solar.PolarNight:
			v.Daylight = "polar night — no sunrise today"
		}
	}
	if c.Next != nil {
		wall := time.Unix(c.Next.At.Unix(), 0).In(time.FixedZone("", c.Next.Before))
		dir := "forward"
		delta := c.Next.Delta()
		if delta < 0 {
			dir = "back"
			delta = -delta
		}
		v.Change = "clocks go " + dir + " " + formatDaylight(delta) +
			" on " + wall.Format("Jan 2 at 15:04")
	}
	return v
}

func fmtOffsetHTML(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	h, m := seconds/3600, seconds%3600/60
	if m == 0 {
		return fmt.Sprintf("%s%d", sign, h)
	}
	return fmt.Sprintf("%s%d:%02d", sign, h, m)
}

func formatDaylight(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dh %02dm", h, m)
	}
}

func (s *Server) renderPage(w http.ResponseWriter, name string, data pageData) {
	s.renderPageStatus(w, http.StatusOK, name, data)
}

func (s *Server) renderPageStatus(w http.ResponseWriter, status int, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	// Everything is inlined and the pages make no outbound requests.
	w.Header().Set("Content-Security-Policy",
		"default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src data:")
	w.WriteHeader(status)
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		s.log.Error("template render failed", "template", name, "err", err)
	}
}
