#!/bin/sh
# Loads the dump taken by upgrade.sh into the new PostgreSQL 18 cluster, in one transaction. It does nothing when
# no upgrade is pending (no /backups/.pg-restore-pending marker).
#
# Environment: DB_USER, DB_NAME. The password is read from /secrets/db_password; the server is the "postgres" host.
set -eu

M=/backups/.pg-restore-pending
[ -f "$M" ] || exit 0
DUMP=$(cat "$M")
[ -s "$DUMP" ] || { echo "db-restore: $DUMP is missing or empty" >&2; exit 1; }

echo "db-restore: restoring $DUMP"
until pg_isready -h postgres -q; do sleep 1; done
export PGHOST=postgres PGUSER="$DB_USER" PGDATABASE="$DB_NAME" PGPASSWORD="$(cat /secrets/db_password)"
gunzip -c "$DUMP" | psql -q -v ON_ERROR_STOP=1 --single-transaction -o /dev/null
mv "$M" "/backups/.pg-restore-done-$(date +%Y%m%d-%H%M%S)"
echo "db-restore: done"
