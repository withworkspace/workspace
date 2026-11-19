package artifact

import (
	"fmt"
	"strings"
	"time"
)

// Artifacts are sketches, and most should go. A live one expires
// ExpireAfter ([artifacts] expire_after, 30 days unless it says) after
// its last version, or after it was put back, if that's later: a new
// version starts the count again, and promoting ends it. Expiry isn't
// written down: it's worked out from the record and the setting, so
// changing the setting changes it for every artifact.
//
// The daemon discards an expired one itself, "expired" its reason. Only
// a live one expires, and a live one is on this machine only, so nothing
// of it is in a repo for ws to take away.

// Warning is how long before it expires an artifact is listed as
// expiring: its last week.
const Warning = 7 * 24 * time.Hour

// KeepDiscarded is how long a discarded folder is kept in
// .ws/artifacts/.discarded, to restore it, before the daemon deletes it
// for good.
const KeepDiscarded = 30 * 24 * time.Hour

// Expires is when artifact e expires, and false where it doesn't: it's
// promoted or discarded, its folder has gone, or the workspace says they
// never do.
func (c Config) Expires(e Entry) (time.Time, bool) {
	if c.ExpireAfter <= 0 || e.Status != Live || e.Gone || e.Away() {
		return time.Time{}, false
	}
	return e.Since().Add(c.ExpireAfter), true
}

// Expired reports whether artifact e has expired by now.
func (c Config) Expired(e Entry, now time.Time) bool {
	at, ok := c.Expires(e)
	return ok && !now.Before(at)
}

// Expiring reports whether artifact e expires within the week, and
// hasn't yet: the list warns of it.
func (c Config) Expiring(e Entry, now time.Time) bool {
	at, ok := c.Expires(e)
	return ok && now.Before(at) && at.Sub(now) <= Warning
}

// ExpiresWords says when it expires, as the list does: "expires in 4
// days", "expires today", "expired".
func ExpiresWords(at, now time.Time) string {
	if !now.Before(at) {
		return "expired"
	}
	days := int(at.Sub(now).Hours() / 24)
	switch days {
	case 0:
		return "expires today"
	case 1:
		return "expires tomorrow"
	}
	return fmt.Sprintf("expires in %d days", days)
}

// ExpiryWords says how long a live one lasts, as [artifacts] expire_after
// does: "30 days", or "never".
func (c Config) ExpiryWords() string {
	if c.ExpireAfter <= 0 {
		return "never"
	}
	days := int(c.ExpireAfter.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

// --- The commit that takes one out of the repo it lives in

// DiscardBody is the body of the commit that takes a discarded
// artifact's folder out of the repo it lived in: who discarded it, why,
// and what it was.
func DiscardBody(by, reason string, a *Artifact) string {
	var b strings.Builder
	if reason = strings.TrimSpace(reason); reason != "" {
		fmt.Fprintf(&b, "Discarded by %s: %s", by, oneLine(reason))
	} else {
		fmt.Fprintf(&b, "Discarded by %s, with no reason given.", by)
	}
	fmt.Fprintf(&b, "\n\n%s. Its versions are on the machine it was discarded on for %d days, where ws artifact restore puts it back, here too.", what(a), int(KeepDiscarded.Hours()/24))
	return b.String()
}

// what says what an artifact was, for a commit's body: "Horizon glow", v1
// to v3, the last added 2026-10-14.
func what(a *Artifact) string {
	latest, _ := a.Latest()
	vs := "v1"
	if latest.N > 1 {
		vs = fmt.Sprintf("v1 to v%d", latest.N)
	}
	return fmt.Sprintf("%q, %s, the last added %s", a.Title, vs, a.LastAt().Local().Format("2006-01-02"))
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
