# AutoLedger 🚗⚡

> **Self-hosted, open-source Total Cost of Ownership (TCO) ledger for electric, hybrid, and combustion vehicles.**  
> Track every cent—energy, financing (cash, loan, lease/LOA/LLD), axle-level tire wear, maintenance, tolls, and carpooling—whether connected via telemetry, imported via CSV, or managed 100% standalone.

[![CI](https://github.com/Rem7474/AutoLedger/actions/workflows/ci.yml/badge.svg)](https://github.com/Rem7474/AutoLedger/actions/workflows/ci.yml)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Rem7474_TeslaCost&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=Rem7474_TeslaCost)
[![Docker Image](https://img.shields.io/badge/docker-ghcr.io%2Frem7474%2Fautoledger-blue?logo=docker)](https://github.com/Rem7474/AutoLedger/pkgs/container/autoledger)

---

## 💡 Why AutoLedger? (Unfair Advantage)

Most vehicle tools force a frustrating compromise: proprietary mobile apps only show battery or fuel status, cloud SaaS charge high monthly fees and lock you into a single brand, and spreadsheets quickly degenerate into unmaintainable formula webs.

**AutoLedger** bridges this gap as a dedicated, self-hosted financial and telemetry companion.

| Feature / Capability | AutoLedger | Spreadsheets (Excel / Sheets) | Paid SaaS (Tronity, Tessie...) | OEM Apps (MyRenault, MyPeugeot...) |
| :--- | :---: | :---: | :---: | :---: |
| **Self-Hosted & Private** | ✅ 100% Local / Open Source | ✅ Local | ❌ Proprietary Cloud ($$) | ❌ OEM Cloud |
| **Universal Multi-Brand** | ✅ EV, Hybrid & Combustion | ⚠️ Manual setup | ❌ Brand-locked / EV-only | ❌ Single brand only |
| **Full TCO Engine** | ✅ To the cent (Financing, Depr., Tires, Tolls) | ⚠️ Manual formula hell | ❌ Energy/Charges only | ❌ Very basic |
| **Ingestion Flexibility** | ✅ 3 modes (Live API, CSV, Manual) | ❌ Manual typing | ❌ Telemetry API only | ❌ App only |
| **Tire Lifecycle by Axle** | ✅ Tread depth & mileage projection | ❌ Manual | ❌ No | ❌ No |
| **Fair Carpooling Module** | ✅ Weighted energy + insurance split | ❌ Manual math | ❌ No | ❌ No |
| **Invoice & Document Storage** | ✅ Built-in encrypted volume | ❌ Separate folders | ❌ No | ❌ No |
| **Homelab Webhooks & OIDC** | ✅ Discord, Telegram, Gotify, SSO | ❌ No | ⚠️ Limited webhooks | ❌ No |

---

## 🔄 The 3 Ingestion Modes

AutoLedger adapts to your vehicle setup, never the other way around:

```mermaid
flowchart LR
    subgraph Ingestion["How Data Enters AutoLedger"]
        M1["🌐 Connected Telemetry\n- TeslaMate live sync\n- Real-time odometer\n- Automated charge & drive history"]
        M2["📄 Semi-Automated CSV\n- Universal CSV importer\n- Smart column mapping\n- Duplicate deduplication"]
        M3["📱 Standalone / Manual\n- Ultra-fast PWA interface\n- Predictive odometer\n- Camera receipt capture & offline sync"]
    end
    Ingestion --> Core["⚡ AutoLedger Core Ledger\nTCO Engine • Financing • Tires • Carpooling • Documents"]
```

1. **🌐 Connected Telemetry**: Automatic, continuous background sync with [TeslaMate](https://github.com/teslamate-org/teslamate). Features real-time odometer readbacks, resumable imports, and sliding re-reads for updated charging fees.
2. **📄 Semi-Automated CSV Import**: Flexible file importer for trips and charges. Features automatic separator detection (comma or semicolon), custom column mapping with live preview, and intelligent deduplication based on timestamps and start/end coordinates.
3. **📱 Standalone & Manual (PWA)**: Full offline-first progressive web app. Quick-add modal for drives, charges, fuel, tolls, and maintenance, with smart odometer progression, photo receipt uploads, and IndexedDB queuing.

Home Assistant, Node-RED, n8n or any script can also push charging sessions, drives, fill-ups and odometer readings: see the [ingestion API](docs/ingestion-api.md) ([FR](docs/ingestion-api.fr.md)). The CSV columns, import profiles, export and an OBD2 logger recipe are in [CSV import and export](docs/csv-import.md) ([FR](docs/csv-import.fr.md)).

---

## 🌟 Highlights & Features

### 📊 TCO Calculator & Flexible Financing
- **Unified Cost Ledger (`cost_ledger`)**: Centralizes every expenditure—energy (electricity/fuel), highway tolls, maintenance invoices, insurance premiums, tire amortization, and vehicle acquisition.
- **Complete Financing Models**:
  - *Cash purchase*: Linear depreciation based on residual value estimations or actual disposal sale prices.
  - *Classic loan*: Amortization schedule tracking with principal/interest split, origination fees, and borrower insurance.
  - *Leasing (LOA / LLD)*: Down payment, monthly payments, security deposit, contract mileage allowance, and excess mileage provisions.
- **Advanced Indicators**: Real cost per km (energy + tolls vs full TCO), net cost factoring carpooling revenues, and an adaptive TCO completeness score.
- **Energy Analytics** (for EVs): Real consumption in kWh/100 km, home vs AC vs DC charging efficiency, battery temperature correlation, and cold-weather impact.

### 🛞 Tire Lifecycle Management
- **Axle-Level Tracking**: Mount, swap, and dismount tires across axles (`FL`, `FR`, `RL`, `RR`, `STORAGE`, `DISPOSED`) with chronological session logs.
- **Wear & Mileage Projections**: Tread-depth measurement history with automatic projection of remaining safe mileage (storage periods are automatically excluded).
- **Universal Specifications**: Brand, model, ISO dimension, load/speed index, season (summer/winter/all-season), DOT manufacturing code, and purchase cost.

### 👥 Fair Carpooling Module
- **Drive Splitting**: Connect legs to actual telemetry drives, imported CSV trips, or manual entries.
- **Fair Share Calculation**: Real-world electricity pricing weighted over recent charges, combined with a consolidated per-km insurance allocation.

### 🔔 Maintenance Reminders & Homelab Notifications
- **Dual Trigger Monitoring**: Proactive notifications based on due dates and/or mileage thresholds calculated against the real odometer.
- **Multi-Channel Dispatchers**: Native integrations for **Discord** (rich embeds), **Telegram** (Markdown bot API), **Gotify** (push notifications), and **Generic JSON Webhooks** (Home Assistant, Node-RED, n8n).

### 🔐 Hybrid Authentication & Homelab Security
- **Local Authentication**: bcrypt password hashing, 15-minute HS256 JWT access tokens, and rotating 30-day `HttpOnly` refresh cookies.
- **OIDC / SSO Integration**: Seamless single sign-on with Authentik, Keycloak, Authelia, or Kanidm via standard Authorization Code Flow with PKCE.
- **Security & Hardening**: AES-256-GCM encryption for stored credentials, brute-force rate limiters, security headers (CSP, HSTS, X-Frame-Options), and configurable trusted reverse proxies.

### 🏠 Home Assistant Integration
- **Official HACS integration**: [Rem7474/autoledger-homeassistant](https://github.com/Rem7474/autoledger-homeassistant) detects charging sessions from your wallbox or energy meter and sends them to AutoLedger, with a configurable debounce for solar charging that pauses and resumes.
- **Multi-vehicle**: one charger can serve several cars, assigned to a fixed vehicle, an `input_select`, automatic correlation, or left unassigned to qualify later in the web UI.
- **Sensors and services**: last charge cost and cost per 100 km per vehicle, plus the `autoledger.sync` and `autoledger.submit_charge` services.
- It talks to the [ingestion API](#-the-3-ingestion-modes) with an `al_live_` token; any other system can use the same API.

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
│   ├── auth/                       # bcrypt hashing, JWT token rotation & OIDC SSO client
│   ├── config/                     # Environment configuration loader with backward fallbacks
│   ├── crypto/                     # AES-256-GCM symmetric encryption engine
│   ├── database/                   # pgx connection pool & repository interfaces
│   ├── handlers/                   # REST API controllers & CSV import pipeline
│   ├── middleware/                 # Security headers, rate limiting, trusted proxies, CORS
│   ├── models/                     # Strongly-typed data models (Vehicles, Drives, TCO, Tires)
│   ├── services/                   # TCO engine, tire wear projection, carpool math, webhooks
│   ├── storage/                    # Document attachment storage on Docker volumes
│   └── teslamate/                  # Telemetry client for optional TeslaMate live integration
├── migrations/                     # Versioned PostgreSQL schema migrations
├── web/                            # Vue 3 + TypeScript + Vite + Tailwind CSS SPA & PWA
├── docker-compose.yml              # Production container stack definition
├── docker-compose.dev.yml          # Local development hot-reload override
├── Dockerfile                      # Multi-stage production container build
└── .env.example                    # Exhaustive environment variable template
```

---

## 🛠️ Deployment & Quick Start

### Option 1: Docker Compose (Recommended)

1. **Create an installation directory and download the configuration:**
   ```bash
   mkdir autoledger && cd autoledger
   curl -O https://raw.githubusercontent.com/Rem7474/AutoLedger/main/docker-compose.yml
   curl -o .env https://raw.githubusercontent.com/Rem7474/AutoLedger/main/.env.example
   ```

2. **Generate your production encryption key and configure `.env`:**
   ```bash
   # Generate a 32-byte AES-256 encryption key (64 hex characters):
   openssl rand -hex 32
   ```
   Open `.env` and set your secrets:
   - `AUTOLEDGER_ENCRYPTION_KEY`: your generated 64-character hex key
   - `AUTOLEDGER_DB_PASSWORD`: a strong database password
   - `AUTOLEDGER_JWT_SECRET`: your random token signing secret
   - *(Optional)* OIDC / SSO parameters if using Authentik, Keycloak, etc.

3. **Start the stack:**
   ```bash
   docker compose up -d
   ```

The application is now live at **`http://localhost:8080`**.

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
- **Production Safety Check**: In `ENVIRONMENT=production`, the application refuses to boot if default repository passwords or secrets are detected.

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

---

## ⚙️ Environment Variables Reference

| Variable (Primary) | Legacy Fallback | Description | Default |
|---|---|---|---|
| `AUTOLEDGER_PORT` | `PORT` | HTTP server listening port | `8080` |
| `ENVIRONMENT` | - | Runtime environment (`production`, `development`) | `production` |
| `AUTOLEDGER_VERSION` | `TESLACOST_VERSION` | Docker image tag to deploy | `latest` |
| `AUTOLEDGER_BASE_URL` | `APP_BASE_URL` | Canonical public URL of the application | `http://localhost:8080` |
| `AUTOLEDGER_DATABASE_URL`| `DATABASE_URL` | Full PostgreSQL connection URL | *Derived from DB_\** |
| `AUTOLEDGER_DB_HOST` | `DB_HOST` | PostgreSQL host (when set, the URL is built from the `DB_*` variables) | *Unset* |
| `AUTOLEDGER_DB_PORT` | `DB_PORT` | PostgreSQL port | `5432` |
| `AUTOLEDGER_DB_USER` | `DB_USER` | PostgreSQL user | `autoledger` |
| `AUTOLEDGER_DB_PASSWORD` | `DB_PASSWORD` | PostgreSQL password | `autoledger_dev_secret` |
| `AUTOLEDGER_DB_NAME` | `DB_NAME` | PostgreSQL database name | `autoledger` |
| `AUTOLEDGER_ENCRYPTION_KEY`| `APP_ENCRYPTION_KEY` | 32-byte AES-256 key for sensitive credentials | *Required in prod* |
| `AUTOLEDGER_JWT_SECRET` | `JWT_SECRET` | Secret key for signing user sessions | *Required in prod* |
| `AUTOLEDGER_STORAGE_DIR` | `STORAGE_DIR` | Filesystem path for document attachments | `/data/documents` |
| `APP_TIMEZONE` | - | IANA timezone for reports and aggregations | `Europe/Paris` |
| `DISABLE_REGISTRATION` | - | Set to `true` to disable public user registration | `false` |
| `INITIAL_ADMIN_EMAIL` | - | Pre-configured admin user email | *Optional* |
| `INITIAL_ADMIN_PASSWORD` | - | Pre-configured admin user password | *Optional* |
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
docker run -d --name autoledger-test-pg -e POSTGRES_USER=autoledger -e POSTGRES_PASSWORD=test -e POSTGRES_DB=autoledger_test -p 55432:5432 postgres:16-alpine
TEST_DATABASE_URL="postgres://autoledger:test@localhost:55432/autoledger_test?sslmode=disable" go test -v ./internal/services/

# Run frontend unit tests and typecheck
cd web && npm test && npm run typecheck

# Build frontend production bundle
cd web && npm run build
```

---

## 📄 License

Distributed under the [MIT](LICENSE) license.

Toll highway calculation data (`internal/tolldata`) is sourced from [OpenTollData](https://github.com/louis2038/OpenTollData), licensed under [ODbL-1.0](https://opendatacommons.org/licenses/odbl/1-0/).
