# AutoLedger 🚗

**English** · [Français](README.fr.md)

> **Self-hosted ledger of what your car really costs, per kilometre.**  
> Electric, plug-in hybrid, range-extender or combustion, any make: energy or fuel, financing (cash, loan, lease), tires, maintenance, insurance, tolls and carpooling in one place, fed by hand, by CSV, by Home Assistant or a script, or synchronised from [TeslaMate](https://github.com/teslamate-org/teslamate).

[![CI](https://github.com/Rem7474/AutoLedger/actions/workflows/ci.yml/badge.svg)](https://github.com/Rem7474/AutoLedger/actions/workflows/ci.yml)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Rem7474_TeslaCost&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=Rem7474_TeslaCost)
[![Docker Image](https://img.shields.io/badge/docker-ghcr.io%2Frem7474%2Fautoledger-blue?logo=docker)](https://github.com/Rem7474/AutoLedger/pkgs/container/autoledger)

<p align="center">
  <a href="#-what-it-does">What it does</a> ·
  <a href="#-how-data-gets-in">Data sources</a> ·
  <a href="#screenshots">Screenshots</a> ·
  <a href="#-highlights--features">Features</a> ·
  <a href="#-deployment--quick-start">Install</a> ·
  <a href="#-upgrading-from-teslacost">Upgrade</a> ·
  <a href="#-reverse-proxy--production-hardening">Reverse proxy</a> ·
  <a href="#-operations-automated-backups--restore">Backups</a> ·
  <a href="#-environment-variables-reference">Configuration</a> ·
  <a href="#-development--testing">Development</a> ·
  <a href="docs/">Documentation</a>
</p>

<p align="center">
  <img src="docs/screenshots/dashboard-ev.en.png" alt="Cost dashboard of an electric car" width="62%">
  <img src="docs/screenshots/mobile-quickadd.en.png" alt="Quick add of a fill-up on a phone" width="22%">
</p>
<p align="center">
  <img src="docs/screenshots/fleet.en.png" alt="Household fleet comparing an electric and a petrol car" width="86%">
</p>

---

## ✨ What it does

1. **Add a vehicle**: name, powertrain (electric, hybrid, range-extender or combustion), optional make and model, currency.
2. **Record what you spend**: log fill-ups and charges from the phone in a few taps, import a CSV ([format](docs/csv-import.md)), let Home Assistant or a script push charging sessions, or connect a TeslaMate instance.
3. **See the real cost per km**: energy, tolls, maintenance, tires, insurance and financing roll up into one figure, with a completeness score telling what is still missing.

Everything stays on your server: no cloud account, no manufacturer login, no telemetry sent out.

## 🔌 How data gets in

Pick whichever fits each vehicle; they can be mixed in the same account.

| Source | What it brings | Setup |
| :--- | :--- | :--- |
| **Manual / PWA** | Quick add of fill-ups, charges, expenses and odometer readings, offline queue, receipt photos | none |
| **CSV import and export** | Charges, drives, fill-ups, odometer; saved column profiles; round-trip export; OBD2 logger recipe | [docs/csv-import.md](docs/csv-import.md) |
| **Home Assistant** | Wallbox or energy-meter charging sessions sent automatically | [HACS integration](https://github.com/Rem7474/autoledger-homeassistant) |
| **Ingestion API** | Any script, Node-RED or n8n pushing charges, drives, fill-ups and odometer readings with a token | [docs/ingestion-api.md](docs/ingestion-api.md) |
| **TeslaMate sync** (optional) | Continuous import of the odometer, charge and drive history of a Tesla already logged by [TeslaMate](https://github.com/teslamate-org/teslamate) | per vehicle, in the vehicle settings |

```mermaid
flowchart LR
    A["📱 Manual / PWA"] --> Core
    B["📄 CSV import"] --> Core
    C["🏠 Home Assistant, scripts, n8n"] --> Core
    D["🔄 TeslaMate sync (optional)"] --> Core
    Core["AutoLedger ledger<br/>cost per km • financing • tires • reminders • documents"]
```

- **Manual / PWA**: offline-first progressive web app with a quick-add modal for drives, charges, fuel, tolls and maintenance, smart odometer progression, photo receipts and an IndexedDB queue.
- **CSV**: separator detection, column mapping with live preview, deduplication on timestamps and coordinates.
- **Push sources**: see the [ingestion API](docs/ingestion-api.md) ([FR](docs/ingestion-api.fr.md)), the [wallbox recipes](docs/wallbox-recipes.md) ([FR](docs/wallbox-recipes.fr.md)) and [CSV import and export](docs/csv-import.md) ([FR](docs/csv-import.fr.md)).
- **TeslaMate**: resumable full import, sliding 30-day re-read, reconciliation of deleted drives and charges, battery and temperature panels. Nothing else in AutoLedger depends on it.

### Compatibility

| Powertrain | Charges | Fill-ups | Efficiency | TeslaMate sync | Home Assistant |
| :--- | :---: | :---: | :---: | :---: | :---: |
| Electric | ✅ | - | kWh/100 km | ✅ | ✅ |
| Plug-in hybrid / range-extender | ✅ | ✅ | kWh/100 km and L/100 km | - | ✅ |
| Combustion | - | ✅ | L/100 km | - | - |

### Vehicle comparison

Compare a tracked vehicle (electric, plug-in hybrid or range-extender) with a configurable combustion one, or the reverse. The comparison uses recorded costs per kilometre; hybrids include fuel and electricity, and fuel and electricity price changes apply separately. See [Vehicle comparison](docs/vehicle-comparison.md) ([FR](docs/vehicle-comparison.fr.md)) for assumptions and limitations, including how insurance premiums are annualized.

### Electricity tariffs

A vehicle can carry a flat, peak / off-peak or multi-band [tariff plan](docs/tariffs.md) ([FR](docs/tariffs.fr.md)). A charge without a cost, from Home Assistant, a script or the charge form, is priced from its start and end times in the instance timezone, across midnight and between bands.

### Screenshots

<p align="center">
  <img src="docs/screenshots/energy.en.png" alt="Energy page of an electric car: efficiency, charges and estimate" width="48%">
  <img src="docs/screenshots/energy-fuel.en.png" alt="Fill-ups of a petrol car on the Energy page" width="48%">
</p>
<p align="center">
  <img src="docs/screenshots/expenses.en.png" alt="Expenses of a vehicle" width="48%">
  <img src="docs/screenshots/maintenance.en.png" alt="Maintenance reminders and records" width="48%">
</p>
<p align="center">
  <img src="docs/screenshots/tires.en.png" alt="Tire sets and wear" width="48%">
  <img src="docs/screenshots/drives.en.png" alt="Drives and trips" width="48%">
</p>

<p align="center">
  <img src="docs/screenshots/comparison.en.png" alt="Five-year comparison of an electric and a petrol car" width="48%">
</p>

French captures sit next to the English ones (`*.fr.png`); `scripts/screenshots/capture.mjs` regenerates all of them from a seeded demo instance.

---

## 🌟 Highlights & Features

### 📊 TCO Calculator & Flexible Financing
- **Unified Cost Ledger (`cost_ledger`)**: Centralizes every expenditure—energy (electricity/fuel), highway tolls, maintenance invoices, insurance premiums, tire amortization, and vehicle acquisition.
- **Complete Financing Models**:
  - *Cash purchase*: Linear depreciation based on residual value estimations or actual disposal sale prices.
  - *Classic loan*: Amortization schedule tracking with principal/interest split, origination fees, and borrower insurance.
  - *Leasing (LOA / LLD)*: Down payment, monthly payments, security deposit, contract mileage allowance, and excess mileage provisions.
- **Advanced Indicators**: Real cost per km (energy + tolls vs full TCO), net cost factoring carpooling revenues, and an adaptive TCO completeness score.
- **Energy Analytics** (vehicles that charge): Real consumption in kWh/100 km, home vs AC vs DC charging efficiency, battery temperature correlation, and cold-weather impact.
- **Battery health and residual value**: Source-agnostic state of health (OBD2 or garage readings, TeslaMate, or estimated from complete charges) and a resale value projection built from your own purchase price and expected resale, adjusted for age, distance and battery health.

### 🛞 Tire Lifecycle Management
- **Axle-Level Tracking**: Mount, swap, and dismount tires across axles (`FL`, `FR`, `RL`, `RR`, `STORAGE`, `DISPOSED`) with chronological session logs.
- **Wear & Mileage Projections**: Tread-depth measurement history with automatic projection of remaining safe mileage (storage periods are automatically excluded).
- **Universal Specifications**: Brand, model, ISO dimension, load/speed index, season (summer/winter/all-season), DOT manufacturing code, and purchase cost.

### 👥 Fair Carpooling Module
- **Drive Splitting**: Connect legs to actual telemetry drives, imported CSV trips, or manual entries.
- **Fair Share Calculation**: Real-world electricity pricing weighted over recent charges, combined with a consolidated per-km insurance allocation.

### 🔔 Maintenance Reminders & Homelab Notifications
- **Dual Trigger Monitoring**: Proactive notifications based on due dates and/or mileage thresholds calculated against the real odometer.
- **Fixed dates**: pin a reminder to a calendar day (winter tires on November 1), once or every year, instead of an interval; marking it done within its lead window moves a yearly one to the next year.
- **Based on a past service**: link a reminder to a maintenance already recorded; its date and odometer become the starting point and follow the record when it is edited. Completing a reminder together with an expense links the new record.
- **Your own templates**: no manufacturer plan is built in. Save a vehicle's reminders as a template, apply it to another vehicle, and see the interval you really follow once a reminder has been completed twice.
- **Multi-Channel Dispatchers**: Native integrations for **Discord** (rich embeds), **Telegram** (Markdown bot API), **Gotify** (push notifications), and **Generic JSON Webhooks** (Home Assistant, Node-RED, n8n).

### 🔐 Hybrid Authentication & Homelab Security
- **Local Authentication**: bcrypt password hashing, 15-minute HS256 JWT access tokens, and rotating 30-day `HttpOnly` refresh cookies.
- **OIDC / SSO Integration**: Seamless single sign-on with Authentik, Keycloak, Authelia, or Kanidm via standard Authorization Code Flow with PKCE.
- **Security & Hardening**: AES-256-GCM encryption for stored credentials, brute-force rate limiters, security headers (CSP, HSTS, X-Frame-Options), and configurable trusted reverse proxies.

### 🏠 Home Assistant Integration
- **Official HACS integration**: [Rem7474/autoledger-homeassistant](https://github.com/Rem7474/autoledger-homeassistant) detects charging sessions from your wallbox or energy meter and sends them to AutoLedger, with a configurable debounce for solar charging that pauses and resumes.
- **Multi-vehicle**: one charger can serve several cars, assigned to a fixed vehicle, an `input_select`, automatic correlation, or left unassigned to qualify later in the web UI.
- **Sensors and services**: last charge cost and cost per 100 km per vehicle, plus the `autoledger.sync` and `autoledger.submit_charge` services.
- It talks to the [ingestion API](#-how-data-gets-in) with an `al_live_` token; any other system can use the same API.

### 📁 Document & Invoice Archiving
- Attachments (PDF invoices, receipts, registration cards) stored securely on a dedicated volume (`/data/documents`) with non-root isolation and strict JWT authorization.

---

## 📁 Architecture & Project Structure

```text
AutoLedger/
├── cmd/
│   └── server/
│       └── main.go                 # Application entry point, chi routing & graceful shutdown
├── internal/
│   ├── apierror/                   # Coded, translatable API errors and messages
│   ├── auth/                       # bcrypt hashing, JWT token rotation & OIDC SSO client
│   ├── config/                     # Environment configuration loader with backward fallbacks
│   ├── crypto/                     # AES-256-GCM symmetric encryption engine
│   ├── database/                   # pgx connection pool, migrations runner & repositories
│   ├── demodata/                   # Seeded data of the read-only demo
│   ├── geocode/                    # Optional reverse geocoding of pushed coordinates
│   ├── handlers/                   # REST API controllers & CSV import pipeline
│   ├── middleware/                 # Security headers, rate limiting, trusted proxies, CORS
│   ├── models/                     # Strongly-typed data models (Vehicles, Drives, TCO, Tires)
│   ├── money/                      # Integer-cent amounts
│   ├── servertext/                 # English / French text for webhooks and stored notes
│   ├── services/                   # TCO engine, tire wear projection, carpool math, webhooks
│   ├── storage/                    # Document attachment storage on Docker volumes
│   ├── teslamate/                  # Client for the optional TeslaMate sync
│   └── tolldata/                   # French motorway toll reference data
├── migrations/                     # Versioned PostgreSQL schema migrations
├── web/                            # Vue 3 + TypeScript + Vite + Tailwind CSS SPA & PWA
├── backup/                         # Backup sidecar image (database dump + documents archive)
├── db-upgrade/                     # One-shot image upgrading a PostgreSQL 16 volume to 18 (dump, verify, restore)
├── docs/                           # Guides (English and French) and screenshots
├── docker-compose.yml              # Production container stack definition
├── Dockerfile                      # Multi-stage production container build
└── .env.example                    # Exhaustive environment variable template
```

---

## 🛠️ Deployment & Quick Start

### Option 1: Docker Compose (Recommended)

1. **Download the compose file:**
   ```bash
   mkdir autoledger && cd autoledger
   curl -O https://raw.githubusercontent.com/Rem7474/AutoLedger/main/docker-compose.yml
   ```

2. **Start the stack:**
   ```bash
   docker compose up -d
   ```

No configuration is required. The database password, the session signing secret and the credential encryption key are generated on first start and kept in the `autoledger_secrets` volume (the backup service archives it next to the database dump, as `autoledger-secrets-<timestamp>.tar.gz`). Keep it with your database backups: the encryption key is needed to read stored credentials. Outside Docker Compose (plain `docker run`), the image generates the session secret and the encryption key itself into `.autoledger-secrets.json` on the documents volume. To manage a value yourself, set `AUTOLEDGER_DB_PASSWORD`, `AUTOLEDGER_JWT_SECRET` or `AUTOLEDGER_ENCRYPTION_KEY` in a `.env` file (template: `.env.example`); an explicit value always wins. Optional settings such as OIDC / SSO also go in `.env`.

The application is served at **`http://localhost:8080`**.

---

### Option 2: Standalone Docker Run

Multi-architecture images (`linux/amd64`, `linux/arm64`) are published to the GitHub Container Registry:

```bash
docker pull ghcr.io/rem7474/autoledger:latest
```

Example standalone run connecting to an existing PostgreSQL database:

```bash
docker run -d \
  --name autoledger-app \
  -p 8080:8080 \
  -v autoledger_documents:/data/documents \
  -e ENVIRONMENT="production" \
  -e AUTOLEDGER_DATABASE_URL="postgres://autoledger:secret@postgres-host:5432/autoledger?sslmode=disable" \
  -e AUTOLEDGER_JWT_SECRET="your_strong_jwt_secret" \
  -e AUTOLEDGER_ENCRYPTION_KEY="hex_key_of_exactly_64_characters" \
  -e APP_TIMEZONE="Europe/Paris" \
  ghcr.io/rem7474/autoledger:latest
```

---

## 🔄 Upgrading from TeslaCost

An existing TeslaCost deployment upgrades in place: no data migration and no manual volume copy. The image is `ghcr.io/rem7474/autoledger`, and every TeslaCost variable (see the legacy column of the environment reference below) is still read, including `TESLACOST_VERSION`.

New installations create `autoledger_*` volumes. Existing ones keep their data by pointing the compose file at the volumes they already have:

| Data | New default volume | TeslaCost volume | Override variable |
|---|---|---|---|
| PostgreSQL | `autoledger_db_data` | `postgres_data` | `DB_VOLUME_NAME` |
| Documents | `autoledger_documents` | `teslacost_documents` | `DOCUMENTS_VOLUME_NAME` |
| Backups | `autoledger_backups` | `teslacost_backups` | `BACKUPS_VOLUME_NAME` |
| Generated secrets (database password, session secret, encryption key) | `autoledger_secrets` | - | `SECRETS_VOLUME_NAME` |

```dotenv
DB_VOLUME_NAME=postgres_data
DOCUMENTS_VOLUME_NAME=teslacost_documents
BACKUPS_VOLUME_NAME=teslacost_backups

# The database role and name stored in the existing volume:
AUTOLEDGER_DB_USER=teslacost
AUTOLEDGER_DB_NAME=teslacost
```

Then pull and restart:
```bash
docker compose pull && docker compose up -d
```
Vehicles, charges, expenses, invoices and settings are unchanged.

---

## 🛡️ Reverse Proxy & Production Hardening

AutoLedger does not terminate TLS: put it behind a modern reverse proxy (Caddy, Traefik, Nginx) with an SSL certificate.

Example configuration with **Caddy** (`Caddyfile`):

```caddyfile
autoledger.homelab.local {
    reverse_proxy localhost:8080
}
```

- **Trusted Proxies**: `TRUSTED_PROXIES` ensures `X-Forwarded-*` headers are only honored from your reverse proxy (defaults to loopback and private LAN/Docker networks).
- **Security Headers**: Built-in strict CSP (`Content-Security-Policy`), HSTS over HTTPS, and `X-Frame-Options: DENY`.
- **Production Safety Check**: In `ENVIRONMENT=production`, the application refuses to boot if default repository passwords or secrets are detected. An unset JWT secret or encryption key is generated and persisted instead.

---

## 🛟 Operations: Automated Backups & Restore

### Automated Backups
The included `backup` sidecar container runs continuously alongside the database and application. Every `BACKUP_INTERVAL_HOURS` hours (default: 24h), it produces a compressed PostgreSQL dump and an archive of the document attachments volume:

```bash
# View backup archives
docker compose exec backup ls -lh /backups

# Inspect backup logs
docker compose logs -f backup
```

### Restoring a Backup
```bash
# 1. Extract database dump from the backup volume
docker compose cp backup:/backups/autoledger-db-<timestamp>.sql.gz .

# 2. Restore PostgreSQL database
gunzip -c autoledger-db-<timestamp>.sql.gz | docker compose exec -T postgres psql -U "${AUTOLEDGER_DB_USER:-autoledger}" -d "${AUTOLEDGER_DB_NAME:-autoledger}"

# 3. Restore document attachments
docker compose cp backup:/backups/autoledger-documents-<timestamp>.tar.gz .
docker run --rm \
  -v autoledger_documents:/data \
  -v "$(pwd)":/backup \
  alpine sh -c "cd /data && tar -xzf /backup/autoledger-documents-<timestamp>.tar.gz --strip-components=1"
```

### Upgrading PostgreSQL 16 to 18
Starting the stack on a database volume written by PostgreSQL 16 upgrades it automatically, with a backup first:

1. `db-upgrade` starts the old server alone and writes a verified dump (`pg16-upgrade-<timestamp>.sql.gz`) and a copy of the data directory (`pg16-datadir-<timestamp>.tar.gz`) to the backups volume. These two files are never pruned.
2. Only once both are verified, it empties the data directory, and PostgreSQL 18 initializes a new cluster.
3. `db-restore` loads the dump in a single transaction, then the application starts.

These steps come from the `db-upgrade` image, published with the same tag as the application, so keep `docker-compose.yml` up to date: an older file that only bumps the `postgres` image makes PostgreSQL refuse the old data directory, and the application logs a hint pointing here while it waits for the database.

If any step fails, the stack stops with the old data untouched; read the logs with `docker compose logs db-upgrade db-restore`. Once the application runs correctly, delete the two `pg16-*` files to free the space. A volume written by another major than 16 is refused with an explicit message.

To restore the pre-upgrade state, stop the stack, empty the data volume, and extract `pg16-datadir-<timestamp>.tar.gz` into it from a `postgres:16-alpine` container.

---

## ⚙️ Environment Variables Reference

| Variable (Primary) | Legacy Fallback | Description | Default |
|---|---|---|---|
| `AUTOLEDGER_PORT` | `PORT` | HTTP server listening port | `8080` |
| `ENVIRONMENT` | - | Runtime environment (`production`, `development`) | `production` in the Compose file, `development` when unset elsewhere |
| `AUTOLEDGER_VERSION` | `TESLACOST_VERSION` | Docker image tag to deploy | `latest` |
| `AUTOLEDGER_BASE_URL` | `APP_BASE_URL` | Canonical public URL of the application | `http://localhost:8080` |
| `AUTOLEDGER_DATABASE_URL`| `DATABASE_URL` | Full PostgreSQL connection URL | *Derived from DB_\** |
| `AUTOLEDGER_DB_HOST` | `DB_HOST` | PostgreSQL host (when set, the URL is built from the `DB_*` variables) | *Unset* |
| `AUTOLEDGER_DB_PORT` | `DB_PORT` | PostgreSQL port | `5432` |
| `AUTOLEDGER_DB_USER` | `DB_USER` | PostgreSQL user | `autoledger` |
| `AUTOLEDGER_DB_PASSWORD` | `DB_PASSWORD` | PostgreSQL password (`DB_PASSWORD_FILE` reads it from a file) | *Generated* |
| `AUTOLEDGER_DB_NAME` | `DB_NAME` | PostgreSQL database name | `autoledger` |
| `AUTOLEDGER_ENCRYPTION_KEY`| `APP_ENCRYPTION_KEY` | 32-byte AES-256 key for sensitive credentials | *Generated* |
| `AUTOLEDGER_JWT_SECRET` | `JWT_SECRET` | Secret key for signing user sessions | *Generated* |
| `AUTOLEDGER_STORAGE_DIR` | `STORAGE_DIR` | Filesystem path for document attachments | `/data/documents` |
| `APP_TIMEZONE` | - | IANA timezone for reports and aggregations | `Europe/Paris` |
| `GEOCODING_ENABLED` | - | Resolve the coordinates sent by integrations (Home Assistant trips) into addresses through OpenStreetMap Nominatim. Off by default: the coordinates are sent to the geocoding service | `false` |
| `GEOCODING_URL` / `GEOCODING_USER_AGENT` | - | Nominatim instance to use (a self-hosted one avoids sending coordinates to a third party) and the User-Agent sent to it | Public OSM instance / `AutoLedger (+https://github.com/Rem7474/AutoLedger)` |
| `DISABLE_REGISTRATION` | - | Set to `true` to disable public user registration | `false` |
| `AUTOLEDGER_DEMO` | - | Read-only public demo: every write except sign-in is refused, registration is closed | `false` |
| `AUTOLEDGER_DEMO_EMAIL` / `AUTOLEDGER_DEMO_PASSWORD` | - | Demo account offered by the "Try the demo" button on the sign-in page (public by design) | - |
| `INITIAL_ADMIN_EMAIL` | - | Pre-configured admin user email | *Optional* |
| `INITIAL_ADMIN_PASSWORD` | - | Pre-configured admin user password | *Optional* |
| `CORS_ALLOWED_ORIGINS` | - | Comma-separated origins allowed to call the API from a browser, besides the public base URL | Derived from `AUTOLEDGER_BASE_URL` |
| `TRUSTED_PROXIES` | - | Reverse proxy CIDR whitelist for client IP resolution | Private ranges |
| `SECURITY_HEADERS` | - | Enable built-in HTTP security headers | `true` |
| `CONTENT_SECURITY_POLICY`| - | Override CSP policy (`off` to disable) | Built-in strict |
| `BACKUP_INTERVAL_HOURS` | - | Interval between automated backup archives | `24` |
| `BACKUP_RETENTION_DAYS` | - | Days to retain backup archives before pruning | `14` |

Each primary variable wins when both are set; the legacy name is only read when the primary one is empty. The legacy names are the ones used by TeslaCost deployments, so an existing `.env` keeps working unchanged.

### OIDC / SSO Configuration (Optional)

| Variable | Description | Example |
|---|---|---|
| `OIDC_ISSUER_URL` | IdP OpenID Connect discovery URL | `https://auth.homelab.local/application/o/autoledger/` |
| `OIDC_CLIENT_ID` | OAuth2 client identifier | `autoledger` |
| `OIDC_CLIENT_SECRET` | OAuth2 client secret | `your_secret_from_idp` |
| `OIDC_REDIRECT_URL` | Registered redirect URI | `https://autoledger.homelab.local/api/auth/oidc/callback` |
| `OIDC_PROVIDER_NAME` | Display name on the login screen | `Authentik` / `Keycloak` |
| `OIDC_SCOPES` | Requested OAuth2 scopes | `openid email profile` |
| `OIDC_ALLOWED_EMAILS` | Comma-separated whitelist of allowed user emails | `user@example.com` |
| `OIDC_DISABLE_LOCAL_AUTH` | Disable local email/password sign-in | `false` |

---

## 🧪 Development & Testing

```bash
# Run backend unit tests
go test -v ./...

# Run backend integration tests with PostgreSQL
docker run -d --name autoledger-test-pg -e POSTGRES_USER=autoledger -e POSTGRES_PASSWORD=test -e POSTGRES_DB=autoledger_test -p 55432:5432 postgres:18-alpine
TEST_DATABASE_URL="postgres://autoledger:test@localhost:55432/autoledger_test?sslmode=disable" go test -v ./...

# Run frontend unit tests and typecheck
cd web && npm test && npm run typecheck

# Build frontend production bundle
cd web && npm run build
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to report a bug, propose a feature and open a pull request.

---

## 📄 License

Distributed under the [MIT](LICENSE) license.

Toll highway calculation data (`internal/tolldata`) is sourced from [OpenTollData](https://github.com/louis2038/OpenTollData), licensed under [ODbL-1.0](https://opendatacommons.org/licenses/odbl/1-0/).
