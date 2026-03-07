# Contributing

## Workflow

1. Branch from the current mainline.
2. Keep changes scoped and atomic.
3. Add or update tests for behavior changes.
4. Update docs and `CHANGELOG.md` for user-visible changes.

## Local checks

Run before opening a merge request:

```bash
go vet ./...
go test -count=1 ./...
```

If `make` is available:

```bash
make ci
```

## Engineering constraints

- No external CDN/runtime online dependencies for UI assets.
- RU/EN localization is required for new UI and API user-facing strings.
- Server-side authorization checks are mandatory for new endpoints.
- Keep module boundaries clear; avoid oversized files where practical.
