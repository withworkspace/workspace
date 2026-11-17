## Goal
Stage 1: write a short proposal in the records repo describing a "hello world" program written in Python.
Stage 2: implement that Python hello world in the demo repo, following the stage 1 proposal.

## Stages
1. **Proposal (records capsule `hello-world-proposal`).** Write `proposals/hello-world-python.md` with: a title, the problem, the proposal, and what changes. The proposal covers a single Python file that prints "Hello, world!", its location in demo (`hello.py` at the repo root), and how to run it (`python3 hello.py`). Output: the proposal file, recorded with `ws mission artifact add`. Check-in: the user reviews the proposal before stage 2 starts.
2. **Implementation (demo capsule `hello-world-python`).** Add `hello.py` at the repo root as the proposal describes. Output: the code change in the demo capsule.

## Context
- Test environment; a pretend-but-real run of the staged flow.
- records README: proposals go in `proposals/<short-name>.md`, with a title, the problem, the proposal and what changes.
- demo README: "A tiny Go program for trying ws missions. Run it with `go run .`." The existing Go code (`main.go`, `main_test.go`, `go.mod`) is not part of this mission and must stay untouched. The Python file sits alongside it.

## Constraints
- Python only for the new program. No new Go code, and no changes to the existing Go files.
- Keep it minimal: one file, `hello.py`, that prints "Hello, world!" when run.
- Stage 2 must not start until the stage 1 proposal exists and the user has checked it.

## Acceptance
- Stage 1: `proposals/hello-world-python.md` in records, recorded as a mission artifact.
- Stage 2: `hello.py` in demo prints "Hello, world!" when run with `python3 hello.py`, matches the stage 1 proposal, and the existing Go tests still pass with `go test ./...`.
