package release

import (
	"context"
	"errors"
	"fmt"
)

// Upgrader is what an upgrade works on: the ws that's running, and where
// its releases are. ws upgrade is one, and so is the daemon's, for the
// web UI's Settings.
type Upgrader struct {
	Exe     string // the ws that's running, as a file
	Version string // the version it says
	Release bool   // it's a release's build
	Source  Source
	OS      string
	Arch    string
}

// Upgraded is what an upgrade found or did.
type Upgraded struct {
	From, To string // the ws that was running, and the release looked at
	// Installed says To is now in place of From, which is Kept if it could
	// be. Otherwise nothing changed: To is what's running (Current), is
	// older than it (Older), or is there to install (neither).
	Installed, Kept bool
	Current, Older  bool
}

// Replaceable is whether the ws that's running is one an upgrade
// replaces: a release's build, at a path that's known.
func (u Upgrader) Replaceable() bool { return u.Release && u.Exe != "" }

// ErrNotARelease is why a ws that isn't a release's build isn't upgraded.
func (u Upgrader) ErrNotARelease() error {
	return fmt.Errorf("this ws (%s, at %s) isn't a release's build, and ws upgrade only replaces one that is: build it again where it was built (just build in its clone, or go install), or install a release with scripts/install.sh", u.Version, u.Exe)
}

// Look finds the release an upgrade would install: the one named, or the
// latest. It's not to be installed if it's what's running (Current), or
// the latest and older than what's running (Older).
func (u Upgrader) Look(ctx context.Context, version string) (Upgraded, error) {
	r := Upgraded{From: u.Version, To: version}
	switch {
	case !u.Replaceable():
		return r, u.ErrNotARelease()
	case version != "" && !IsTag(version):
		return r, fmt.Errorf("%q isn't a release's tag: they're like v1.4.0, or v1.4.0-rc.1 for a prerelease", version)
	}
	// Said before anything's fetched, for a platform with no builds.
	if _, err := Asset("", u.OS, u.Arch); err != nil {
		return r, err
	}
	if version == "" {
		latest, err := u.Source.Latest(ctx)
		if err != nil {
			return r, err
		}
		r.To = latest
		// The latest is taken only if it's later: a prerelease that's
		// installed is ahead of it, and isn't taken back.
		r.Older = Newer(u.Version, latest)
	}
	r.Current = r.To == u.Version
	return r, nil
}

// Install fetches the release Look found and puts it in place of the ws
// that's running.
func (u Upgrader) Install(ctx context.Context, r Upgraded) (Upgraded, error) {
	path, says, err := u.Source.Fetch(ctx, r.To, u.OS, u.Arch, u.Exe)
	if err != nil {
		if _, ok := errors.AsType[*Unwritable](err); ok {
			return r, fmt.Errorf("%w: ws upgrade replaces %s where it is. Run it as whoever that folder belongs to, or install a release somewhere of your own with scripts/install.sh [version] [dir]", err, u.Exe)
		}
		return r, err
	}
	if err := Install(path, u.Exe); err != nil {
		return r, fmt.Errorf("putting ws %s in place of %s: %w", says, u.Exe, err)
	}
	r.To, r.Installed = says, true
	_, err = VersionOf(PrevPath(u.Exe))
	r.Kept = err == nil
	return r, nil
}

// Upgrade installs a release in place of the ws that's running: the one
// named, or the latest if it's later than this one. With check it only
// looks.
func (u Upgrader) Upgrade(ctx context.Context, version string, check bool) (Upgraded, error) {
	r, err := u.Look(ctx, version)
	if err != nil || r.Current || r.Older || check {
		return r, err
	}
	return u.Install(ctx, r)
}
