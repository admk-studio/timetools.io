// Package server is the HTTP layer: routing, format negotiation between
// terminals and browsers, rate limiting, and the HTML pages.
package server

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/netip"
	"time"
)

type Server struct {
	cfg            Config
	trustedProxies []netip.Prefix
	log            *slog.Logger
	limiter        *limiter
	tmpl           *template.Template

	// now is swappable so handler tests can pin the clock.
	now func() time.Time
}

func New(cfg Config, log *slog.Logger) (*Server, error) {
	trusted, err := parseTrustedProxies(cfg)
	if err != nil {
		return nil, err
	}
	tmpl, err := parseTemplates()
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	return &Server{
		cfg:            cfg,
		trustedProxies: trusted,
		log:            log,
		limiter:        newLimiter(cfg.RateRPM, cfg.RateBurst, time.Now),
		tmpl:           tmpl,
		now:            time.Now,
	}, nil
}

// Handler returns the full middleware stack.
func (s *Server) Handler() http.Handler {
	return s.recoverPanics(s.rateLimit(http.HandlerFunc(s.route)))
}

// HTTPServer wraps the handler in an http.Server with timeouts suitable
// for a small public service.
func (s *Server) HTTPServer() *http.Server {
	return &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    16 << 10,
	}
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// `curl timetools.io/tokyo` has to work without -L, so the proxy must
	// not blanket-redirect to HTTPS. Instead we upgrade selectively:
	// browsers get sent to HTTPS, terminals get their answer over HTTP.
	// Only possible when a trusted proxy reports the original scheme.
	if trustedIP(remoteIP(r), s.trustedProxies) && r.Header.Get("X-Forwarded-Proto") == "http" &&
		negotiate(r) == formatHTML {
		http.Redirect(w, r, "https://"+r.Host+r.URL.RequestURI(), http.StatusMovedPermanently)
		return
	}

	switch trimPath(r) {
	case "":
		s.handleRoot(w, r)
	case "help":
		s.handleHelp(w, r)
	case "timezone-converter":
		s.handleConverter(w, r)
	case "health", "healthz":
		s.writeText(w, http.StatusOK, "ok\n")
	case "version":
		s.writeText(w, http.StatusOK, s.cfg.Version+"\n")
	case "unix", "epoch":
		s.writeText(w, http.StatusOK, fmt.Sprintf("%d\n", s.now().Unix()))
	case "utc":
		s.writeText(w, http.StatusOK, s.now().UTC().Format(time.RFC3339)+"\n")
	case "zones":
		s.handleZones(w, r)
	case "robots.txt":
		s.writeText(w, http.StatusOK, "User-agent: *\nAllow: /\n")
	case "favicon.ico":
		w.WriteHeader(http.StatusNoContent)
	default:
		s.handleCities(w, r)
	}
}

func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health checks must never be throttled or the orchestrator
		// will restart a perfectly healthy container.
		if p := trimPath(r); p == "health" || p == "healthz" {
			next.ServeHTTP(w, r)
			return
		}
		if !s.limiter.allow(clientIP(r, s.trustedProxies)) {
			w.Header().Set("Retry-After", "10")
			s.writeText(w, http.StatusTooManyRequests,
				"easy there — rate limit hit, try again in a few seconds\n")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.log.Error("panic serving request", "path", r.URL.Path, "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) writeText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	fmt.Fprint(w, body)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	// The JSON endpoint is an API; let pages on other origins call it.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	w.Write(append(body, '\n'))
}
