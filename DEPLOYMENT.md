# Deployment Guide

Production deployment guide for openByte speed test server.

## Prerequisites

- Go 1.26.5+ installed
- SSH access to production server
- Root or sudo access for service installation and firewall changes
- Firewall rules configured (port 8080)

## Quick Deploy

```bash
# Build binaries
make build

# Copy to server (web assets are embedded in the binary)
scp bin/openbyte user@server:/opt/openbyte/

# SSH to server
ssh user@server

# Create systemd service
sudo nano /etc/systemd/system/openbyte.service
```

## CI Deploy (GHCR + SSH)

Publish Docker image to GitHub Container Registry (GHCR) and deploy via SSH.

### 1. GitHub Actions workflow

Workflow lives at `.github/workflows/ci.yml`:

- Runs `go test ./...`
- Builds and pushes `ghcr.io/<owner>/openbyte:edge` and `:SHA` on `main`
- Optionally SSH deploys to your server

### 2. Required secrets (GitHub repo settings)

**Repository variables**

- `SSH_HOST` — server hostname/IP
- `SSH_HOST_FINGERPRINT` — SSH host key fingerprint in SHA256 form (for example `SHA256:abcd...`)
- `SSH_USER` — SSH user
- `SSH_PORT` — optional (default 22)
- `REMOTE_DIR` — path containing `docker/docker-compose.ghcr.yaml` (e.g., `/opt/openbyte`)
- `GHCR_USERNAME` — GHCR username (e.g., `SaveEnergy`)
- `SERVER_NAME` — optional display name used by CI/release deploys (e.g., `Frankfurt 10G`)

Notes:

- CI/release workflows derive `GHCR_OWNER` from the repository owner.
- CI deploy sets `IMAGE_TAG` to the commit SHA; release deploy sets `IMAGE_TAG` to the semver tag.
- If `SERVER_NAME` is set as a repository variable, it overrides the remote `.env` value for CI/release deploys.
- CI/release deploys fail closed if the scanned SSH host key fingerprint does not match `SSH_HOST_FINGERPRINT`.

Record the fingerprint out-of-band before enabling deploys:

```bash
ssh-keyscan -t ed25519 your-server.example.com | ssh-keygen -lf - -E sha256
```

**GHCR pull on server**

- `GHCR_TOKEN` — PAT with `read:packages` scope

**Secrets**

- `SSH_KEY` — private key (no passphrase) with Docker access

### 3. Server setup

On the server:

```bash
mkdir -p /opt/openbyte
cd /opt/openbyte
```

Copy `docker/docker-compose.ghcr.yaml`, `docker/docker-compose.ghcr.traefik.yaml`, and `docker/traefik-openbyte.yaml`, then create a `.env` with runtime values:

```bash
SERVER_NAME="Frankfurt 10G"
ALLOWED_ORIGINS="https://speedtest.example.com"
```

Then, GH Actions can SSH in and run:

```bash
docker compose -f docker/docker-compose.ghcr.yaml -f docker/docker-compose.ghcr.traefik.yaml pull
docker compose -f docker/docker-compose.ghcr.yaml -f docker/docker-compose.ghcr.traefik.yaml up -d
```

CI deploy path is Traefik-based (the workflow syncs and uses both compose files). It creates the external `traefik` network when needed and injects its exact IPAM subnet into openByte's trusted-proxy configuration.

Workflow behavior details (important for on-call):

- Deployment pins the previous openByte image under a temporary local rollback tag until health checks pass.
- Health gate requires both openByte and Traefik to be running and healthy for up to 20 attempts (3s interval, ~60s window).
- On compose failure, image mismatch, or a failed health gate, the workflow recreates only openByte from the pinned image when a previous container exists, verifies that restored image is healthy, then exits non-zero. A failed first deployment is left in place for inspection because there is nothing to restore.
- GHCR session is logged out on script exit via shell trap.

For direct HTTP access without Traefik, run only the base GHCR compose file:

```bash
docker compose -f docker/docker-compose.ghcr.yaml pull
docker compose -f docker/docker-compose.ghcr.yaml up -d
```

## Release Pipeline (SemVer)

SemVer release workflow: `.github/workflows/release.yml`.

### 1. Tag and push

```bash
git tag v1.2.3
git push origin v1.2.3
```

