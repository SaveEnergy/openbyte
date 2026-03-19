#!/usr/bin/env bash
# Pull image DEPLOY_TAG on remote host and recreate openbyte container. Run from repo root.
# Required env: SSH_HOST, SSH_HOST_FINGERPRINT, SSH_USER, SSH_PORT, REMOTE_DIR, SSH_KEY,
# GHCR_USERNAME, GHCR_TOKEN, DEPLOY_TAG, GITHUB_REPOSITORY_OWNER
set -euo pipefail
[[ -n "${REMOTE_DIR:-}" ]] || { echo "REMOTE_DIR not set"; exit 1; }
[[ -n "${DEPLOY_TAG:-}" ]] || { echo "DEPLOY_TAG not set"; exit 1; }
[[ -n "${GITHUB_REPOSITORY_OWNER:-}" ]] || { echo "GITHUB_REPOSITORY_OWNER not set"; exit 1; }

SSH_TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$SSH_TMP_DIR"' EXIT
printf '%s\n' "$SSH_KEY" > "$SSH_TMP_DIR/id_rsa"
chmod 600 "$SSH_TMP_DIR/id_rsa"
touch "$SSH_TMP_DIR/known_hosts"
timeout 15 ssh-keyscan -t ed25519 -p "${SSH_PORT:-22}" -H "$SSH_HOST" > "$SSH_TMP_DIR/known_hosts.tmp"
[[ -s "$SSH_TMP_DIR/known_hosts.tmp" ]] || { echo "ssh-keyscan returned no ed25519 host keys"; exit 1; }
scanned_fingerprint="$(ssh-keygen -lf "$SSH_TMP_DIR/known_hosts.tmp" -E sha256 | awk 'NR==1 {print $2}')"
[[ "$scanned_fingerprint" == "$SSH_HOST_FINGERPRINT" ]] || {
  echo "ssh host fingerprint mismatch"
  echo "expected=$SSH_HOST_FINGERPRINT"
  echo "got=${scanned_fingerprint:-missing}"
  exit 1
}
cat "$SSH_TMP_DIR/known_hosts.tmp" >> "$SSH_TMP_DIR/known_hosts"
rm -f "$SSH_TMP_DIR/known_hosts.tmp"

OWNER_LC="$(printf '%s' "${GITHUB_REPOSITORY_OWNER}" | tr '[:upper:]' '[:lower:]')"
REMOTE_DIR_Q="$(printf '%q' "$REMOTE_DIR")"
GHCR_USERNAME_Q="$(printf '%q' "$GHCR_USERNAME")"
GHCR_TOKEN_Q="$(printf '%q' "$GHCR_TOKEN")"
OWNER_LC_Q="$(printf '%q' "$OWNER_LC")"
DEPLOY_TAG_Q="$(printf '%q' "$DEPLOY_TAG")"

ssh -i "$SSH_TMP_DIR/id_rsa" \
  -o UserKnownHostsFile="$SSH_TMP_DIR/known_hosts" \
  -o IdentitiesOnly=yes \
  -p "${SSH_PORT:-22}" \
  "${SSH_USER}@${SSH_HOST}" \
  "REMOTE_DIR=$REMOTE_DIR_Q GHCR_USERNAME=$GHCR_USERNAME_Q GHCR_TOKEN=$GHCR_TOKEN_Q OWNER_LC=$OWNER_LC_Q DEPLOY_TAG=$DEPLOY_TAG_Q sh -seu" <<'EOF'
[ -n "$REMOTE_DIR" ] || { echo "REMOTE_DIR not set"; exit 1; }
test -f "$REMOTE_DIR/.env" || { echo "Missing .env at $REMOTE_DIR/.env"; exit 1; }
echo "$GHCR_TOKEN" | docker login ghcr.io -u "$GHCR_USERNAME" --password-stdin
trap 'docker logout ghcr.io >/dev/null 2>&1 || true' EXIT
cd "$REMOTE_DIR"
docker network inspect traefik >/dev/null 2>&1 || docker network create traefik >/dev/null 2>&1 || { echo "failed to create traefik network"; exit 1; }
docker network inspect traefik >/dev/null 2>&1 || { echo "traefik network missing after create"; exit 1; }
GHCR_OWNER="$OWNER_LC" IMAGE_TAG="$DEPLOY_TAG" docker compose --env-file "$REMOTE_DIR/.env" -f docker/docker-compose.ghcr.yaml -f docker/docker-compose.ghcr.traefik.yaml pull || { echo "docker compose pull failed"; exit 1; }
EXPECTED_IMAGE="ghcr.io/${OWNER_LC}/openbyte:${DEPLOY_TAG}"
expected_id=$(docker image inspect "$EXPECTED_IMAGE" -f '{{.Id}}' 2>/dev/null || true)
[ -n "$expected_id" ] || { echo "expected image missing after pull: $EXPECTED_IMAGE"; docker logout ghcr.io || true; exit 1; }
rollback_name="openbyte_rollback_prev"
rollback_enabled=0
if docker inspect openbyte >/dev/null 2>&1; then
  docker rm -f "$rollback_name" >/dev/null 2>&1 || true
  docker rename openbyte "$rollback_name"
  rollback_enabled=1
fi
GHCR_OWNER="$OWNER_LC" IMAGE_TAG="$DEPLOY_TAG" docker compose --env-file "$REMOTE_DIR/.env" -f docker/docker-compose.ghcr.yaml -f docker/docker-compose.ghcr.traefik.yaml up -d --force-recreate
running_id=$(docker inspect -f '{{.Image}}' openbyte 2>/dev/null || true)
if [ -z "$expected_id" ] || [ -z "$running_id" ] || [ "$expected_id" != "$running_id" ]; then
  echo "deployed image mismatch expected=$EXPECTED_IMAGE expected_id=$expected_id running_id=$running_id"
  if [ "$rollback_enabled" -eq 1 ]; then
    docker rm -f openbyte >/dev/null 2>&1 || true
    docker rename "$rollback_name" openbyte >/dev/null 2>&1 || { echo "rollback rename failed"; exit 1; }
    docker start openbyte >/dev/null 2>&1 || { echo "rollback start failed"; exit 1; }
  fi
  docker logout ghcr.io || true
  exit 1
fi
i=1
while [ "$i" -le 20 ]; do
  state=$(docker inspect -f '{{.State.Status}}' openbyte 2>/dev/null || true)
  if [ "$state" != "running" ]; then
    echo "container not running: state=$state"
    i=$((i + 1))
    sleep 3
    continue
  fi
  status=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' openbyte 2>/dev/null || true)
  if [ "$status" = "healthy" ]; then
    if [ "$rollback_enabled" -eq 1 ]; then
      docker rm -f "$rollback_name" >/dev/null 2>&1 || true
    fi
    docker logout ghcr.io || true
    exit 0
  fi
  i=$((i + 1))
  sleep 3
done
docker inspect openbyte --format '{{json .State}}' || true
if [ "$rollback_enabled" -eq 1 ]; then
  docker rm -f openbyte >/dev/null 2>&1 || true
  docker rename "$rollback_name" openbyte >/dev/null 2>&1 || { echo "rollback rename failed"; exit 1; }
  docker start openbyte >/dev/null 2>&1 || { echo "rollback start failed"; exit 1; }
fi
docker logout ghcr.io || true
exit 1
EOF
