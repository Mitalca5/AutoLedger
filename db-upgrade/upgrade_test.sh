#!/bin/sh
# Regression test of the PostgreSQL 16 to 18 upgrade, on real containers. Run from the repository root with a
# working Docker CLI: sh db-upgrade/upgrade_test.sh
set -eu

IMG=autoledger-db-upgrade:test
N=$$
NET=dbup-net-$N
DATA=dbup-data-$N
BK=dbup-backups-$N
SEC=dbup-secrets-$N
PW=test-password

cleanup() {
  docker rm -f "dbup-pg-$N" >/dev/null 2>&1 || true
  docker volume rm -f "$DATA" "$BK" "$SEC" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }

docker build -q -t "$IMG" db-upgrade >/dev/null
docker network create "$NET" >/dev/null
docker volume create "$DATA" >/dev/null
docker volume create "$BK" >/dev/null
docker volume create "$SEC" >/dev/null
docker run --rm -v "$SEC":/secrets alpine sh -c "printf %s '$PW' > /secrets/db_password"

run_upgrade() {
  docker run --rm -e DB_USER=autoledger -e DB_NAME="${1:-autoledger}" -v "$DATA":/var/lib/postgresql/data -v "$BK":/backups "$IMG"
}
in_volume() {
  docker run --rm -v "$DATA":/data -v "$BK":/backups alpine sh -c "$1"
}

echo "== seed a PostgreSQL 16 volume"
docker run -e POSTGRES_USER=autoledger -e POSTGRES_PASSWORD="$PW" -e POSTGRES_DB=autoledger \
  -e PGDATA=/var/lib/postgresql/data -v "$DATA":/var/lib/postgresql/data --name "dbup-pg-$N" -d postgres:16-alpine >/dev/null
until docker exec "dbup-pg-$N" pg_isready -h 127.0.0.1 -U autoledger -q; do sleep 1; done
docker exec "dbup-pg-$N" psql -h 127.0.0.1 -U autoledger -d autoledger -qc "CREATE TABLE t(v text); INSERT INTO t VALUES ('kept')"
docker stop "dbup-pg-$N" >/dev/null
docker rm -f "dbup-pg-$N" >/dev/null

echo "== a failing dump leaves the old data untouched"
if run_upgrade no_such_database >/dev/null 2>&1; then fail "the upgrade succeeded with an unknown database"; fi
in_volume 'test "$(cat /data/PG_VERSION)" = 16' || fail "the data directory was modified"
in_volume 'ls /backups | grep -q pg16 && exit 1 || exit 0' || fail "a partial backup was left behind"

echo "== the upgrade backs up and empties the data directory"
run_upgrade
in_volume 'ls /backups/pg16-upgrade-*.sql.gz /backups/pg16-datadir-*.tar.gz >/dev/null && test -f /backups/.pg-restore-pending' || fail "backups or marker missing"
in_volume 'test -z "$(ls -A /data)"' || fail "the data directory was not emptied"

echo "== PostgreSQL 18 starts on the emptied volume and the dump is restored"
docker run -d --name "dbup-pg-$N" --network "$NET" --network-alias postgres \
  -e POSTGRES_USER=autoledger -e POSTGRES_PASSWORD="$PW" -e POSTGRES_DB=autoledger -e PGDATA=/var/lib/postgresql/data \
  -v "$DATA":/var/lib/postgresql/data postgres:18-alpine >/dev/null
until docker exec "dbup-pg-$N" pg_isready -h 127.0.0.1 -U autoledger -q; do sleep 1; done
docker run --rm --network "$NET" -e DB_USER=autoledger -e DB_NAME=autoledger -v "$BK":/backups -v "$SEC":/secrets:ro \
  --entrypoint /usr/local/bin/db-restore "$IMG"
test "$(docker exec "dbup-pg-$N" psql -h 127.0.0.1 -U autoledger -d autoledger -Atc 'SELECT v FROM t')" = kept || fail "the data was not restored"
in_volume 'test ! -f /backups/.pg-restore-pending && ls /backups/.pg-restore-done-* >/dev/null' || fail "the marker was not retired"
in_volume 'test "$(cat /data/PG_VERSION)" = 18' || fail "the volume is not a PostgreSQL 18 cluster"

echo "== a second run changes nothing"
run_upgrade
in_volume 'test "$(ls /backups/pg16-upgrade-*.sql.gz | wc -l)" = 1' || fail "a second backup was taken"

echo "OK"
