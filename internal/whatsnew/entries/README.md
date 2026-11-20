# What's new

What ws says is new after someone upgrades. It's shown once, in a dialog,
and stays on Settings › About.

Nothing goes here by itself. An entry is something worth stopping someone
for: a new place, a new thing they can do, a change to how something they
use works. Not a fix, a refactor, or a change they'd never notice. Most
pull requests have no entry, and a release with none shows nothing.

## Writing one

One file, named for what it's about, in the pull request that makes it:

```
---
title: Plugins are in Settings
link: /settings/plugins
picture: plugins.png
---
Add, update and remove plugins from **Settings › Plugins**, without the
terminal.
```

- `title` (required): at most 60 characters. Say what's new, not the feature's name.
- The lines under the head (required): a sentence or two, at most 320 characters, in markdown.
- `link`: where in the web UI to see it, as a path. The dialog has a button to it.
- `picture`: a `.png`, `.jpg` or `.webp` file beside the entry. It isn't put in the binary: it's published with the release, and fetched when it's shown.

Crop a picture to what's new, not the page it's on: a whole page shrinks to
nothing in the dialog. Take it at 2× and keep it under 100 KB (`.webp`). The
first entry's picture is shown 4:3, beside its words, so crop that one near
4:3; the others' keep their own shape, up to 240px tall. Entries are shown
newest release first and by file name within one, so the first entry is the
release's first by name.

To see it as it'll read, `just try up` and open **Settings › About › Show
what's new**: a build of a clone shows every entry, with its picture from
this folder.

Don't say which release it's in. A build works that out from the tags
(`scripts/whatsnew`): an entry is in the first release that has the commit
that added it. `go run ./scripts/whatsnew -check` reads every entry, and
`just check` runs it.

A release's entries are also the top of its notes on GitHub, above the
list of pull requests.
