#!/usr/bin/env bash
# Pull image DEPLOY_TAG on remote host and recreate openbyte container. Run from repo root.
# Required env: SSH_HOST, SSH_HOST_FINGERPRINT, SSH_USER, SSH_PORT, REMOTE_DIR, SSH_KEY,
# GHCR_USERNAME, GHCR_TOKEN, DEPLOY_TAG, GITHUB_REPOSITORY_OWNER
# Optional env: SERVER_NAME (overrides remote .env for this deploy)
set -euo pipefail
[[ -n "${REMOTE_DIR:-}" ]] || { echo "REMOTE_DIR not set"; exit 1; }
[[ -n "${DEPLOY_TAG:-}" ]] || { echo "DEPLOY_TAG not set"; exit 1; }
[[ -n "${GITHUB_REPOSITORY_OWNER:-}" ]] || { echo "GITHUB_REPOSITORY_OWNER not set"; exit 1; }
REMOTE_SCRIPT="scripts/deploy/deploy_host.sh"
[[ -f "$REMOTE_SCRIPT" ]] || { echo "Missing $REMOTE_SCRIPT"; exit 1; }

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
SERVER_NAME_Q="$(printf '%q' "${SERVER_NAME:-}")"

ssh -i "$SSH_TMP_DIR/id_rsa" \
  -o UserKnownHostsFile="$SSH_TMP_DIR/known_hosts" \
  -o IdentitiesOnly=yes \
  -p "${SSH_PORT:-22}" \
  "${SSH_USER}@${SSH_HOST}" \
  "REMOTE_DIR=$REMOTE_DIR_Q GHCR_USERNAME=$GHCR_USERNAME_Q GHCR_TOKEN=$GHCR_TOKEN_Q OWNER_LC=$OWNER_LC_Q DEPLOY_TAG=$DEPLOY_TAG_Q SERVER_NAME=$SERVER_NAME_Q sh -seu" \
  < "$REMOTE_SCRIPT"
