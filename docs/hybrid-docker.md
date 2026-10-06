# Hybrid fork: Docker installation

## Portainer / prebuilt registry images

The application and backup images are published to GitHub Container Registry:

- `ghcr.io/mitalca5/autoledger:hybrid`
- `ghcr.io/mitalca5/autoledger-backup:hybrid`

Both images support Linux amd64 and arm64. Anonymous pulls were verified after the first successful publication.

Copy `docker-compose.portainer.yml` into your deployment Git repository. It is standalone: no source checkout, build step, override file or local bind-mounted configuration is needed.

In Portainer, create a Docker Standalone stack from your Git repository and select this Compose file. Set these stack environment variables:

| Variable | Example | Purpose |
| --- | --- | --- |
| AUTOLEDGER_BASE_URL | http://192.168.1.50:8080 | Actual browser URL; use your HTTPS domain if served through a reverse proxy |
| AUTOLEDGER_PORT | 8080 | Host port |
| APP_TIMEZONE | America/Argentina/Buenos_Aires | Server timezone |
| HYBRID_TAG | hybrid | Published image tag |

Deploy the stack, open the configured URL and create your account. Compose initializes secrets and PostgreSQL and stores the database, documents, secrets and backups in named volumes. PostgreSQL has no published host port in this stack.

To update, use Portainer's update/redeploy operation and enable pulling the image again. The `hybrid` tag follows successful publications from `homelab/hybrid-docker`; commits on upstream or other branches do not update it automatically.

For a fixed version, set HYBRID_TAG to `hybrid-<full commit SHA>`. The same tag is published for both images. The first successful build published `hybrid-c8f29eb02cb39bdd45e4d06fb129fb2c447a5da8`.

The publishing workflow uses the repository's GitHub Actions token; no additional personal token is required.

This Compose targets Docker Standalone, not Docker Swarm. Keep the data volumes when removing or recreating the stack.

## Optional local build

If you prefer building on your Docker host:

```sh
git clone --branch homelab/hybrid-docker --single-branch https://github.com/Mitalca5/AutoLedger.git autoledger-hybrid
cd autoledger-hybrid
cp .env.homelab.example .env
# Edit .env: set AUTOLEDGER_BASE_URL to the address used by your browser.
docker compose -f docker-compose.yml -f docker-compose.hybrid.yml up -d --build
```

This alternative builds `autoledger-hybrid:local` and `autoledger-hybrid-backup:local` for the host CPU architecture.
