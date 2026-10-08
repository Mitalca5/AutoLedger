# Upgrades a database volume written by PostgreSQL 16 so that PostgreSQL 18 can use it.
# Based on postgres:16-alpine: the old server binaries are needed to read the old data directory, and a 16 client
# restores a 16 dump into a 18 server without any compatibility concern.
FROM postgres:16-alpine

COPY upgrade.sh /usr/local/bin/db-upgrade
COPY restore.sh /usr/local/bin/db-restore
RUN chmod +x /usr/local/bin/db-upgrade /usr/local/bin/db-restore

# Runs as root: it must read the data directory owned by the postgres user, then hand its backups back to it.
ENTRYPOINT ["/usr/local/bin/db-upgrade"]
