// Package slug makes names fit for a folder, a branch or an id out of
// what people write: a mission's title, a document's, a person's name.
package slug

import "strings"

// Make is s in lowercase letters and digits, each run of anything else a
// single hyphen, none at either end. Longer than max (0 for no limit), it's
// cut at a hyphen where there's one to cut at. fallback is for an s with
// no letters or digits at all.
func Make(s string, max int, fallback string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		default:
			dash = true
		}
	}
	out := b.String()
	if max > 0 && len(out) > max {
		cut := out[:max]
		if out[max] != '-' {
			if i := strings.LastIndexByte(cut, '-'); i > 0 {
				cut = cut[:i]
			}
		}
		out = strings.TrimRight(cut, "-")
	}
	if out == "" {
		return fallback
	}
	return out
}
