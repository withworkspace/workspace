# Capsule files

Status: built on `v2`, 2026-11-10.

A capsule's files in the web UI, as they are now, without an editor open: what crew have written, read where it lives, with what's changed since the capsule left its base branch marked.

## Where

`/missions/<slug>/files/<repo>/<name>`, with `?path=` for the open file and `?only=changed` for the filter. It opens from:

- a capsule's **Files** button on the mission's Work tab;
- `f` on the mission page (the capsule under the cursor, or the mission's only one);
- **Browse files** on a review;
- a capsule's hover card;
- ⌘K ("Files" items, one per capsule).

## What it shows

- **The tree**, left: every file in the working tree, tracked and untracked, ignored files left out. A folder holding only a folder shows as one row (`internal/web`). Each file carries its change since the branch point: added, modified, deleted, renamed. It gets a dot when it isn't committed. A closed folder with changes in it has a dot too.
- **Changed / All**: only the files that differ from the branch point (committed or not, untracked included), with every folder open; or everything.
- **The file**, right: highlighted, numbered, with the lines that changed since the branch point marked in the gutter (added, changed, and a rule where lines were removed). A line number links to the line (`#L12`; shift-click for `#L12-L20`).
  - Markdown renders, with a switch to its text.
  - Images show.
  - A deleted file shows as it was at the branch point.
  - Binary and very large files (over 2 MB) say so.
- **Finding**: `t` or ⌘P opens a fuzzy finder over every path. The letters you type match in order, scored higher when they start a word or a path part, or fall in the file's name. Matches are marked. Changed files come first before you type, and win ties after.
- **Keys**: `j`/`k` and `h`/`l` move through the tree, `c` toggles the filter, `[`/`]` step through changed files, `m` switches markdown, `v` reviews, `y` copies the path, `r` reloads.
- It reloads when the mission changes, so it follows crew as they work.

## How

- `GET .../capsules/{repo}/{name}/tree` (`review.ReadTree`): `git ls-files --cached --others --exclude-standard`, then `git diff --name-status -M <branch point>` against the working tree, and `git status` for what isn't committed. Up to 20,000 files.
- `GET .../file?path=` (`review.ReadFile`): the file from the working tree, with `git diff -U0 <branch point> -- path` read as the changed stretches.
- `GET .../raw?path=`: images only, sandboxed.
- Paths are resolved inside the capsule. `..`, absolute paths and links that lead out are refused (`review.ResolvePath`).

The branch point is the merge base of `HEAD` and `origin/<default branch>`, as reviews use. When that isn't fetched, nothing's marked changed and the filter is off.

## Not yet

- Search inside files (a `git grep` over the capsule).
- Blame, and a file's history.
- Comments on a file's lines from here: reviews comment on diffs, and this isn't one.
