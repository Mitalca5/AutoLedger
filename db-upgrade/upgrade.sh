#!/bin/sh
# Upgrades a PostgreSQL 16 database volume (mounted on /var/lib/postgresql/data) to PostgreSQL 18.
#
# It starts the old server alone, takes a verified dump and a copy of the data directory into /backups
# (pg16-upgrade-*.sql.gz and pg16-datadir-*.tar.gz, never pruned), then empties the data directory so that
# PostgreSQL 18 initializes a new cluster. restore.sh loads the dump afterwards. Nothing is deleted unless both
# files were verified, and any failure leaves the old data untouched. It does nothing on an up-to-date volume.
#
# Environment: DB_USER, DB_NAME.
set -eu
set -o pipefail

D=/var/lib/postgresql/data
[ -s "$D/PG_VERSION" ] || exit 0
V=$(cat "$D/PG_VERSION")
[ "$V" != 18 ] || exit 0
if [ "$V" != 16 ]; then
  echo "db-upgrade: the database volume holds PostgreSQL $V; only 16 is upgraded automatically. See the README." >&2
  exit 1
fi

TS=$(date +%Y%m%d-%H%M%S)
DUMP=/backups/pg16-upgrade-$TS.sql.gz
TAR=/backups/pg16-datadir-$TS.tar.gz
echo "db-upgrade: PostgreSQL 16 volume found, backing it up to $DUMP and $TAR"

su-exec postgres pg_ctl -D "$D" -o "-c listen_addresses= -c unix_socket_directories=/tmp" -w start
trap 'su-exec postgres pg_ctl -D "$D" -m fast -w stop >/dev/null 2>&1 || true; rm -f "$DUMP.tmp" "$TAR.tmp"' EXIT

PGHOST=/tmp pg_dump -U "$DB_USER" -d "$DB_NAME" | gzip > "$DUMP.tmp"
gzip -t "$DUMP.tmp"
gunzip -c "$DUMP.tmp" | tail -n 5 | grep -q "PostgreSQL database dump complete"

su-exec postgres pg_ctl -D "$D" -m fast -w stop
tar -C "$D" -czf "$TAR.tmp" .
tar -tzf "$TAR.tmp" >/dev/null

mv "$DUMP.tmp" "$DUMP"
mv "$TAR.tmp" "$TAR"
chown postgres "$DUMP" "$TAR"
echo "$DUMP" > /backups/.pg-restore-pending
find "$D" -mindepth 1 -delete
trap - EXIT
echo "db-upgrade: done, the data will be restored into PostgreSQL 18"
