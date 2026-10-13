# AutoLedger (module path, image, database and volume names keep the original `teslacost` identifier)

Self-hosted total-cost-of-ownership tracker for cars: energy/fuel, maintenance, tires, documents, reminders, carpooling, financing. Vehicles can be fed automatically by a [TeslaMate](https://github.com/teslamate-org/teslamate) instance through `teslamateapi`, or tracked entirely by hand.

- Backend: Go (`go.mod` pins the toolchain), chi router, pgx/pgxpool, PostgreSQL. Module path `github.com/teslacost/teslacost`.
- Frontend: Vue 3 `<script setup>` + TypeScript, Pinia, vue-router, Tailwind 4, Chart.js, Vite. Built into `web/dist` and embedded in the Go binary (`web/embed.go`).
- One Docker image serves API + SPA. `docker-compose.yml` runs `postgres`, `api` and `backup`.
- `README.md` and code comments are in English. UI strings and API error/message text are bilingual (English and French) through vue-i18n; see the Conventions section below.
- Detail behind the short rules below (errors, i18n, units, currency, TeslaMate sync, Home Assistant events) is in [docs/dev/reference.md](docs/dev/reference.md): read the matching section before changing that area.

## Commands

```bash
# Backend
go build ./... && go vet ./...
gofmt -l .                       # must print nothing (run it on files you touch; some drift)
go test ./...                    # unit tests; integration tests are skipped without TEST_DATABASE_URL
make dev                         # go run ./cmd/server (needs DATABASE_URL + secrets, see .env.example)

# Frontend (from web/)
npm ci --ignore-scripts
npm run typecheck                # vue-tsc --noEmit
npm test                         # vitest, node environment: utils/composables, plus components rendered to a string (no DOM)
npm run build                    # typecheck + vite build; the Go build embeds web/dist
```

Integration tests (services, handlers, database) need PostgreSQL. Use your own container name, other projects run containers on the same host:

```bash
docker run -d --rm --name tc-pg -e POSTGRES_USER=teslacost -e POSTGRES_PASSWORD=test \
  -e POSTGRES_DB=teslacost_test -p 55433:5432 postgres:18-alpine
# wait until a TCP connection works, pg_isready alone is not enough (psql is not installed on the host):
until docker exec -e PGPASSWORD=test tc-pg psql -h 127.0.0.1 -U teslacost -d teslacost_test -c 'select 1' >/dev/null 2>&1; do sleep 1; done
TEST_DATABASE_URL='postgres://teslacost:test@localhost:55433/teslacost_test?sslmode=disable' go test -timeout 20m ./...
docker rm -f tc-pg
```

End-to-end suite (`e2e/`, Playwright against the built SPA and a real server): start the server on a fresh database (`ENVIRONMENT=development DATABASE_URL=... AUTOLEDGER_PORT=8080 ./server`), seed it with `DEMO_PASSWORD=... go run ./cmd/demoseed -url http://127.0.0.1:8080`, then `cd e2e && npm ci --ignore-scripts && ./node_modules/.bin/playwright install chromium && BASE_URL=http://127.0.0.1:8080 DEMO_PASSWORD=... ./node_modules/.bin/playwright test`. It mocks nothing; the `End-to-end tests` CI job runs it and uploads traces on failure. Add a test there only for a flow unit tests cannot cover (a form, a redirect, a layout check).

The full run takes several minutes (`internal/services` ~5 min). A `go test` that hangs on pgxpool acquire means the database was not really up: recreate the container. CI runs `go test -race` on postgres:14, `govulncheck`, `npm audit --audit-level=high`, and SonarCloud.

## Domain notes

- Money is stored and computed in integer cents (`internal/money`); convert at the edges only.
- A vehicle has a `Powertrain` (`EV`, `ICE`, `PHEV`, `REEV`) and a server-derived `telemetry_mode` (clients never send it); `make` and `model` are free text. TeslaMate-fed features are conditioned on `hasTeslaMate` (`web/src/utils/vehicles.ts`). Details in [docs/dev/reference.md](docs/dev/reference.md#vehicle-model).
- Manual drives carry no energy cost; the server estimates their energy and the energy statistics ignore it. See [docs/dev/reference.md](docs/dev/reference.md#manual-drives).
- `cost_ledger` is the single view the TCO engine reads; charges, fuel, tolls, maintenance, insurance, tire amortization and acquisition cost all flow into it.
- A vehicle's TeslaMate connection is a `vehicle_data_sources` row, read and written only by the repository; changing what the sync reads means resetting `sync_state.full_import_completed_at` in a migration. See [docs/dev/reference.md](docs/dev/reference.md#teslamate-connection-and-sync).
- Manual charges require a cost. `UpdateCharge` overwrites notes/`document_id` with what it receives: send the existing values back when only completing a cost. Foreign currency (`fx_rate`) exists only on manual entries.
- Charging sessions pushed by Home Assistant or a script are deduplicated by `event_id`; a session without a vehicle is routed or left pending. See [docs/dev/reference.md](docs/dev/reference.md#home-assistant-charging-events).
- Sessions: 15-minute JWT access token, 30-day rotating refresh token in an `HttpOnly` cookie with reuse detection. With `ENVIRONMENT=production` the server refuses to start on published default secrets.
- `telemetry_mode` is derived by the server on every vehicle write (`Vehicle.DerivedTelemetryMode`): `CONNECTED` for an electric vehicle with a teslamateapi URL, `MANUAL` otherwise (`SEMI_AUTO` is reserved for push sources). Clients never send it. `make` and `model` are empty when not given, never filled with a default.

## Conventions

Backend
- Handlers are thin: decode, validate (`validation.go`), check ownership/role (`ownership.go`), call a repository or service, write with `response.go`. Errors returned to clients are generic; details go to logs with a subsystem prefix (`[sync]`, `[auth]`, ...).
- A user-facing error is an `*apierror.Error` with a stable dotted code, answered through `writeAPIError`/`writeErr`; each code needs an entry in `web/src/locales/{en,fr}/errors.json` or `messages.json`. Never put French text in Go source. See [docs/dev/reference.md](docs/dev/reference.md#errors-and-server-built-text).
- Text with no request to translate through (webhooks from a background job, text stored as plain text) uses `internal/servertext` with the owner's stored language.
- Every repository query that touches user data is scoped by ownership; new endpoints need an ownership test.
- API tokens (`al_live_…`, account-level, hashed in `api_tokens`) only open the integration routes, mounted in their own group with `AuthenticateIntegration` (`/api/integrations/**`: Home Assistant events, the integration view of the vehicles, metrics for sensors). Every other route uses `AuthenticateJWT`, which refuses them. A new endpoint meant for Home Assistant or scripts goes under `/api/integrations/`.
- Migrations: next number after the last file, always with a `.down.sql`, each applied in its own transaction. Do not edit an applied migration. `sonar.cpd.exclusions` already excludes `migrations/`.
- `internal/handlers/auth_handler.go` uses CRLF line endings: edit it with a tool that preserves them (Python `newline=''`), or the whole file shows as changed.
- SonarCloud coverage comes from `coverage.out` and `web/coverage/lcov.info` produced by CI. See [docs/dev/reference.md](docs/dev/reference.md#sonarcloud-coverage).
- Keep files focused (the large ones were split by responsibility; do not regrow them). New behavior gets a test next to it; integration tests use `TEST_DATABASE_URL` and skip otherwise.

Frontend
- Views orchestrate; anything with its own form or API call is a component under `components/<area>/`. Modals use `defineModel('open')`, seed their form in `watch(open)` and emit `saved`. A modal is built on `components/ModalShell.vue` (backdrop on the `z-modal` layer, header with title, optional icon and close button, scrolling body, `#footer`; `size` `sm`/`md`/`lg` (`max-w-md`/`max-w-xl`/`max-w-3xl`; a modal not yet moved onto the shell uses the same three widths, the document preview is the one wider exception), `nested` for a modal opened from another one, `#title` and `#actions` slots for a richer header): never hand-write the backdrop and panel markup.
- Put logic in `utils/*.ts` with a test, not in the component. Shared date helpers live in `utils/dates.ts`.
- Every user-visible string goes through vue-i18n (English and French): `$t('ns.key')` in templates, `t` from `@/i18n` elsewhere, both languages added together (`i18n.test.ts` checks key sets and placeholders), dates and numbers through `intlLocale()`. See [docs/dev/reference.md](docs/dev/reference.md#frontend-i18n).
- **Brand & Model Neutrality**: AutoLedger is a universal tracker for all vehicle makes (electric, hybrid, ICE). **Never hardcode lists of brands or models in the codebase (no pre-defined chip buttons, no enums, no brand lists)**. The `make` and `model` fields must always be 100% free-text inputs across the entire application (onboarding, vehicle modals, tires, etc.). Placeholders and labels must remain strictly generic across the app: use `placeholder="Marque"` / `placeholder="Make"` / `placeholder="Brand"`, `placeholder="Modèle"` / `placeholder="Model"`, `placeholder="Nom du véhicule"` / `placeholder="Vehicle name"`, and `placeholder="VIN"`. Never include brand-specific examples in placeholders (e.g. no "Tesla", "Renault", "Model 3", "Megane", "Michelin", "5YJ...", etc.). Never provide brand-biased presets (such as tire dimension presets tied to a specific manufacturer). Never branch business logic or tracking modes on brand string comparisons (e.g. never do `make === 'Tesla'`).
- Distances are stored and sent in kilometers and shown through `web/src/units.ts` (`formatDistance`, `perDistance`, `<DistanceInput>`), never a hard-coded "km"; fuel volumes are stored in litres and follow the same rule. See [docs/dev/reference.md](docs/dev/reference.md#distance-fuel-volume-and-consumption-units).
- Each vehicle has its own `Currency`, fixed at creation; a foreign amount needs an `fx_rate`, SQL never compares to a literal `'EUR'`, and amounts are formatted with `web/src/currency.ts` using the vehicle's currency. See [docs/dev/reference.md](docs/dev/reference.md#currency).
- Colour: theme tokens only, never a hex in `web/src/assets/main.css` (`var(--color-slate-800)`). Rose is the brand, the primary action and the selected option (active tab, range, filter); slate carries secondary controls and their icons; links share one hue (`sky-400`); green, amber and red mean status (good, warning, danger) and are not used to tell cost categories apart. Chart series keep the palette of their own chart. Features do not get their own accent hue (no violet, indigo, cyan or teal in components): a chip or icon is slate, or sky when it links somewhere. A date picker is themed through the `--dp-*` variables on `.dp--theme-dark` and the `dp--*` classes of the library (v14 renamed the `dp__*` ones).
- Theme: the UI is dark only (`color-scheme: dark` is forced in `main.css`). A light theme is not planned; do not add `light:` variants or a theme switch.
- Layout: pages use the full content width; the Account page is a single `max-w-3xl` column on purpose (a settings form read top to bottom). A page's primary action is one "Add" button whose label stays the same across tabs, with the type as secondary text; an empty list explains what goes there and offers the action.
- Unit tests run with the French locale (`web/vitest.setup.ts`).
- `<table class="sr-only">` does not clip: wrap it in `<div class="sr-only">`. After changing a page, check that `documentElement.scrollWidth` equals the viewport at 320/360/375/390/414 px.
- Do not pipe `vite build` through `tail` and trust the result: a failed build leaves the previous `dist`. Grep for `built|error`.

Refactors
- Splitting a file is a pure move unless stated otherwise: verify declaration hashes and the multiset of non-blank lines before and after, then `gofmt`, build, vet, tests. For Vue views, compare the rendered DOM and recorded API writes before and after with a Playwright characterization run (mock `/api/**`, fixed clock).
- SonarCloud counts moved code as new code: duplication that already existed inside a big file fails `new_duplicated_lines_density` (3 %) once the file is split. Find the blocks with `api/duplications/show?key=Rem7474_TeslaCost:<path>&pullRequest=<n>` and extract a small helper in a separate commit.

## Continuous improvement

When a change touches a component, view, handler, repository or util, look around it before finishing and propose what is worth doing, instead of only patching the one spot:

- a refactor of the part being edited (a function or file that has outgrown its role, a branch that repeats);
- a shared component, helper or composable when the same markup or logic already exists, or is about to exist, in two or more places (search for the pattern first: `grep` the other views and handlers);
- an adjacent improvement the change makes cheap (a missing test, an untranslated string, a hard-coded unit or currency, a missing empty/loading/error state, an accessibility or mobile-width gap, a stale doc).

How to propose:
- Say it in the end-of-task summary or the PR `## Notes`, one line each: what, where, why, rough size. Mention what was found, not only what was done.
- Do the small, obviously safe ones in the same PR only when they sit in code already being changed and add no review burden; anything larger or in a different area goes to its own commit, its own PR, or an issue (link it from the roadmap issue when one exists).
- Never widen a PR silently: a pure move stays a pure move (see Refactors), and a behaviour change is never hidden inside a refactor.
- Do not invent work: if nothing is worth proposing, say nothing. A proposal needs a concrete duplication, defect or gap to point at.

## Out of scope

AutoLedger is a self-hosted ledger: it records what the user enters or pushes and computes costs from it. Do not build:

- Connectors that log into a manufacturer cloud, or that go through an aggregator (Enode, Smartcar, scrapers). Data comes from TeslaMate, CSV, the ingestion API, Home Assistant or manual entry.
- Per-operator or per-brand parsers in the core (charging network exports, manufacturer CSV layouts). Mapping is done by the generic CSV importer and saved import profiles; a layout belongs in documentation or a shared profile.
- Hard-coded lists of brands, models, manufacturer maintenance plans or tire presets (see Brand & Model Neutrality). Maintenance intervals are user-defined reminders.
- Country-specific tax engines in the code (mileage allowance scales, benefit-in-kind rules). Exports with the pro/personal split by tag are in scope; rate tables are data, not logic.
- Receipt OCR or any AI-based extraction.
- Route planning, live navigation, or real-time telemetry dashboards that compete with TeslaMate/Grafana.
- A native mobile app: the PWA is the mobile client.
- Outbound telemetry or analytics. Product metrics, if any, are computed locally.
- A generic plugin or scripting system, and multi-tenant SaaS features (billing, organisations).
- Recomputing recorded history when a tariff, rate or setting changes: stored costs stay as entered.

A feature that needs one of these goes to an issue for discussion first.

## Git and PRs

- Check `gh pr list --state all` before touching a branch that had a PR: PRs are merged quickly and a merged branch must not be reused. Follow-up work goes on a fresh branch from `origin/main`, one PR per topic, independent PRs rather than stacks unless the work truly depends on the previous one.
- Commit messages and PR titles/bodies are in English. PR body: `## Summary` and `## Notes` sections. `gh pr edit` fails on the deprecated Projects-classic GraphQL field; update a body with `gh api -X PATCH repos/Rem7474/TeslaCost/pulls/<n> -F body=@file`.
- No AI attribution anywhere on git or GitHub: no "Generated with Claude Code" line and no `Co-Authored-By` trailer in commit messages, PR bodies, comments or issues.
- Do not push local `feat/*` branches left over from merged PRs.
- Review comments on external contributors' PRs are written in English and stay constructive: start with what is correct (checked against the code), phrase concerns as observations or questions with a concrete suggestion, label what is blocking and what is not, never imply fault, and say what happens next (merge, rebase, re-run). A failing check is investigated before it is reported, and a transient failure is re-run rather than put on the contributor.
- `README.md` and `README.fr.md` keep the same heading structure (level and leading emoji), checked by `internal/readme`; edit both together.
- Documentation states the current behavior; it does not narrate history ("now", "again", "re-introduced") or cite PR numbers. That belongs in commit messages.

## Releases

A release is a `vX.Y.Z` tag on a commit whose `ci.yml` run is green (new feature: minor bump, fixes only: patch). `release.yml` publishes the image and creates the GitHub Release with generated notes, then a `chore: sync version files to X.Y.Z` PR updates `cmd/server/main.go`, `web/package.json`, `web/package-lock.json` (its two version lines) and `web/src/version.ts`.

Replace the generated list of PR titles with written release notes (`gh release edit vX.Y.Z --notes-file`), in English, for a user of the app rather than a contributor:

- A one or two sentence summary of the release first.
- `### New features`: one bullet per feature, saying what the user can now do, not how it is built.
- `### Improvements`: changes to existing behaviour, performance, UI and site.
- `### Bug fixes`: one bullet per fix, saying what was wrong from the user's side.
- `### Documentation` and `### Maintenance` (dependencies, CI, version sync) only when there is something to say; omit an empty section.
- Related PRs are merged into one bullet (a feature and its follow-up fixes read as one line), each bullet ends with its PR numbers `(#123, #124)`, and the version-sync PR is not listed.
- End with the `**Full Changelog**` compare link of the generated notes.

## Browser checks

Playwright and Chromium are available on the dev host (`PLAYWRIGHT_BROWSERS_PATH=/opt/pw-browsers` in the remote environment). Run `vite preview` bound to 127.0.0.1, mock `/api/**` with `context.route`, and never `pkill -f` a pattern that also appears in your own command line.
