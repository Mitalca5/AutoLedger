#!/bin/sh
# Periodic backup sidecar: dumps the PostgreSQL database and archives the
# documents volume on a fixed interval, with basic retention-based pruning.
# Runs as a simple loop rather than cron so failures are visible directly in
# `docker logs` instead of being swallowed by a cron daemon.
set -eu

INTERVAL_HOURS="${BACKUP_INTERVAL_HOURS:-24}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
BACKUP_DIR="/backups"

log() {
	echo "[backup] $(date -u +%FT%TZ) $*"
}

run_backup() {
	ts="$(date -u +%Y%m%d-%H%M%S)"
	db_dump="${BACKUP_DIR}/autoledger-db-${ts}.sql.gz"
	docs_archive="${BACKUP_DIR}/autoledger-documents-${ts}.tar.gz"

	log "starting backup cycle..."

	if PGPASSWORD="${DB_PASSWORD}" pg_dump -h "${DB_HOST}" -p "${DB_PORT:-5432}" -U "${DB_USER}" -d "${DB_NAME}" | gzip > "${db_dump}.tmp"; then
		mv "${db_dump}.tmp" "${db_dump}"
		log "database dump OK -> ${db_dump} ($(du -h "${db_dump}" | cut -f1))"
	else
		rm -f "${db_dump}.tmp"
		log "ERROR: pg_dump failed, no database backup produced this cycle"
	fi

	if tar -czf "${docs_archive}.tmp" -C /data documents 2>/dev/null; then
		mv "${docs_archive}.tmp" "${docs_archive}"
		log "documents archive OK -> ${docs_archive} ($(du -h "${docs_archive}" | cut -f1))"
	else
		rm -f "${docs_archive}.tmp"
		log "ERROR: documents archive failed this cycle"
	fi

	log "pruning backups older than ${RETENTION_DAYS} day(s)..."
	find "${BACKUP_DIR}" -maxdepth 1 \( -name 'autoledger-*.gz' -o -name 'teslacost-*.gz' \) -mtime "+${RETENTION_DAYS}" -print -delete
}

log "AutoLedger backup sidecar started (every ${INTERVAL_HOURS}h, retention ${RETENTION_DAYS}d, target dir ${BACKUP_DIR})"

while true; do
	run_backup || log "ERROR: backup cycle exited unexpectedly, will retry next interval"
	sleep "$((INTERVAL_HOURS * 3600))"
done
