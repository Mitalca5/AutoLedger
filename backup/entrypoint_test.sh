#!/bin/sh
# Run under the backup image's BusyBox ash (or bash).
set -eu
test_root="$(mktemp -d)"
trap 'rm -rf "$test_root"' EXIT
script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

# Load the real functions without starting the periodic loop.
sed '/^log "AutoLedger backup sidecar started/,$d' "$script_dir/entrypoint.sh" > "$test_root/functions.sh"
DB_PASSWORD=test DB_HOST=test DB_USER=test DB_NAME=test
export DB_PASSWORD DB_HOST DB_USER DB_NAME
. "$test_root/functions.sh"

pg_dump() {
	printf 'database contents\n'
	if [ "$test_mode" = dump_failure ]; then return 1; fi
}
gzip() {
	if [ "$test_mode" = compression_failure ]; then
		cat > /dev/null
		printf 'partial archive'
		return 1
	fi
	command gzip "$@"
}
# Documents are independent of the database pipeline tested here.
tar() { : > "$2"; }

for test_mode in success dump_failure compression_failure; do
	BACKUP_DIR="$test_root/$test_mode"
	mkdir -p "$BACKUP_DIR"
	run_backup > "$test_root/$test_mode.log" 2>&1
	set -- "$BACKUP_DIR"/autoledger-db-*.sql.gz
	if [ "$test_mode" = success ]; then
		[ -f "$1" ]
		[ "$(command gzip -dc "$1")" = 'database contents' ]
		grep -q 'database dump OK' "$test_root/$test_mode.log"
	else
		[ ! -e "$1" ]
		[ -z "$(find "$BACKUP_DIR" -name 'autoledger-db-*.tmp' -print)" ]
		! grep -q 'database dump OK' "$test_root/$test_mode.log"
		grep -q 'ERROR: database dump or compression failed' "$test_root/$test_mode.log"
	fi
	printf 'PASS: %s\n' "$test_mode"
done
