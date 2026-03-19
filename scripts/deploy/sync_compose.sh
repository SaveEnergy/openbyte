#!/usr/bin/env bash
# Sync docker compose files to remote host; verify checksums. Run from repo root.
# Uses: SSH_HOST, SSH_HOST_FINGERPRINT, SSH_USER, SSH_PORT, REMOTE_DIR, SSH_KEY
set -euo pipefail
[[ -n "${REMOTE_DIR:-}" ]] || { echo "REMOTE_DIR not set"; exit 1; }
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
ssh -i "$SSH_TMP_DIR/id_rsa" -o UserKnownHostsFile="$SSH_TMP_DIR/known_hosts" -p "${SSH_PORT:-22}" "${SSH_USER}@${SSH_HOST}" "mkdir -p \"${REMOTE_DIR}/docker\""
scp -i "$SSH_TMP_DIR/id_rsa" -o UserKnownHostsFile="$SSH_TMP_DIR/known_hosts" -P "${SSH_PORT:-22}" \
  docker/docker-compose.ghcr.yaml \
  docker/docker-compose.ghcr.traefik.yaml \
  "${SSH_USER}@${SSH_HOST}:${REMOTE_DIR}/docker/"
ssh -i "$SSH_TMP_DIR/id_rsa" -o UserKnownHostsFile="$SSH_TMP_DIR/known_hosts" -p "${SSH_PORT:-22}" "${SSH_USER}@${SSH_HOST}" \
  "test -f \"${REMOTE_DIR}/docker/docker-compose.ghcr.yaml\" && test -f \"${REMOTE_DIR}/docker/docker-compose.ghcr.traefik.yaml\""
local_main_sha="$(sha256sum docker/docker-compose.ghcr.yaml | cut -d ' ' -f1)"
local_traefik_sha="$(sha256sum docker/docker-compose.ghcr.traefik.yaml | cut -d ' ' -f1)"
remote_main_sha="$(ssh -i "$SSH_TMP_DIR/id_rsa" -o UserKnownHostsFile="$SSH_TMP_DIR/known_hosts" -p "${SSH_PORT:-22}" "${SSH_USER}@${SSH_HOST}" "if command -v sha256sum >/dev/null 2>&1; then sha256sum \"${REMOTE_DIR}/docker/docker-compose.ghcr.yaml\" | cut -d ' ' -f1; else shasum -a 256 \"${REMOTE_DIR}/docker/docker-compose.ghcr.yaml\" | cut -d ' ' -f1; fi")"
remote_traefik_sha="$(ssh -i "$SSH_TMP_DIR/id_rsa" -o UserKnownHostsFile="$SSH_TMP_DIR/known_hosts" -p "${SSH_PORT:-22}" "${SSH_USER}@${SSH_HOST}" "if command -v sha256sum >/dev/null 2>&1; then sha256sum \"${REMOTE_DIR}/docker/docker-compose.ghcr.traefik.yaml\" | cut -d ' ' -f1; else shasum -a 256 \"${REMOTE_DIR}/docker/docker-compose.ghcr.traefik.yaml\" | cut -d ' ' -f1; fi")"
if [[ "$local_main_sha" != "$remote_main_sha" ]]; then
  echo "compose checksum mismatch for docker-compose.ghcr.yaml"
  exit 1
fi
if [[ "$local_traefik_sha" != "$remote_traefik_sha" ]]; then
  echo "compose checksum mismatch for docker-compose.ghcr.traefik.yaml"
  exit 1
fi
