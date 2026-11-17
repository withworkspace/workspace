## Goal
Add a MANUAL.md at the root of the demo repo: a short user manual for the program.

## Context
demo is a tiny Go program (README.md is one line). main.go currently just prints "hello". greetings.go holds a map of ~42 language tags to greetings, plus languages(), normalizeTag() (turns "en-GB"/"EN_gb" into "en") and greetingFor(lang) (falls back to English). Nothing in main.go calls greetingFor or languages yet.

## Constraints
- Documentation only: don't change any Go code.
- Document what the code does today, not what it might do. Don't invent flags or CLI options.
- Keep it short, and link to it from README.md.

## Acceptance
- MANUAL.md covers: running the program (`go run .`), what it prints, running tests, and how greetings/language tags work (normalization, English fallback, the supported tag list).
- README.md links to MANUAL.md.
- Everything stated in the manual matches the code.
