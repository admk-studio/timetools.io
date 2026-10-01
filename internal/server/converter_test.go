package server

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func converterQuery(from, to, date, clock string) string {
	return "/timezone-converter?" + url.Values{
		"from": {from}, "to": {to}, "date": {date}, "time": {clock},
	}.Encode()
}

func TestConverterPage(t *testing.T) {
	for _, path := range []string{"/timezone-converter", "/timezone-converter/"} {
		s := testServer(t)
		// Search crawlers and ordinary requests must get a useful HTML page.
		resp, body := get(t, s, path, "", "")
		if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
			t.Fatalf("converter response = %d, %s", resp.StatusCode, resp.Header.Get("Content-Type"))
		}
		for _, want := range []string{
			"<h1>Time Zone Converter</h1>", `name="from"`, `name="to"`,
			`name="date"`, `name="time"`, `value="2024-07-16"`,
			`datetime="2024-07-16T13:41:00Z"`, `datetime="2024-07-16T09:41:00-04:00"`,
			`value="Asia/Kathmandu"`, "Link to this conversion", "daylight saving",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("page missing %q", want)
			}
		}
		if strings.Contains(body, "data-clock") && strings.Contains(body, `<p class="big clock" data-clock`) {
			t.Error("converted times must not tick away from the selected instant")
		}
		if resp.Header.Get("Content-Security-Policy") == "" {
			t.Error("missing CSP")
		}
	}
	_, home := get(t, testServer(t), "/", browserUA, htmlAccept)
	if !strings.Contains(home, `href="/timezone-converter"`) {
		t.Error("homepage must link to the converter")
	}
}

func TestConverterConversions(t *testing.T) {
	for _, tt := range []struct {
		name, from, to, date, clock, suffix, target, detail string
	}{
		{"summer", "New York", "London", "2026-07-15", "09:00", "", "2026-07-15T14:00:00+01:00", "5h ahead"},
		{"winter", "New York", "London", "2026-01-15", "09:00", "", "2026-01-15T14:00:00Z", "Daylight saving is not active"},
		{"different DST dates", "New York", "London", "2026-03-15", "09:00", "", "2026-03-15T13:00:00Z", "4h ahead"},
		{"next year", "Los Angeles", "Tokyo", "2026-12-31", "16:30", "", "2027-01-01T09:30:00+09:00", "Next day"},
		{"previous day", "Tokyo", "Los Angeles", "2026-01-01", "01:30", "", "2025-12-31T08:30:00-08:00", "Previous day"},
		{"quarter hour", "UTC", "Asia/Kathmandu", "2026-07-15", "12:00", "", "2026-07-15T17:45:00+05:45", "5h 45m ahead"},
		{"fixed offset", "UTC+05:30", "UTC", "2026-07-15", "09:00", "", "2026-07-15T03:30:00Z", "5h 30m behind"},
		{"southern summer", "Sydney", "London", "2026-01-15", "09:00", "", "2026-01-14T22:00:00Z", "Previous day"},
		{"same zone", "London", "Europe/London", "2026-07-15", "09:00", "", "2026-07-15T09:00:00+01:00", "same UTC offset"},
		{"seconds", "UTC", "Tokyo", "2026-07-15", "09:00:37", "", "2026-07-15T18:00:37+09:00", "Same day"},
		{"fold first", "New York", "UTC", "2026-11-01", "01:30", "", "2026-11-01T05:30:00Z", "This time occurs twice"},
		{"fold second", "New York", "UTC", "2026-11-01", "01:30", "&occurrence=later", "2026-11-01T06:30:00Z", "This time occurs twice"},
		{"half-hour fold first", "Australia/Lord_Howe", "UTC", "2026-04-05", "01:45", "", "2026-04-04T14:45:00Z", "This time occurs twice"},
		{"half-hour fold second", "Australia/Lord_Howe", "UTC", "2026-04-05", "01:45", "&occurrence=later", "2026-04-04T15:15:00Z", "This time occurs twice"},
		{"two day rollover", "UTC-12", "UTC+14", "2026-07-15", "23:00", "", "2026-07-17T01:00:00+14:00", "2 days later"},
		{"historic second offset", "UTC", "Asia/Kathmandu", "1900-01-01", "12:00", "", "1900-01-01T12:00:00Z", "UTC+05:41:16"},
		{"leap day", "UTC", "Tokyo", "2024-02-29", "12:00", "", "2024-02-29T21:00:00+09:00", "Same day"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := get(t, testServer(t), converterQuery(tt.from, tt.to, tt.date, tt.clock)+tt.suffix, browserUA, htmlAccept)
			body = html.UnescapeString(body)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d", resp.StatusCode)
			}
			for _, want := range []string{`datetime="` + tt.target + `"`, tt.detail} {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q", want)
				}
			}
		})
	}
}

func TestConverterInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name, from, to, date, clock, suffix, message string
	}{
		{"spring gap", "New York", "UTC", "2026-03-08", "02:30", "", "does not exist"},
		{"half-hour gap", "Australia/Lord_Howe", "UTC", "2026-10-04", "02:15", "", "does not exist"},
		{"skipped date", "Pacific/Apia", "UTC", "2011-12-30", "12:00", "", "does not exist"},
		{"invalid date", "UTC", "Tokyo", "2026-02-29", "12:00", "", "valid date and time"},
		{"invalid time", "UTC", "Tokyo", "2026-07-15", "24:00", "", "valid date and time"},
		{"missing date", "UTC", "Tokyo", "", "12:00", "", "valid date and time"},
		{"missing time", "UTC", "Tokyo", "2026-07-15", "", "", "valid date and time"},
		{"year zero", "UTC", "Tokyo", "0000-01-01", "12:00", "", "valid date and time"},
		{"year overflow", "UTC", "Tokyo", "9999-12-31", "23:00", "", "outside the supported years"},
		{"unknown source", "Atlantis", "Tokyo", "2026-07-15", "12:00", "", "valid starting city"},
		{"unknown target", "UTC", "Atlantis", "2026-07-15", "12:00", "", "valid destination city"},
		{"empty source", "", "Tokyo", "2026-07-15", "12:00", "", "valid starting city"},
		{"invalid occurrence", "UTC", "Tokyo", "2026-07-15", "12:00", "&occurrence=third", "first or second occurrence"},
		{"escaped input", `<script>alert(1)</script>`, "Tokyo", "2026-07-15", "12:00", "", "valid starting city"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := get(t, testServer(t), converterQuery(tt.from, tt.to, tt.date, tt.clock)+tt.suffix, browserUA, htmlAccept)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
			if !strings.Contains(body, tt.message) || !strings.Contains(body, `role="alert"`) {
				t.Errorf("missing error %q", tt.message)
			}
			if strings.Contains(body, `id="conversion-target"`) || strings.Contains(body, "<script>alert(1)</script>") {
				t.Error("invalid input must not show a result or unescaped HTML")
			}
		})
	}
}

func TestConverterSwapAndShare(t *testing.T) {
	// Swapping into a repeated hour must retain the second occurrence.
	path := converterQuery("UTC", "New York", "2026-11-01", "06:30") + "&action=swap"
	s := testServer(t)
	_, body := get(t, s, path, browserUA, htmlAccept)
	for _, want := range []string{`name="from" value="New York"`, `value="01:30"`, `value="later" selected`, `datetime="2026-11-01T06:30:00Z"`} {
		if !strings.Contains(body, want) {
			t.Errorf("swap missing %q", want)
		}
	}
	match := regexp.MustCompile(`id="converter-share"[^>]*href="([^"]+)"`).FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatal("no share link")
	}
	share := html.UnescapeString(match[1])
	if strings.Contains(share, "action=") {
		t.Fatal("share link must freeze the result instead of repeating an action")
	}
	_, sharedBody := get(t, s, share, browserUA, htmlAccept)
	if sharedBody != body {
		t.Error("shared result does not reproduce the conversion")
	}
}

func TestConverterNow(t *testing.T) {
	s := testServer(t)
	s.now = func() time.Time { return time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC) }
	path := converterQuery("New York", "UTC", "invalid", "invalid") + "&action=now"
	resp, body := get(t, s, path, browserUA, htmlAccept)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("now status = %d", resp.StatusCode)
	}
	for _, want := range []string{`value="2026-11-01"`, `value="01:30"`, `value="later" selected`, `datetime="2026-11-01T06:30:00Z"`} {
		if !strings.Contains(body, want) {
			t.Errorf("now missing %q", want)
		}
	}
}
