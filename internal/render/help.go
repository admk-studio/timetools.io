package render

import (
	"fmt"
	"strings"
	"time"
)

// Help is the response for the bare domain and /help: a live UTC line so
// the empty call is still useful, then the cheat sheet.
func Help(now time.Time, o Options) string {
	st := styler{on: !o.Plain}
	host := o.BaseURL
	var b strings.Builder

	utc := now.UTC()
	dash := "—"
	if o.Plain {
		dash = "-"
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "  %s %s\n", st.bold(st.cyan(host)), st.dim(dash+" time from your terminal"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "  %s %s%s%s%sunix %d\n",
		st.dim("UTC now"),
		st.bold(utc.Format(o.timeFormat(true))),
		o.dot(), utc.Format("Mon, Jan 2 2006"),
		o.dot(), utc.Unix())
	b.WriteString("\n")

	rows := [][2]string{
		{"curl " + host + "/tokyo", "current time in Tokyo"},
		{"curl " + host + "/nyc/london/tokyo", "compare cities, see overlap"},
		{"curl " + host + "/est", "time zone abbreviations work too"},
		{"curl " + host + "/utc+5:30", "any fixed UTC offset"},
		{"curl " + host + "/unix", "epoch seconds, nothing else"},
		{"curl " + host + "/utc", "UTC in RFC 3339, nothing else"},
		{"curl " + host + "/zones?q=india", "search the zone list"},
	}
	width := 0
	for _, r := range rows {
		if len(r[0]) > width {
			width = len(r[0])
		}
	}
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s   %s\n", pad(r[0], width), st.dim(r[1]))
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "  %s\n", st.dim("options: ?12 twelve-hour clock, ?plain no colors, ?format=json"))
	if o.Repo != "" {
		fmt.Fprintf(&b, "  %s\n", st.dim("open source: "+o.Repo))
	}
	b.WriteString("\n")
	return b.String()
}

// ErrorText renders a not-found message with any spelling suggestions.
func ErrorText(query string, suggestions []string, o Options) string {
	st := styler{on: !o.Plain}
	var b strings.Builder
	b.WriteString("\n")
	fmt.Fprintf(&b, "  I don't know a place called %s.\n", st.bold(query))
	if len(suggestions) > 0 {
		b.WriteString("\n")
		fmt.Fprintf(&b, "  did you mean:\n")
		for _, s := range suggestions {
			fmt.Fprintf(&b, "    curl %s/%s\n", o.BaseURL, s)
		}
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "  %s\n", st.dim("curl "+o.BaseURL+"/zones lists everything I know"))
	b.WriteString("\n")
	return b.String()
}