Release tags must point to commits already reachable from `origin/main`; the release workflow verifies tag ancestry before publishing artifacts.

### 2. Outputs

- **Binaries**: GitHub Release assets for each OS/arch
  - `openbyte_<version>_<os>_<arch>.tar.gz` (linux/darwin)
  - `openbyte_<version>_<os>_<arch>.zip` (windows)
- **Container images** on GHCR:
  - `:1.2.3`, `:1.2`, `:1`, `:latest`

## Systemd Service

```ini
[Unit]
Description=openByte Speed Test Server
After=network.target

[Service]
Type=simple
User=openbyte
Group=openbyte
WorkingDirectory=/opt/openbyte
ExecStart=/opt/openbyte/openbyte server
Restart=always
RestartSec=5

Environment="PORT=8080"
Environment="CAPACITY_GBPS=25"
Environment="MAX_CONCURRENT_PER_IP=64"
Environment="RATE_LIMIT_PER_IP=100"
Environment="GLOBAL_RATE_LIMIT=1000"
Environment="TRUST_PROXY_HEADERS=true"
Environment="TRUSTED_PROXY_CIDRS=10.0.0.0/8,192.168.0.0/16"
Environment="ALLOWED_ORIGINS=https://speedtest.example.com"
Environment="DATA_DIR=/opt/openbyte/data"
Environment="MAX_TEST_DURATION=300s"

[Install]
WantedBy=multi-user.target
```

## Deployment Steps

### 1. Server Setup

```bash
# Create user
sudo useradd -r -s /sbin/nologin openbyte
sudo mkdir -p /opt/openbyte
sudo chown openbyte:openbyte /opt/openbyte
```

### 2. Build & Transfer

```bash
# Local machine
make build
tar -C bin -czf openbyte.tar.gz openbyte

# Transfer
scp openbyte.tar.gz user@server:/tmp/
ssh user@server "cd /opt/openbyte && sudo tar xzf /tmp/openbyte.tar.gz && sudo chown -R openbyte:openbyte /opt/openbyte"
```

### 3. Configure Firewall

```bash
# UFW
sudo ufw allow 8080/tcp

# firewalld
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --reload
```

### 4. Start Service

```bash
sudo systemctl daemon-reload
sudo systemctl enable openbyte
sudo systemctl start openbyte
sudo systemctl status openbyte
```

### 5. Verify

```bash
# Health check
curl http://localhost:8080/health

# Verify the public endpoint
./bin/openbyte check --json https://speedtest.example.com
```

## Reverse Proxy (Nginx)

