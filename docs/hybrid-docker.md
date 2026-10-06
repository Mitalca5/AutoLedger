# Hybrid fork: Docker installation

Run these commands on your Docker host (Docker Compose v2 and Git required):

```sh
git clone --branch homelab/hybrid-docker --single-branch https://github.com/Mitalca5/AutoLedger.git autoledger-hybrid
cd autoledger-hybrid
cp .env.homelab.example .env
# Edit .env: set AUTOLEDGER_BASE_URL to the address used by your browser.
docker compose -f docker-compose.yml -f docker-compose.hybrid.yml up -d --build
docker compose -f docker-compose.yml -f docker-compose.hybrid.yml ps
```

Open http://YOUR_SERVER_IP:8080 and create your account. The initial build compiles the frontend and Go backend and may take several minutes. Docker builds for the host CPU architecture (amd64 or arm64); no TeslaMate is required for manual tracking.

The app and backup images are built locally as `autoledger-hybrid:local` and `autoledger-hybrid-backup:local`. They are not published to a registry. PostgreSQL uses the upstream `postgres:16-alpine` image. Compose generates the initial secrets and keeps the database, documents, secrets and backups in persistent Docker volumes.

## Update

From the same directory, after changes have been integrated into this deployment branch:

```sh
git pull --ff-only
docker compose -f docker-compose.yml -f docker-compose.hybrid.yml up -d --build
```

This deployment branch includes the hybrid comparison changes and the upstream commit cfc95179968bb152f02954ff5dd65dd17994e60f. Future commits on other branches are not integrated automatically.

## Stop and logs

```sh
docker compose -f docker-compose.yml -f docker-compose.hybrid.yml logs --tail=100 api
docker compose -f docker-compose.yml -f docker-compose.hybrid.yml down
```

Do not add `--volumes` / `-v` to `down` if you want to retain your data.

## Move the images to another Docker host

Use a build machine with the same CPU architecture as the destination:

```sh
docker save autoledger-hybrid:local autoledger-hybrid-backup:local | gzip > autoledger-hybrid-images.tar.gz
```

Copy the archive, both Compose files and your .env to the destination. There:

```sh
gunzip -c autoledger-hybrid-images.tar.gz | docker load
docker compose -f docker-compose.yml -f docker-compose.hybrid.yml up -d --no-build
```

PostgreSQL is pulled separately. Copying images does not copy existing data.
