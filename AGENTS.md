# AGENTS.md

Rules for anyone (human or agent) changing this repo.

## Build and test

- `go build ./... && go vet ./... && go test ./...` must pass before committing.
- testify: `assert` for checks, `require` for preconditions.
- Prefer property tests (rapid) over small unit tests; unit tests only to pin
  a hand-computed number, anchor a shared definition, or lock a regression.

## Working rules

- Hand-computed expectations for anything numeric — a test asserting
  "whatever the code returned" proves nothing.
- No optimisation without a benchmark (`go test -bench` next to the code,
  `bench/` for macro).

## Code style

- Inline code beats small helpers; extract a function only when it is
  actually used in several places.
- Use blank lines sparingly to separate logical blocks; a single blank line is
  enough (gofmt collapses consecutive blank lines). Do not pad the start or end
  of blocks, split a call from its immediate error check, or add blank lines to
  short, linear code where they do not improve readability. In tests, separate
  distinct arrange, act, and assert sections; very short tests need no extra
  spacing.

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
