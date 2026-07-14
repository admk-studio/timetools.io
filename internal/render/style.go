package render

import "strconv"

// styler wraps strings in ANSI escapes, or leaves them alone in plain
// mode. Style is applied after any padding math, so column widths never
// have to account for escape bytes.
type styler struct {
	on bool
}

func (s styler) wrap(code int, str string) string {
	if !s.on || str == "" {
		return str
	}
	return "\x1b[" + strconv.Itoa(code) + "m" + str + "\x1b[0m"
}

func (s styler) bold(str string) string   { return s.wrap(1, str) }
func (s styler) dim(str string) string    { return s.wrap(2, str) }
func (s styler) green(str string) string  { return s.wrap(32, str) }
func (s styler) yellow(str string) string { return s.wrap(33, str) }
func (s styler) cyan(str string) string   { return s.wrap(36, str) }
