# Contributing to AutoLedger

Thanks for helping. This page covers how to propose changes; the technical conventions are in [CLAUDE.md](CLAUDE.md) (layout, commands, backend and frontend rules).

## Before you start

- **Bug**: open a *Bug report* issue with steps to reproduce.
- **New feature**: open a *Feature request* issue first, so the approach can be agreed before code is written. Small fixes can go straight to a PR.
- **Question or idea**: open an issue with the `question` label.

### Out of scope

AutoLedger is a self-hosted ledger. These are not accepted; a feature that needs one goes to an issue first:

- connectors that log into a manufacturer cloud or an aggregator;
- per-operator or per-brand parsers in the core (use the generic CSV importer and import profiles);
- hard-coded lists of brands, models, maintenance plans or tire presets;
- country-specific tax engines;
- receipt OCR or AI-based extraction;
- route planning or real-time telemetry dashboards;
- a native mobile app, outbound telemetry, plugin systems, multi-tenant SaaS features;
- recomputing recorded history when a tariff or setting changes.

## Development setup

```bash
go build ./... && go vet ./... && go test ./...
cd web && npm ci --ignore-scripts && npm run typecheck && npm test && npm run build
```

Integration tests need PostgreSQL through `TEST_DATABASE_URL`; the exact container command is in CLAUDE.md. They are skipped otherwise, and CI runs them.

## Pull requests

- One topic per PR, branched from `main`. Keep it small enough to review.
- Title and description in English, with `## Summary` and `## Notes` sections (the template provides them).
- Add or update tests next to the code. Logic belongs in `web/src/utils` or `internal/services`, not in components or handlers.
- Every user-visible string exists in English and French (`web/src/locales`), and server errors use `apierror` codes. See CLAUDE.md.
- Money is integer cents on the server and currency units in JSON; distances are kilometres in the API.
- Document behaviour changes in `docs/` (English and French) and describe the current behaviour only.
- CI must pass: backend tests, frontend build, vulnerability scan, Docker build and SonarCloud. New code is held to the SonarCloud quality gate, but a test written only to raise coverage is not wanted.

## Reviews

Reviews check the change against the code and the conventions, and say which points block merging and which are suggestions. Please keep discussion on the PR; a maintainer may push small fixes to your branch or ask for a rebase.

## Language

Issues and PRs may be written in English or French; code, commits, PR titles and descriptions are in English.
