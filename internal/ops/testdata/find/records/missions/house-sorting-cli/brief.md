## Goal
Add a CLI to the demo Go program: given a name, print which Hogwarts house (Gryffindor, Hufflepuff, Ravenclaw, Slytherin) it belongs to. The result is deterministic: the same name always gives the same house.

## Context
The demo repo is a tiny Go program (main.go, greetings.go, main_test.go). Run with `go run .`. Currently it only prints a greeting.

## Constraints
- Standard library only.
- Hash the normalised name (trimmed, case-insensitive) with a stable hash such as sha256 or fnv, then take it modulo 4 to pick the house. Do not use math/rand or anything seeded by time.
- Keep the sorting logic in its own function, separate from CLI parsing, so it is easy to test.
- Name is taken from the first argument, or from stdin if none is given. No name prints usage and exits non-zero.

## Acceptance
- `go run . sort "Harry Potter"` prints a house, and gives the same house on every run.
- "harry potter" and " Harry Potter " give the same result as "Harry Potter".
- Tests cover determinism, normalisation, and that all outputs are one of the four houses.
- The README documents the usage.