```nginx
server {
    listen 80;
    server_name speedtest.example.com;
    client_max_body_size 70m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_request_buffering off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

**Reverse proxy upload limits (important):**

Speed tests use adaptive request bodies from 8 MiB through 64 MiB. Many reverse proxies default to 1 MiB and will reject or buffer uploads, which can produce errors or unrealistic upload speeds.

Minimum recommendations:

- Increase the maximum request body above 64 MiB (for example, `client_max_body_size 70m;` in Nginx).
- Disable request buffering for upload so the proxy does not turn a live measurement into store-and-forward:
  - Nginx: `location /api/v1/upload { proxy_request_buffering off; proxy_http_version 1.1; }`
- If you enable HTTP/2 or HTTP/3 at the proxy edge, keep the upstream to OpenByte as HTTP/1.1 for `/api/v1/upload`.

When running behind a proxy, set `TRUST_PROXY_HEADERS=true` and `TRUSTED_PROXY_CIDRS` to the proxy IP ranges so rate limiting and client IP logging are accurate.

## 25 Gbit/s Deployment Notes

For 25 Gbit/s tests, prefer a direct data path for `/api/v1/download`, `/api/v1/upload`, and `/api/v1/ping`:

- For bare-metal deployments, run openByte with direct TLS (`TLS_CERT_FILE` and `TLS_KEY_FILE`) or route the speed-test API paths around the TLS proxy. A reverse proxy copies every byte and can become the bottleneck before openByte does.
- For local proof runs, `TLS_AUTO_GEN=1` serves HTTPS with an ephemeral self-signed localhost certificate. Do not use it for public deployments.
- The provided Docker Compose health checks always probe plain HTTP and do not mount certificate files. Use the bundled Traefik overlay for Docker TLS, or supply custom certificate mounts plus an HTTPS health check before enabling direct TLS in the container.
- Keep `HTTP2_ENABLED=true` by default, but benchmark `HTTP2_ENABLED=false` for browser speed-test endpoints on target hardware. On the 4-vCPU Cloud VM, direct TLS with HTTP/1.1 measured **20.38 Gbit/s download / 13.11 Gbit/s upload** median, while direct TLS with HTTP/2 measured **11.26 Gbit/s / 8.77 Gbit/s**.
- The bundled Traefik HTTPS routers default to `TRAEFIK_TLS_OPTIONS=openbyte-h1@file`, which advertises HTTP/1.1 only for the browser data path. On the 4-vCPU Cloud VM, local Traefik h1-only measured **15.11 Gbit/s download / 12.66 Gbit/s upload** median, while Traefik h2 measured **9.95 Gbit/s / 7.97 Gbit/s**. Set `TRAEFIK_TLS_OPTIONS=openbyte-h2@file` only when you intentionally want to compare Traefik HTTP/2.
- Use host networking for Docker deployments when the server is dedicated to speed tests. Docker bridge/NAT adds per-packet CPU cost at high packet rates.
- Keep `CAPACITY_GBPS=25` and `MAX_CONCURRENT_PER_IP=64` unless you deliberately restrict single-client stream count.
- Disable request buffering on upload routes. Buffering changes a live upload test into a proxy store-and-forward test.

Tune the host network stack to match the bandwidth-delay product of the link. Example starting point:

```bash
sudo sysctl -w net.core.rmem_max=134217728
sudo sysctl -w net.core.wmem_max=134217728
sudo sysctl -w net.ipv4.tcp_rmem='4096 131072 134217728'
sudo sysctl -w net.ipv4.tcp_wmem='4096 65536 134217728'
sudo sysctl -w net.ipv4.tcp_congestion_control=bbr
```

Also verify NIC RSS queues, IRQ affinity, CPU frequency governor, and jumbo MTU only when the full path supports it.

## Reverse Proxy (Traefik)

Traefik integration via Docker labels. openByte only needs HTTP(S) proxying to container port 8080.

```bash
# Source-build deployment; the overlay starts Traefik and manages its network
cd docker
TRAEFIK_HOST=speedtest.example.com \
  docker compose -f docker-compose.yaml -f docker-compose.traefik.yaml up -d
```

The GHCR overlay declares `traefik` as an external network; create it before a manual GHCR deployment. CI and release deployments create it and trust its exact subnet automatically. For a manual Traefik deployment, set `TRUST_PROXY_HEADERS=true` and set `TRUSTED_PROXY_CIDRS` to the inspected subnet rather than a blanket private range:

```bash
docker network inspect traefik --format '{{ (index .IPAM.Config 0).Subnet }}'
```

For reliable upload tests through Traefik:

- Ensure any request body limit is above the browser's 64 MiB maximum payload.
- Do not use Traefik buffering middleware on `/api/v1/upload`; it prevents live upload streaming and can spill speed-test data to disk.

The provided Traefik compose files include dedicated upload routers without buffering middleware.

The HTTPS routers also use `docker/traefik-openbyte.yaml` via Traefik's file
provider. By default, `openbyte-h1@file` restricts ALPN to HTTP/1.1 for the
openByte routers, avoiding the slower h2 browser path measured on the Cloud VM.
Use `TRAEFIK_TLS_OPTIONS=openbyte-h2@file` for explicit h2 comparison runs.

**Environment variables:**

| Variable                 | Default                | Description                                |
| ------------------------ | ---------------------- | ------------------------------------------ |
| `TRAEFIK_HOST`           | `speedtest.localhost`  | Source-build overlay hostname              |
| `TRAEFIK_DASHBOARD_HOST` | `traefik.localhost`    | Dashboard hostname                         |
| `ACME_EMAIL`             | `admin@example.com`    | Let's Encrypt account email                |
| `TRAEFIK_TLS_OPTIONS`    | `openbyte-h1@file`     | TLS ALPN policy for openByte HTTPS routers |

The GHCR overlay uses `TRAEFIK_HOST_RULE` instead of `TRAEFIK_HOST`, allowing a compound Traefik rule for the main, IPv4, and IPv6 hosts.

**Important:** openByte serves the HTTP API and UI on one listener; expose or proxy only port 8080 (or the configured `PORT`), whether that listener is plain HTTP or direct TLS.

## IPv4/IPv6 Detection

openByte shows the client's IPv4 and IPv6 addresses separately using dedicated single-stack subdomains.

### How It Works

On page load, the UI runs three parallel probes:

1. **Main ping** (`/api/v1/ping`) — captures whichever address family the browser chose (Happy Eyeballs).
2. **IPv4 probe** (`v4.<hostname>`) — A-only DNS record forces IPv4. Shows IPv4 address.
3. **IPv6 probe** (`v6.<hostname>`) — AAAA-only DNS record forces IPv6. Shows IPv6 address.

The results section displays both addresses (or `-` if unavailable).

### DNS Setup

Add two single-stack subdomains. Each must have **only one** record type to force the address family.

```
v4.speedtest.example.com.  A     203.0.113.10
v6.speedtest.example.com.  AAAA  2001:db8::1
```

| Subdomain     | Record    | Value       | Notes          |
| ------------- | --------- | ----------- | -------------- |
| `v4.<domain>` | A only    | Server IPv4 | No AAAA record |
| `v6.<domain>` | AAAA only | Server IPv6 | No A record    |

Verify:

```bash
# IPv4 probe — should return only A record
dig A v4.speedtest.example.com
curl -4 https://v4.speedtest.example.com/api/v1/ping

