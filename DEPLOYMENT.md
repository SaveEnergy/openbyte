# Deployment Guide

Production deployment guide for openByte speed test server.

## Prerequisites

- Go 1.25+ installed
- SSH access to production server
- Root or sudo access for port binding (<1024)
- Firewall rules configured (ports 8080, 8081, 8082)

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

## Production Deploy Script (Docker + Traefik)

Use `deploy-openbyte-prod.sh` to rsync code and run `docker compose` on the host.

```bash
ACME_EMAIL="you@example.com" \
TRUSTED_PROXY_CIDRS="172.20.0.0/16" \
./deploy-openbyte-prod.sh
```

Optional overrides:

```bash
HOST=49.12.213.184 USER=oezmen DOMAIN=openbyte.sqrtops.de REMOTE_DIR=/opt/openbyte ./deploy-openbyte-prod.sh
```

## CI Deploy (GHCR + SSH)

Publish Docker image to GitHub Container Registry (GHCR) and deploy via SSH.

### 1. GitHub Actions workflow

Workflow lives at `.github/workflows/ci.yml`:
- Runs `go test ./...`
- Builds and pushes `ghcr.io/<owner>/openbyte:latest` and `:SHA`
- Optionally SSH deploys to your server

### 2. Required secrets (GitHub repo settings)

**Repository variables**
- `SSH_HOST` — server hostname/IP
- `SSH_USER` — SSH user
- `SSH_PORT` — optional (default 22)
- `REMOTE_DIR` — path containing `docker/docker-compose.ghcr.yaml` (e.g., `/opt/openbyte`)
- `GHCR_USERNAME` — GHCR username (e.g., `SaveEnergy`)
- `GHCR_OWNER` — image owner/namespace (e.g., `SaveEnergy`)
- `IMAGE_TAG` — optional image tag override (e.g., `1.2.3`, default `edge`)

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

Copy `docker/docker-compose.ghcr.yaml` and create a `.env` with runtime values:

```bash
SERVER_ID=openbyte-1
SERVER_NAME="OpenByte Server"
SERVER_LOCATION="EU"
PUBLIC_HOST="speedtest.example.com"
ALLOWED_ORIGINS="https://speedtest.example.com"
TRUST_PROXY_HEADERS=true
TRUSTED_PROXY_CIDRS="10.0.0.0/8,192.168.0.0/16"
GHCR_OWNER=SaveEnergy
IMAGE_TAG=edge
```

Then, GH Actions can SSH in and run:

```bash
docker compose -f docker/docker-compose.ghcr.yaml pull
docker compose -f docker/docker-compose.ghcr.yaml up -d
```

For direct HTTP access without Traefik, include the web override:

```bash
docker compose -f docker/docker-compose.ghcr.yaml -f docker/docker-compose.ghcr.web.yaml pull
docker compose -f docker/docker-compose.ghcr.yaml -f docker/docker-compose.ghcr.web.yaml up -d
```

## Release Pipeline (SemVer)

SemVer release workflow: `.github/workflows/release.yml`.

### 1. Tag and push

```bash
git tag v1.2.3
git push origin v1.2.3
```

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

Environment="SERVER_ID=prod-1"
Environment="SERVER_NAME=Production Server"
Environment="SERVER_LOCATION=US-East"
Environment="PUBLIC_HOST=speedtest.example.com"
Environment="PORT=8080"
Environment="TCP_TEST_PORT=8081"
Environment="UDP_TEST_PORT=8082"
Environment="CAPACITY_GBPS=25"
Environment="RATE_LIMIT_PER_IP=100"
Environment="GLOBAL_RATE_LIMIT=1000"
Environment="TRUST_PROXY_HEADERS=true"
Environment="TRUSTED_PROXY_CIDRS=10.0.0.0/8,192.168.0.0/16"
Environment="ALLOWED_ORIGINS=https://speedtest.example.com"
Environment="DATA_DIR=/opt/openbyte/data"

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

# Create log directory
sudo mkdir -p /var/log/openbyte
sudo chown openbyte:openbyte /var/log/openbyte
```

### 2. Build & Transfer

```bash
# Local machine
make build
tar czf openbyte.tar.gz bin/openbyte

# Transfer
scp openbyte.tar.gz user@server:/tmp/
ssh user@server "cd /opt/openbyte && sudo tar xzf /tmp/openbyte.tar.gz && sudo chown -R openbyte:openbyte /opt/openbyte"
```

### 3. Configure Firewall

```bash
# UFW
sudo ufw allow 8080/tcp
sudo ufw allow 8081/tcp
sudo ufw allow 8082/tcp
sudo ufw allow 8082/udp

