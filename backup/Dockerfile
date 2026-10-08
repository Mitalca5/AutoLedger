# Based on postgres:18-alpine so pg_dump matches the server version exactly
# (avoids client/server version-mismatch dump failures).
FROM postgres:18-alpine

# Reuse the "postgres" user already created by the base image instead of running as root.
RUN mkdir -p /backups && chown postgres:postgres /backups

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER postgres

ENTRYPOINT ["/entrypoint.sh"]
