## Goal
Find out the current scope of the `demo` repo as committed on main, and record it as a note in the records repo, `notes/demo-scope.md`. Read-only on demo: change no code.

Scope here means what the program does on main (commit a86d200). Leave out in-flight capsules and missions (greet-by-name, hello-world-python) entirely.

## Context
- demo is at `repos/demo/.ground` (read-only). Its README says it is "a tiny Go program for trying ws missions. Run it with `go run .`". Module `example.com/demo`, Go 1.22, no dependencies.
- Files on main: `main.go` (prints "hello" via `greeting()`), `main_test.go` (checks `greeting()`), `greetings.go` (a language-tag to greeting table, with `greetingFor`, `languages` and `normalizeTag`). Check whether `main.go` calls anything in `greetings.go`. Say what is wired up and what is only defined.
- Use `git show a86d200:<file>` or read `.ground` to check facts. Do not check out or edit anything.

## Constraints
- Read-only on demo. Do not edit, commit or stash in any demo worktree.
- The note goes in the records capsule `records/demo-scope-note`, as `notes/demo-scope.md`, following the records README (`notes/<short-name>.md`: anything worth keeping that isn't a decision). Record it with `ws mission artifact add`.
- Be factual: say what the code does, cite `file:line` where useful, and say when something is unwired. No recommendations unless asked.
- Keep it short: a few sections, bullets over prose.

## Acceptance
- `notes/demo-scope.md` exists in the records capsule `demo-scope-note`, with a title, the commit it describes (a86d200), and what the program does on main: files, behaviour, tests, and what is wired up versus only defined.
- Each claim matches the code on main.
- Recorded with `ws mission artifact add`.
