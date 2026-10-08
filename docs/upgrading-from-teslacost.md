# Upgrading from TeslaCost

Français : [upgrading-from-teslacost.fr.md](upgrading-from-teslacost.fr.md).

An existing TeslaCost deployment upgrades in place: the data stays in its current volumes. The image is `ghcr.io/rem7474/autoledger`, and every TeslaCost variable (see the legacy column of the [environment reference](../README.md)) is still read, including `TESLACOST_VERSION`.

New installations create `autoledger_*` volumes. An existing installation must name the volumes it already has, otherwise the stack starts on an empty database and shows the onboarding page.

1. **List the existing volumes.** Compose prefixes volume names with the project (directory) name:
   ```bash
   docker volume ls | grep -Ei "postgres_data|documents|backups"
   ```
   For a stack started from a directory called `teslacost`, with the TeslaCost compose file, the volumes are typically `teslacost_postgres_data`, `teslacost_teslacost_documents` and `teslacost_teslacost_backups`.

2. **Declare them in `.env`**, together with the credentials and the encryption key of the TeslaCost stack:

   | Data | New default volume | Override variable |
   |---|---|---|
   | PostgreSQL | `autoledger_db_data` | `DB_VOLUME_NAME` |
   | Documents | `autoledger_documents` | `DOCUMENTS_VOLUME_NAME` |
   | Backups | `autoledger_backups` | `BACKUPS_VOLUME_NAME` |
   | Generated secrets (database password, session secret, encryption key) | `autoledger_secrets` | `SECRETS_VOLUME_NAME` |

   ```dotenv
   DB_VOLUME_NAME=teslacost_postgres_data
   DOCUMENTS_VOLUME_NAME=teslacost_teslacost_documents
   BACKUPS_VOLUME_NAME=teslacost_teslacost_backups

   # The database role, name and password stored in the existing volume:
   DB_USER=teslacost
   DB_NAME=teslacost
   DB_PASSWORD=<your TeslaCost database password>

   # Stored secrets are encrypted with this key: keep the value used by TeslaCost
   # (the compose default when it was never set is change-this-to-a-secure-32-byte-key-in-prod!).
   APP_ENCRYPTION_KEY=<your TeslaCost key>
   JWT_SECRET=<your TeslaCost secret>
   ```

3. **Pull and restart.** A PostgreSQL 16 volume is then upgraded to 18 automatically, with a verified backup first (see [Upgrading PostgreSQL 16 to 18](../README.md#upgrading-postgresql-16-to-18)):
   ```bash
   docker compose pull && docker compose up -d
   docker compose logs db-upgrade db-restore
   ```

Vehicles, charges, expenses, invoices and settings are unchanged.

Never run `docker compose down -v` or `docker volume rm` on these volumes: they hold the data.

## Moving to the `autoledger_*` volumes (optional)

Instead of keeping the TeslaCost volume names, the data can be copied into the default `autoledger_*` volumes. The copy reads the old volumes read-only and leaves them untouched, so they remain a fallback. Stop the stack first (without `-v`), then, with the real names from `docker volume ls`:

```bash
docker compose down
for pair in teslacost_postgres_data:autoledger_db_data \
            teslacost_teslacost_documents:autoledger_documents \
            teslacost_teslacost_backups:autoledger_backups; do
  docker run --rm -v "${pair%%:*}":/from:ro -v "${pair##*:}":/to alpine cp -a /from/. /to/
done
```

Then remove the three `*_VOLUME_NAME` lines from `.env` and start the stack. The database role and name keep the TeslaCost values until they are renamed in the last section, and the encryption key value stays the same; the key can be set as `AUTOLEDGER_ENCRYPTION_KEY` instead of `APP_ENCRYPTION_KEY`.

## Renaming the database role and name (optional)

The role and database of a TeslaCost stack are named `teslacost`. They can be renamed to `autoledger` once the stack runs on the migrated data (the stored data, ownership and password are kept). PostgreSQL cannot rename the role of the current session, nor a database in use, so the commands go through a temporary superuser:

```bash
docker compose stop api backup
export OLD=teslacost NEW=autoledger PW='<your database password>'
docker compose exec -T postgres psql -U "$OLD" -d postgres -v ON_ERROR_STOP=1 \
  -c "CREATE ROLE tmp_admin SUPERUSER LOGIN"
docker compose exec -T postgres psql -h 127.0.0.1 -U tmp_admin -d postgres -v ON_ERROR_STOP=1 -v pw="$PW" <<SQL
ALTER DATABASE $OLD RENAME TO $NEW;
ALTER ROLE $OLD RENAME TO $NEW;
ALTER ROLE $NEW PASSWORD :'pw';
SQL
docker compose exec -T postgres psql -h 127.0.0.1 -U "$NEW" -d postgres -c "DROP ROLE tmp_admin"
```

Then delete `DB_USER` and `DB_NAME` (or `AUTOLEDGER_DB_USER` and `AUTOLEDGER_DB_NAME`) from `.env`, keep `DB_PASSWORD` set to the same password, and run `docker compose up -d`.
