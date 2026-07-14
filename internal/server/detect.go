package server

import (
	"net/http"
	"strings"
)

type format int

const (
	formatText format = iota
	formatHTML
	formatJSON
)

// cliAgents are substrings that identify terminal HTTP clients. Anything
// matching gets text even if it also sends a permissive Accept header.
var cliAgents = []string{
	"curl", "wget", "httpie", "http/", "fetch", "powershell",
	"python-requests", "python-urllib", "go-http-client", "okhttp", "xh/",
}

// negotiate picks the response format. Explicit ?format= wins, then the
// Accept header, then the user agent. When in doubt we answer in text:
// a browser mis-rendering text is a curiosity, a terminal full of HTML
// is a bug report.
func negotiate(r *http.Request) format {
	switch strings.ToLower(r.URL.Query().Get("format")) {
	case "json":
		return formatJSON
	case "html":
		return formatHTML
	case "text", "txt", "plain":
		return formatText
	}

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return formatJSON
	}

	ua := strings.ToLower(r.Header.Get("User-Agent"))
	for _, agent := range cliAgents {
		if strings.Contains(ua, agent) {
			return formatText
		}
	}
	if strings.Contains(ua, "mozilla") && strings.Contains(accept, "text/html") {
		return formatHTML
	}
	return formatText
}
