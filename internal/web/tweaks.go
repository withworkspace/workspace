package web

import (
	"bytes"
	_ "embed"
	"regexp"
)

// Tweaks: what an artifact's page says can be changed about it (a
// density, a toggle, a radius), for ws to draw the controls beside the
// frame. The page declares them with ws.tweaks, which ws writes into
// every frame it serves, ahead of the page's own scripts: the frame
// fetches nothing, so the page can't load it. The script (tweaks.js) reads
// the declaration, writes each tweak's value on <html> for the page's
// styles (data-name, and --name for a number or a colour), and tells the
// page around it over postMessage: what's declared, and the state as it
// changes. The web UI (web/src/artifacts) draws the tray from that, and
// says back what to set. A frame's state is in its address (?t.name=value,
// which the script reads), so one opened alone opens as it was.
//
// The server reads none of it: a version's file is as it was written, and
// what the page declares is known only by running it.

//go:embed tweaks.js
var tweaksScript []byte

// frameStart is where a frame's own HTML begins: after its doctype, with
// whatever comes before it, so the page isn't drawn in quirks mode for
// what's written in ahead of it.
var frameStart = regexp.MustCompile(`(?is)^\x{FEFF}?\s*(?:<!--.*?-->\s*)*<!doctype[^>]*>`)

// withTweaks is a frame's HTML with ws.tweaks written into it, ahead of
// everything of the page's own: after its doctype, or at its top where it
// has none. A script there is in its head, as the parser reads it, and
// what the page says of <html> and <head> after it still holds.
func withTweaks(html []byte) []byte {
	at := len(frameStart.Find(html))
	out := make([]byte, 0, len(html)+len(tweaksScript)+32)
	out = append(out, html[:at]...)
	out = append(out, "<script>"...)
	out = append(out, bytes.TrimSpace(tweaksScript)...)
	out = append(out, "</script>"...)
	return append(out, html[at:]...)
}
