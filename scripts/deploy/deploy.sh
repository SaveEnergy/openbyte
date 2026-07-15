#!/usr/bin/env bash
# Verify the SSH host, sync the deployment bundle, and deploy over one SSH connection.
set -euo pipefail

required=(
  SSH_HOST SSH_HOST_FINGERPRINT SSH_USER REMOTE_DIR SSH_KEY
  GHCR_USERNAME GHCR_TOKEN DEPLOY_TAG GITHUB_REPOSITORY_OWNER
)
for name in "${required[@]}"; do
  [[ -n "${!name:-}" ]] || { echo "$name not set"; exit 1; }
done

if [[ "$DEPLOY_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  DEPLOY_TAG="${DEPLOY_TAG#v}"
fi

files=(
  docker/docker-compose.yaml
  docker/docker-compose.traefik.yaml
  docker/traefik-openbyte.yaml
  scripts/deploy/deploy_host.sh
)
for file in "${files[@]}"; do
  [[ -f "$file" ]] || { echo "Missing $file"; exit 1; }
done

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
key_file="$tmp_dir/id_rsa"
known_hosts="$tmp_dir/known_hosts"
payload="$tmp_dir/payload"

printf '%s\n' "$SSH_KEY" > "$key_file"
chmod 600 "$key_file"
timeout 15 ssh-keyscan -t ed25519 -p "${SSH_PORT:-22}" -H "$SSH_HOST" > "$known_hosts"
[[ -s "$known_hosts" ]] || { echo "ssh-keyscan returned no ed25519 host keys"; exit 1; }

scanned_fingerprint="$(ssh-keygen -lf "$known_hosts" -E sha256 | awk 'NR==1 {print $2}')"
[[ "$scanned_fingerprint" == "$SSH_HOST_FINGERPRINT" ]] || {
  echo "ssh host fingerprint mismatch"
  echo "expected=$SSH_HOST_FINGERPRINT"
  echo "got=${scanned_fingerprint:-missing}"
  exit 1
}

mkdir -p "$payload"
for file in "${files[@]}"; do
  mkdir -p "$payload/$(dirname "$file")"
  cp "$file" "$payload/$file"
done
(
  cd "$payload"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${files[@]}" > .openbyte-deploy-checksums
  else
    shasum -a 256 "${files[@]}" > .openbyte-deploy-checksums
  fi
)

owner_lc="$(printf '%s' "$GITHUB_REPOSITORY_OWNER" | tr '[:upper:]' '[:lower:]')"
remote_dir_q="$(printf '%q' "$REMOTE_DIR")"
ghcr_username_q="$(printf '%q' "$GHCR_USERNAME")"
ghcr_token_q="$(printf '%q' "$GHCR_TOKEN")"
owner_lc_q="$(printf '%q' "$owner_lc")"
deploy_tag_q="$(printf '%q' "$DEPLOY_TAG")"
server_name_q="$(printf '%q' "${SERVER_NAME:-}")"

remote_command="set -eu
mkdir -p $remote_dir_q
if test -d $remote_dir_q/docker/traefik-openbyte.yaml; then rmdir $remote_dir_q/docker/traefik-openbyte.yaml; fi
cd $remote_dir_q
tar -xf -
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum -c .openbyte-deploy-checksums
else
  shasum -a 256 -c .openbyte-deploy-checksums
fi
rm -f .openbyte-deploy-checksums
REMOTE_DIR=$remote_dir_q GHCR_USERNAME=$ghcr_username_q GHCR_TOKEN=$ghcr_token_q OWNER_LC=$owner_lc_q DEPLOY_TAG=$deploy_tag_q SERVER_NAME=$server_name_q TRAEFIK_NETWORK_EXTERNAL=true TRAEFIK_NETWORK_DRIVER= sh scripts/deploy/deploy_host.sh"

tar -C "$payload" -cf - . | ssh \
  -i "$key_file" \
  -o UserKnownHostsFile="$known_hosts" \
  -o IdentitiesOnly=yes \
  -p "${SSH_PORT:-22}" \
  "${SSH_USER}@${SSH_HOST}" \
  "$remote_command"