# firewalld
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=8081/tcp
sudo firewall-cmd --permanent --add-port=8082/tcp
sudo firewall-cmd --permanent --add-port=8082/udp
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

# Test from client
./bin/openbyte client -S production -d download -t 10
```

## Multi-Server Deployment

### Server Configuration

Each server needs unique identity:

```bash
# Server 1 (NYC)
SERVER_ID=nyc-1
SERVER_NAME="New York"
SERVER_LOCATION="US-East"
PUBLIC_HOST=nyc.speedtest.example.com

# Server 2 (AMS)
SERVER_ID=ams-1
SERVER_NAME="Amsterdam"
SERVER_LOCATION="EU-West"
PUBLIC_HOST=ams.speedtest.example.com
```

### Client Configuration

Update `~/.config/openbyte/config.yaml`:

```yaml
default_server: nyc
servers:
  nyc:
    url: https://nyc.speedtest.example.com
    name: "New York"
  ams:
    url: https://ams.speedtest.example.com
    name: "Amsterdam"
```

## Reverse Proxy (Nginx)

```nginx
server {
    listen 80;
    server_name speedtest.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

**Reverse proxy upload limits (important):**

Speed tests upload multi-megabyte request bodies (default ~4MB per request, repeated). Many reverse proxies default to 1MB and will reject or buffer uploads, which can produce errors or unrealistic upload speeds.

Minimum recommendations:
- Increase max request body (e.g. `client_max_body_size 35m;` in Nginx).
- Disable request buffering for upload to avoid early responses:
  - Nginx: `location /api/v1/upload { proxy_request_buffering off; proxy_http_version 1.1; }`
- If you enable HTTP/2 or HTTP/3 at the proxy edge, keep the upstream to OpenByte as HTTP/1.1 for `/api/v1/upload`.

When running behind a proxy, set `TRUST_PROXY_HEADERS=true` and `TRUSTED_PROXY_CIDRS` to the proxy IP ranges so rate limiting and client IP logging are accurate.

## Reverse Proxy (Traefik)

Traefik integration via Docker labels. TCP/UDP test ports must stay exposed directly.

```bash
# Create traefik network first
docker network create traefik

# Deploy with Traefik labels
cd docker
TRAEFIK_HOST=speedtest.example.com docker compose up -d

# Or with HTTPS override
docker compose -f docker-compose.yaml -f docker-compose.traefik.yaml up -d
```

When running behind Traefik, set `TRUSTED_PROXY_CIDRS` to the Traefik network subnet:

```bash
docker network inspect traefik --format '{{ (index .IPAM.Config 0).Subnet }}'
```

For reliable upload tests through Traefik:
- Ensure the upload router allows large request bodies (e.g. 35MB).
- Apply buffering middleware only to `/api/v1/upload` to avoid impacting download streams.

The provided Traefik compose files include a dedicated upload router with a 35MB request body limit.

**Environment variables:**

| Variable | Default | Description |
|----------|---------|-------------|
| `TRAEFIK_HOST` | `speedtest.localhost` | Domain for HTTP routing |
| `TRAEFIK_ENTRYPOINT` | `web` | Traefik entrypoint name |
| `TRAEFIK_NETWORK` | `traefik` | External network name |
| `TRAEFIK_CERTRESOLVER` | `letsencrypt` | TLS cert resolver |

**Important:** TCP (8081) and UDP (8082) ports cannot be proxied through HTTP. They must be:
- Exposed directly on the host, or
- Configured as Traefik TCP/UDP routers (advanced)

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

| Subdomain | Record | Value | Notes |
|-----------|--------|-------|-------|
| `v4.<domain>` | A only | Server IPv4 | No AAAA record |
| `v6.<domain>` | AAAA only | Server IPv6 | No A record |

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

Add both subdomains to the Traefik host rule in `.env`:

```bash
TRAEFIK_HOST_RULE="Host(`speedtest.example.com`) || Host(`v4.speedtest.example.com`) || Host(`v6.speedtest.example.com`)"
```

Traefik auto-issues Let's Encrypt certificates for each subdomain. The IPv6 cert uses HTTP-01 challenge over IPv6 — ensure Traefik listens on IPv6 (default for `traefik:v3`).

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

## Monitoring

### Logs

```bash
# View logs
sudo journalctl -u openbyte -f

# Log rotation
sudo nano /etc/logrotate.d/openbyte
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
