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
- `picture`: a `.png`, `.jpg` or `.webp` file beside the entry. It isn't put in the binary: it's published with the release, and fetched when it's shown. Keep it small (a cropped `just shots` picture, 2× at most).

Don't say which release it's in. A build works that out from the tags
(`scripts/whatsnew`): an entry is in the first release that has the commit
that added it. `go run ./scripts/whatsnew -check` reads every entry, and
`just check` runs it.

A release's entries are also the top of its notes on GitHub, above the
list of pull requests.
