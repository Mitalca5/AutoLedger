# AutoLedger (repository and technical names: TeslaCost)

Self-hosted total-cost-of-ownership tracker for cars: energy/fuel, maintenance, tires, documents, reminders, carpooling, financing. Vehicles can be fed automatically by a [TeslaMate](https://github.com/teslamate-org/teslamate) instance through `teslamateapi`, or tracked entirely by hand.

- Backend: Go (`go.mod` pins the toolchain), chi router, pgx/pgxpool, PostgreSQL. Module path `github.com/teslacost/teslacost`.
- Frontend: Vue 3 `<script setup>` + TypeScript, Pinia, vue-router, Tailwind 4, Chart.js, Vite. Built into `web/dist` and embedded in the Go binary (`web/embed.go`).
- One Docker image serves API + SPA. `docker-compose.yml` runs `postgres`, `api` and `backup`.
- `README.md` and code comments are in English. UI strings and API error/message text are bilingual (English and French) through vue-i18n; see the Conventions section below.

## Commands

```bash
# Backend
go build ./... && go vet ./...
gofmt -l .                       # must print nothing (run it on files you touch; some drift)
go test ./...                    # unit tests; integration tests are skipped without TEST_DATABASE_URL
make dev                         # go run ./cmd/server/main.go (needs DATABASE_URL + secrets, see .env.example)

# Frontend (from web/)
npm ci --ignore-scripts
npm run typecheck                # vue-tsc --noEmit
npm test                         # vitest, node environment: utils/composables only, no DOM
npm run build                    # typecheck + vite build; the Go build embeds web/dist
```

Integration tests (services, handlers, database) need PostgreSQL. Use your own container name, other projects run containers on the same host:

```bash
docker run -d --rm --name tc-pg -e POSTGRES_USER=teslacost -e POSTGRES_PASSWORD=test \
  -e POSTGRES_DB=teslacost_test -p 55433:5432 postgres:16-alpine
# wait until a TCP connection works, pg_isready alone is not enough (psql is not installed on the host):
until docker exec -e PGPASSWORD=test tc-pg psql -h 127.0.0.1 -U teslacost -d teslacost_test -c 'select 1' >/dev/null 2>&1; do sleep 1; done
TEST_DATABASE_URL='postgres://teslacost:test@localhost:55433/teslacost_test?sslmode=disable' go test -timeout 20m ./...
docker rm -f tc-pg
```

The full run takes several minutes (`internal/services` ~5 min). A `go test` that hangs on pgxpool acquire means the database was not really up: recreate the container. CI runs `go test -race` on postgres:14, `govulncheck`, `npm audit --audit-level=high`, and SonarCloud.

## Layout

```
cmd/server/main.go          wiring, chi routes, background workers
internal/config             env-var loading and validation (Config)
internal/auth               JWT, bcrypt, OIDC client, login throttle
internal/middleware         auth, client IP / trusted proxies, security headers, Origin check
internal/crypto             AES-256-GCM for stored credentials
internal/models             plain structs and request types, grouped by domain (models_vehicle.go, ...)
internal/database           pgx pool, embedded migrations runner, one repository_*.go per domain
internal/handlers           HTTP handlers, one file per resource
internal/services           TCO engine (tco_*.go), sync (sync_*.go), energy stats, carpool, comparison, reminders/notifications, toll detection
internal/teslamate          HTTP client for teslamateapi
internal/storage            document files on the volume
internal/money, tolldata    integer-cent helpers, toll reference data
migrations/                 NNNNNN_name.{up,down}.sql, embedded via migrations/embed.go
web/src/views               one orchestration view per route (kept thin)
web/src/components/<area>   modals, panels and cards extracted from the views
web/src/utils               pure logic + colocated *.test.ts (this is what vitest covers)
web/src/composables         shared reactive logic (document preview/attach, selection, confirm)
web/src/stores              Pinia: auth, vehicle, offline queue, quick-add
web/src/services            api.ts (fetch wrapper, refresh handling), offlineQueue.ts (IndexedDB)
backup/                     sidecar image: pg_dump + documents archive on a schedule
```

## Domain notes

- Money is stored and computed in integer cents (`internal/money`); convert at the edges only.
- A vehicle has a `Powertrain` (`EV` or `ICE`) and a `telemetry_mode` (`CONNECTED`, `SEMI_AUTO`, `MANUAL`). Vehicles also carry optional free-form `make` and `model` text attributes (`VARCHAR(100)`). ICE vehicles are tracked through fuel logs and have no TeslaMate link. EV vehicles may or may not have a TeslaMate connection (`teslamate_api_url`, `teslamate_car_id`, credentials stored encrypted) or can be tracked semi-automatically (CSV imports) or fully declaratively. TeslaMate-fed features (continuous telemetry sync, battery and temperature panels, sync button) are conditioned on `hasTeslaMate` (`web/src/utils/vehicles.ts`, exposed by the vehicle store): electric and a teslamateapi URL set. Non-Tesla and manual EV vehicles can log drives, import CSV files, and track expenses without requiring TeslaMate.
- `cost_ledger` is the single view the TCO engine reads; charges, fuel, tolls, maintenance, insurance, tire amortization and acquisition cost all flow into it.
- TeslaMate sync (`internal/services/sync_*.go`): per-vehicle background jobs, resumable full import, a 30-day sliding re-read, reconciliation of deleted drives/charges, and a per-vehicle circuit breaker. Changing what is read from TeslaMate means resetting `sync_state.full_import_completed_at` in a migration so the next sync re-reads the history.
- teslamateapi facts (verified in its source, its README has no field docs): drives/charges carry `battery_details{start_battery_level,end_battery_level}` and `outside_temp_avg` in `units.unit_of_temperature` (F is possible); `charge_energy_used` is `GREATEST(used, added)`, so used == added means "not measured" (DC); missing levels arrive as 0; `/battery-health` reports current capacity over the best capacity ever observed.
- Manual charges require a cost. `UpdateCharge` overwrites notes/`document_id` with what it receives: send the existing values back when only completing a cost. Foreign currency (`fx_rate`) exists only on manual entries.
- Sessions: 15-minute JWT access token, 30-day rotating refresh token in an `HttpOnly` cookie with reuse detection. With `ENVIRONMENT=production` the server refuses to start on published default secrets.

## Conventions

Backend
- Handlers are thin: decode, validate (`validation.go`), check ownership/role (`ownership.go`), call a repository or service, write with `response.go`. Errors returned to clients are generic; details go to logs with a subsystem prefix (`[sync]`, `[auth]`, ...).
- A user-facing error is an `*apierror.Error` (`internal/apierror`): a stable dotted code (`vehicle.not_found`), an English message and, for formatted messages (`apierror.Newf`), the values as `p0`, `p1`, ... in verb order. Answer with `writeAPIError`, or `writeErr` for an error that may carry one; the JSON body is `{error, code, params}`. Non-failure server-built text (sync warnings, comparison assumptions, tire wear explanations) uses the same shape through `apierror.Message` (`= Error`) and `NewMessage`/`NewMessagef`, embedded as a struct field and serialized as `{code, message, params}`. Each code needs an entry in `web/src/locales/{en,fr}/errors.json` (failures) or `messages.json` (non-failures) — a front-end test lists the codes used in the Go source — with `{p0}` placeholders. Internal failures use the code `internal` and log the cause. Never put French text in Go source.
- Text with no HTTP request to translate through (reminder and sync-failure webhooks, sent from a background job) or that ends up stored as plain text rather than a code (auto-generated toll notes, a trip's default name) uses `internal/servertext` instead: `servertext.Text(lang, "reminder.discord_title", args...)`, catalog and both languages kept in one file. `lang` is the acting/owning user's stored `Language` ("en"/"fr", column added by `000032_user_language`); resolve it with `requestLanguage(r, repo)` in a handler or a small local `language(ctx, userID)` method in a service that already holds `*database.Repository`. The UI's own language stays client-side (`web/src/i18n`, `localStorage`); `PUT /api/auth/language` only updates this stored copy, called by `LanguageSwitcher.vue` and once after registration.
- Every repository query that touches user data is scoped by ownership; new endpoints need an ownership test.
- API tokens (`al_live_…`, account-level, hashed in `api_tokens`) only open the integration routes, mounted in their own group with `AuthenticateIntegration` (`/api/integrations/**`: Home Assistant events, metrics for sensors). Every other route uses `AuthenticateJWT`, which refuses them. A new endpoint meant for Home Assistant or scripts goes under `/api/integrations/`.
- Migrations: next number after the last file, always with a `.down.sql`, each applied in its own transaction. Do not edit an applied migration. `sonar.cpd.exclusions` already excludes `migrations/`.
- `internal/handlers/auth_handler.go` uses CRLF line endings: edit it with a tool that preserves them (Python `newline=''`), or the whole file shows as changed.
- Keep files focused (the large ones were split by responsibility; do not regrow them). New behavior gets a test next to it; integration tests use `TEST_DATABASE_URL` and skip otherwise.

Frontend
- Views orchestrate; anything with its own form or API call is a component under `components/<area>/`. Modals use `defineModel('open')`, seed their form in `watch(open)` and emit `saved`.
- Put logic in `utils/*.ts` with a test, not in the component. Shared date helpers live in `utils/dates.ts`.
- Every user-visible string goes through vue-i18n (English and French). Catalogs are `web/src/locales/<lang>/<namespace>.json`; the message key is `<namespace>.<path>`. Templates use `$t('ns.key', { param })`, scripts, stores and utils use `t` from `@/i18n` (call it while rendering so it follows the language). Plurals use the `singular | plural` form with `t(key, count)`. Escape `@ | { } $` in a message as `{'@'}`. Add a key to both languages: `i18n.test.ts` fails when the key sets or placeholders differ. Shared wording (`Cancel`, `Save`, ...) lives in the `common` namespace. A word that sits between two values ("at 45,210 km", "in September", "From"/"To") is part of the message too, with the values as placeholders; when one value needs its own markup, use `<i18n-t keypath="...">` with a named slot rather than splitting the sentence. Input examples use `common.example` (`ex : 850` / `e.g. 850`), decimals through `$n()`. Dates and numbers use `intlLocale()`, never a hard-coded `fr-FR`. The product name (AutoLedger) is `APP_NAME` in `web/src/brand.ts`; the module path, image, database and volume names keep the original `teslacost` identifiers.
- **Brand & Model Neutrality**: AutoLedger is a universal tracker for all vehicle makes (electric, hybrid, ICE). **Never hardcode lists of brands or models in the codebase (no pre-defined chip buttons, no enums, no brand lists)**. The `make` and `model` fields must always be 100% free-text inputs across the entire application (onboarding, vehicle modals, tires, etc.). Placeholders and labels must remain strictly generic across the app: use `placeholder="Marque"` / `placeholder="Make"` / `placeholder="Brand"`, `placeholder="Modèle"` / `placeholder="Model"`, `placeholder="Nom du véhicule"` / `placeholder="Vehicle name"`, and `placeholder="VIN"`. Never include brand-specific examples in placeholders (e.g. no "Tesla", "Renault", "Model 3", "Megane", "Michelin", "5YJ...", etc.). Never provide brand-biased presets (such as tire dimension presets tied to a specific manufacturer). Never branch business logic or tracking modes on brand string comparisons (e.g. never do `make === 'Tesla'`).
- Distances are always stored, computed and sent to the API in kilometers; `web/src/units.ts` converts for display (`formatDistance(km)`) and form input (`displayDistanceToKm(value)`) based on the signed-in account's stored `distance_unit` ("km"/"mi", column added by `000033_user_distance_unit`, set with `PUT /api/auth/distance-unit`). New UI that shows or accepts a distance goes through it rather than hard-coding "km": `formatDistance`/`formatDistanceValue` for a distance, `perDistance`/`formatPerDistanceValue` for a figure per km or per 100 km (a cost per km, kWh/100 km: per mile it is 1.609 times larger), `speedUnit`/`formatSpeed` for a speed, and `<DistanceInput>` (`web/src/components/DistanceInput.vue`) for a form field, which keeps its model in km (`kind="per-distance"` for a figure per km, `whole` when the API field is an integer, `text` when the form keeps the field as a string). Catalog strings take the unit through `{unit}` (`"Odometer ({unit})"`, `"{cost}/{unit}"`) and `i18n.test.ts` fails when a call forgets `unit` (or `cur`, `speed`, `min`, `band`) for a message that needs it. Figures per km that the API computes (`cost_per_km`, `consumption_kwh_100km`) are converted on the frontend with `perDistance`; the API itself stays metric. Litres stay litres; L/100 km follows the distance unit like any figure per distance. A distance inside an API error or message is passed to `apierror.Newf` as `apierror.Km(v)` (a figure per km as `apierror.PerKm(v)`): the English text formats it like a float, the parameter reaches the client as `"km:<v>"` / `"perkm:<v>"`, and `apiErrorMessage` converts it and fills `{unit}`. Server-built text with no request (reminder webhooks) uses `servertext.Distance(unit, km)` with the owner's stored unit.
- Each vehicle has its own `Currency` (ISO 4217, `vehicles.currency`, migration `000034_vehicle_currency`), fixed at creation and never edited afterward — `UpdateVehicle`'s SQL simply has no `currency` column in its `SET`, so a value sent on update is silently ignored; existing (pre-migration) vehicles keep `EUR`. A stored money amount in another currency needs an `fx_rate` to the vehicle's own currency, same mechanism as before but relative to the vehicle instead of hard-coded to the euro (`normalizeCurrency(currency, baseCurrency, fxRate)` in `internal/handlers/validation.go`; SQL sites compare `currency = (SELECT currency FROM vehicles WHERE id = ...)` or join `vehicles v` and compare `v.currency`, never a literal `'EUR'` — `amountInVehicleCurrencyExpr` in `internal/database/repository_drives.go` is the shared helper). The toll dataset (French autoroutes, OpenTollData) is the one amount that stays euro-denominated regardless of the vehicle's currency. On the frontend, `web/src/currency.ts`'s `formatMoney(cents, currency)` / `formatAmount(amount, currency)` use `Intl.NumberFormat` (`currencySymbol(currency)` for a bare symbol, e.g. a chart axis); pass the vehicle's own `currency` (`vehicleStore.currency` for the active vehicle), never assume EUR. Catalog strings carry no currency sign: an amount placeholder receives an already formatted amount (`"Fair share due: {amount}"`, called with `formatAmount(...)`), and a field label takes the symbol through `{cur}` (`"Amount ({cur})"`, called with `currencySymbol(...)`), so the sign lands where the locale puts it (`12,00 €` vs `US$12.00`). A manual expense form's own currency field defaults to the vehicle's currency (never `'EUR'`) and calls `currencyPayload(form, baseCurrency)` from `web/src/utils/expenses.ts` to decide whether `fx_rate` is required — the same `baseCurrency` the backend validates against.
- Unit tests run with the French locale (`web/vitest.setup.ts`).
- `<table class="sr-only">` does not clip: wrap it in `<div class="sr-only">`. After changing a page, check that `documentElement.scrollWidth` equals the viewport at 320/360/375/390/414 px.
- Do not pipe `vite build` through `tail` and trust the result: a failed build leaves the previous `dist`. Grep for `built|error`.

Refactors
- Splitting a file is a pure move unless stated otherwise: verify declaration hashes and the multiset of non-blank lines before and after, then `gofmt`, build, vet, tests. For Vue views, compare the rendered DOM and recorded API writes before and after with a Playwright characterization run (mock `/api/**`, fixed clock).
- SonarCloud counts moved code as new code: duplication that already existed inside a big file fails `new_duplicated_lines_density` (3 %) once the file is split. Find the blocks with `api/duplications/show?key=Rem7474_TeslaCost:<path>&pullRequest=<n>` and extract a small helper in a separate commit.

## Git and PRs

- Check `gh pr list --state all` before touching a branch that had a PR: PRs are merged quickly and a merged branch must not be reused. Follow-up work goes on a fresh branch from `origin/main`, one PR per topic, independent PRs rather than stacks unless the work truly depends on the previous one.
- Commit messages and PR titles/bodies are in English. PR body: `## Summary` and `## Notes` sections. `gh pr edit` fails on the deprecated Projects-classic GraphQL field; update a body with `gh api -X PATCH repos/Rem7474/TeslaCost/pulls/<n> -F body=@file`.
- Do not push local `feat/*` branches left over from merged PRs.
- Documentation states the current behavior; it does not narrate history ("now", "again", "re-introduced") or cite PR numbers. That belongs in commit messages.

## Browser checks

Playwright and Chromium are available on the dev host (`PLAYWRIGHT_BROWSERS_PATH=/opt/pw-browsers` in the remote environment). Run `vite preview` bound to 127.0.0.1, mock `/api/**` with `context.route`, and never `pkill -f` a pattern that also appears in your own command line.
