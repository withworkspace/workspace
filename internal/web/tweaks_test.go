package web

import (
	"bytes"
	"strings"
	"testing"
)

// ws.tweaks is written into a frame's HTML ahead of everything of the
// page's own, and after its doctype, so the page is drawn as it was
// written to be: a script before a doctype would have it in quirks mode.
func TestWithTweaks(t *testing.T) {
	script := "<script>" + string(bytes.TrimSpace(tweaksScript)) + "</script>"
	for _, c := range []struct{ name, html, before, after string }{
		{"a doctype", "<!doctype html><title>One</title>", "<!doctype html>", "<title>One</title>"},
		{"a doctype in capitals, with a newline", "<!DOCTYPE html>\n<html lang=\"en\">", "<!DOCTYPE html>", "\n<html lang=\"en\">"},
		{"space and a comment before it", " \n<!-- a note -->\n<!doctype html><p>", " \n<!-- a note -->\n<!doctype html>", "<p>"},
		{"a byte order mark", "\xef\xbb\xbf<!doctype html><p>", "\xef\xbb\xbf<!doctype html>", "<p>"},
		{"no doctype", "<p>quirks</p>", "", "<p>quirks</p>"},
		{"a doctype that isn't first", "<p><!doctype html>", "", "<p><!doctype html>"},
		{"nothing", "", "", ""},
	} {
		if got, want := string(withTweaks([]byte(c.html))), c.before+script+c.after; got != want {
			t.Errorf("%s: %q, want %q…%q around the script", c.name, got, c.before, c.after)
		}
	}
}

// The script is written into a <script> in an HTML page, so nothing in
// it may end the element or open a comment; and it's the page's ws.tweaks.
func TestTweaksScript(t *testing.T) {
	s := strings.ToLower(string(tweaksScript))
	for _, never := range []string{"</script", "<!--"} {
		if strings.Contains(s, never) {
			t.Errorf("the script has %q, which an HTML page reads as its own", never)
		}
	}
	if !strings.Contains(s, "window.ws = ") {
		t.Error("the script doesn't define ws")
	}
}
