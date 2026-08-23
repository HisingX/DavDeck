# Contributing

Thank you for contributing to DavDeck.

## Before coding

Read:

- `README.md` and the relevant user guide
- `docs/PROJECT_SPEC.md`
- `docs/ARCHITECTURE.md`
- relevant public technical documentation

## Contribution principles

- Keep changes focused.
- Preserve architecture boundaries.
- Add tests for behavior changes.
- Use migrations for schema changes.
- Do not weaken security to simplify implementation.
- Do not introduce roadmap features without prior scope agreement.
- Keep Linux headless support in mind for backend features.

## Pull requests

A PR should explain:

- what changed
- why it changed
- architecture/security implications
- tests added
- tests actually run
- known limitations

## Commit style

Use English Conventional Commit prefixes:

```text
feat: add share permission model
fix: preserve the last working runtime
docs: clarify Linux headless setup
build: update release tooling
chore: refresh test fixtures
```

Example:

Keep commit messages concise and describe the primary change.

## Architecture changes

Significant architectural changes require an ADR under `docs/adr/` before or with implementation.
