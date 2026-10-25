# AGENTS.md

Rules for anyone (human or agent) changing this repo.

## General

- A question is not a request for action: if the owner asked something,
  answer and discuss — don't edit code, files, or git history until told to.

## Build and test

- `go build ./... && go vet ./... && go test ./...` must pass before committing.
- testify: `assert` for checks, `require` for preconditions.
- Prefer property tests (rapid) over small unit tests; unit tests only to pin
  a hand-computed number, anchor a shared definition, or lock a regression.

## Working rules

- The in-memory index is the oracle: every disk structure, merge, or
  optimisation must match it on random input. Write the oracle test first.
- Hand-computed expectations for anything numeric — a test asserting
  "whatever the code returned" proves nothing.
- No optimisation without a benchmark (`go test -bench` next to the code,
  `bench/` for macro).
- Version every on-disk format and keep `docs/formats.md` in sync. Pre-1.0
  formats may break, but old ones must be detected, not misread.
- Keep the public API tiny until there is a second user.
- Concept unclear? Look at the corresponding Tantivy code first, then write
  a 20-line test on three documents, then implement.

## Code style

- Inline code beats small helpers; extract a function only when it is
  actually used in several places.

## Comments

- No comments by default — including package-level and public-API docs.
  Prefer clear names and straightforward code.
- Comment only non-obvious behavior or invariants that naming and structure
  can't express. Short; why, not how.
- Update or delete comments in the same edit that changes the code.
- No diff narration, no section banners, no commented-out code.

## Commit messages

Conventional Commits, one concept per commit: `<type>(<scope>): <imperative
subject, ≤72 chars>`.

- Types: `feat`, `fix`, `perf`, `refactor`, `test`, `docs`, `build`
  (deps/tooling), `chore` (last resort).
- Scope: the package's leaf directory name (`index`, `parser`); full path
  only on a leaf-name collision. Optional — omit when the subject names the
  component or the change spans packages. Never invent scopes.
- Body: only when the *why* isn't recoverable from the diff.

Examples: `feat(schema): add Document and Value types` (subject doesn't say
where), `feat: add MemSegment with posting lists` (subject names it).
