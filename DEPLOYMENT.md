# Deployment Guide

openByte exposes the UI and API on one HTTP port, `8080` by default. Web assets
are embedded in the binary.

## Docker

Build and run locally:

```bash
cd docker
docker compose up -d
```

Run the published image directly:

```bash
cd docker
docker compose -f docker-compose.ghcr.yaml up -d
```

Add the shared Traefik overlay to either base file:

```bash
cd docker
TRAEFIK_HOST_RULE='Host(`speedtest.example.com`)' \
  docker compose -f docker-compose.yaml -f docker-compose.traefik.yaml up -d

# Substitute docker-compose.ghcr.yaml to use the published image.
```

The overlay terminates TLS, keeps uploads unbuffered, and defaults the openByte
routers to the measured-faster `openbyte-h1@file` ALPN policy. Set
`TRAEFIK_TLS_OPTIONS=openbyte-h2@file` only for an intentional comparison.

## Automated GHCR deployment

Pushes to `main` publish `edge` and commit-SHA images. SemVer tags publish
release archives plus `X.Y.Z`, `X.Y`, `X`, and `latest` multi-architecture
images with SBOM and provenance.

Configure these GitHub repository variables:

- `SSH_HOST`, `SSH_USER`, optional `SSH_PORT`
- `SSH_HOST_FINGERPRINT` — expected SHA256 fingerprint of the host's ED25519 key
- `REMOTE_DIR` — deployment directory, for example `/opt/openbyte`
- `GHCR_USERNAME`
- optional `SERVER_NAME`

Configure these secrets:

- `SSH_KEY` — unencrypted deployment key with Docker access
- `GHCR_TOKEN` — token with `read:packages`

Record the fingerprint through a trusted channel before enabling deployment:

```bash
ssh-keyscan -t ed25519 server.example.com | ssh-keygen -lf - -E sha256
```

Create the remote directory and `.env` once:

```bash
mkdir -p /opt/openbyte
cat >/opt/openbyte/.env <<'EOF'
SERVER_NAME="Frankfurt 25G"
TRAEFIK_HOST_RULE="Host(`speedtest.example.com`) || Host(`v4.speedtest.example.com`) || Host(`v6.speedtest.example.com`)"
EOF
```

Both CI and release call `scripts/deploy/deploy.sh`. It validates the host key,
streams the compose bundle and its checksum manifest over one SSH connection,
verifies every remote checksum, pulls the immutable image tag, and deploys.
The host script inspects the active `traefik` network and overrides
`TRUST_PROXY_HEADERS` plus `TRUSTED_PROXY_CIDRS` with its exact IPv4/IPv6 IPAM
subnets. Deployment fails before replacing the application if no subnet exists.

The health gate requires both openByte and Traefik to be running and healthy.
Before replacement, the current openByte image is pinned under a temporary
rollback tag. Compose failure, image mismatch, or failed health checks restore
that image and verify it. A failed first deployment remains available for
inspection because no previous image exists.

## Reverse proxies

Browser uploads use adaptive request bodies up to 64 MiB. Every proxy must:

- accept bodies larger than 64 MiB;
- disable request buffering on `/api/v1/upload`;
- forward the original client address; and
- use HTTP/1.1 upstream unless target-hardware measurements prove otherwise.

When proxy headers are enabled, restrict trust to the proxy network:

```bash
TRUST_PROXY_HEADERS=true
TRUSTED_PROXY_CIDRS="172.18.0.0/16"
```

For Docker, obtain that exact subnet with `docker network inspect traefik`
instead of trusting a blanket private range.

Minimal Nginx example:

```nginx
server {
    listen 443 ssl;
    server_name speedtest.example.com v4.speedtest.example.com v6.speedtest.example.com;
    client_max_body_size 70m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_request_buffering off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## IPv4 and IPv6 discovery

The UI deliberately probes both address families on page load so it also works
as a quick public-IP lookup. Configure two single-stack hostnames:

```text
v4.speedtest.example.com.  A     203.0.113.10
v6.speedtest.example.com.  AAAA  2001:db8::10
```

`v4` must have no AAAA record and `v6` must have no A record. Route all three
hosts to the same server:

```bash
TRAEFIK_HOST_RULE='Host(`speedtest.example.com`) || Host(`v4.speedtest.example.com`) || Host(`v6.speedtest.example.com`)'
```

Verify DNS and reachability independently:

```bash
dig A v4.speedtest.example.com
dig AAAA v6.speedtest.example.com
curl -4 https://v4.speedtest.example.com/api/v1/ping
curl -6 https://v6.speedtest.example.com/api/v1/ping
```

A failed single-stack probe displays `Not detected` for that family without
preventing the speed test. Make sure HTTP/HTTPS firewall rules permit both IPv4
and IPv6.

## Bare-metal service

```ini
[Unit]
Description=openByte Speed Test Server
After=network-online.target

[Service]
User=openbyte
Group=openbyte
WorkingDirectory=/opt/openbyte
ExecStart=/opt/openbyte/openbyte server
Environment="PORT=8080"
Environment="CAPACITY_GBPS=25"
Environment="DATA_DIR=/opt/openbyte/data"
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Install the binary, create `/opt/openbyte/data`, enable the service, and expose
only the application port or reverse-proxy ports required by the deployment.

## High-capacity links

- Prefer direct TLS or a bypass for ping/download/upload when proxy CPU becomes
  the bottleneck.
- Keep `CAPACITY_GBPS=25` and `MAX_CONCURRENT_PER_IP=64` unless deliberately
  restricting a client.
- Benchmark `HTTP2_ENABLED=false` and Traefik h1/h2 on the target hardware.
- Use host networking only after measuring Docker bridge/NAT overhead.
- Tune socket buffers, RSS queues, IRQ affinity, CPU governor, congestion
  control, and MTU for the actual bandwidth-delay product.

The committed performance harness and prior protocol evidence are documented in
[`test/perf/README.md`](test/perf/README.md).

## Verification and monitoring

```bash
curl -f http://127.0.0.1:8080/health
curl -f http://127.0.0.1:8080/api/v1/version
sudo journalctl -u openbyte -f   # bare metal
docker compose ps               # containers
```