# IPv6 probe — should return only AAAA record
dig AAAA v6.speedtest.example.com
curl -6 https://v6.speedtest.example.com/api/v1/ping
```

### Traefik Configuration

The provided GHCR overlay supports a compound host rule. Add both subdomains to its `.env`:

```bash
TRAEFIK_HOST_RULE="Host(`speedtest.example.com`) || Host(`v4.speedtest.example.com`) || Host(`v6.speedtest.example.com`)"
```

The source-build overlay accepts only one `TRAEFIK_HOST`; use the GHCR overlay or customize its router labels when you need both probe subdomains. Ensure every configured hostname reaches the same Traefik instance on ports 80 and 443 so it can complete certificate validation.

### Nginx Configuration

Add server blocks for both subdomains:

```nginx
# IPv4-only
server {
    listen 80;
    listen 443 ssl;
    server_name v4.speedtest.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# IPv6-only
server {
    listen [::]:80;
    listen [::]:443 ssl;
    server_name v6.speedtest.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Firewall

Ensure both IPv4 and IPv6 traffic is allowed on HTTP/HTTPS ports:

```bash
# UFW (allows both IPv4 and IPv6 by default)
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# firewalld
sudo firewall-cmd --permanent --add-port=80/tcp
sudo firewall-cmd --permanent --add-port=443/tcp
sudo firewall-cmd --reload
```

### v6 probe console errors

If the browser console shows `v6.<domain> ... ERR_NAME_NOT_RESOLVED` or IPv6 connection failures, ensure:

- `v6.<domain>` has an **AAAA-only** DNS record (no A record).
- Traefik listens on IPv6 (`443/tcp` allowed in host/cloud firewall for IPv6).
- `v6.<domain>` is included in `TRAEFIK_HOST_RULE`.

These probe failures do not affect the main page padlock, but IPv6-only clients need working AAAA + Traefik IPv6.

## Monitoring

### Logs

```bash
sudo journalctl -u openbyte -f
```

### Health Monitoring

```bash
# Health check script
#!/bin/bash
if ! curl -f http://localhost:8080/health > /dev/null 2>&1; then
    systemctl restart openbyte
fi
```

## Troubleshooting

### Port Already in Use

```bash
# Find process
sudo lsof -i :8080
sudo kill <PID>
```

### Permission Denied

```bash
# Check user
ps aux | grep openbyte

# Fix permissions
sudo chown -R openbyte:openbyte /opt/openbyte
```

### High CPU Usage

- Reduce `CAPACITY_GBPS` (HTTP concurrency scales with it)
- Check for connection leaks
- Monitor with `htop`

## Rollback

```bash
# Stop service
sudo systemctl stop openbyte

# Restore previous binary
sudo cp /opt/openbyte/openbyte.backup /opt/openbyte/openbyte

# Start service
sudo systemctl start openbyte
```
