#!/bin/sh
# Runs on the deployment host after scripts/deploy/deploy.sh verifies and syncs it.
set -eu

[ -n "${REMOTE_DIR:-}" ] || { echo "REMOTE_DIR not set"; exit 1; }
test -f "$REMOTE_DIR/.env" || { echo "Missing .env at $REMOTE_DIR/.env"; exit 1; }
if ! docker compose up --help 2>/dev/null | grep -q -- '--wait-timeout'; then
  echo "Docker Compose with up --wait-timeout support is required"
  exit 1
fi

traefik_host_rule=$(sed -n 's/^TRAEFIK_HOST_RULE=//p' "$REMOTE_DIR/.env" | tail -n 1)
if [ -n "$traefik_host_rule" ]; then
  case "$traefik_host_rule" in
    \"*\") traefik_host_rule=${traefik_host_rule#\"}; traefik_host_rule=${traefik_host_rule%\"} ;;
    \'*\') traefik_host_rule=${traefik_host_rule#\'}; traefik_host_rule=${traefik_host_rule%\'} ;;
  esac
  TRAEFIK_HOST_RULE=$(printf '%s\n' "$traefik_host_rule" | sed 's/\\`/`/g')
  export TRAEFIK_HOST_RULE
fi

if [ -n "${SERVER_NAME:-}" ]; then
  export SERVER_NAME
else
  unset SERVER_NAME
fi

rollback_enabled=0
rollback_image_tag=
rollback_image_ref=

cleanup() {
  docker logout ghcr.io >/dev/null 2>&1 || true
  if [ -n "$rollback_image_ref" ]; then
    docker image rm "$rollback_image_ref" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

run_compose() {
  image_tag=$1
  shift
  GHCR_OWNER="$OWNER_LC" IMAGE_TAG="$image_tag" docker compose \
    --env-file "$REMOTE_DIR/.env" \
    -f docker/docker-compose.yaml \
    -f docker/docker-compose.traefik.yaml \
    "$@"
}

restore_openbyte() {
  [ "$rollback_enabled" -eq 1 ] || return 0

  echo "restoring previous openbyte image"
  if ! run_compose "$rollback_image_tag" up --wait --wait-timeout 60 --no-deps --force-recreate openbyte; then
    echo "rollback compose up failed"
    docker inspect openbyte --format '{{json .State}}' || true
    return 1
  fi

  restored_id=$(docker inspect -f '{{.Image}}' openbyte 2>/dev/null || true)
  if [ "$restored_id" != "$previous_id" ]; then
    echo "rollback image mismatch expected_id=$previous_id running_id=$restored_id"
    docker inspect openbyte --format '{{json .State}}' || true
    return 1
  fi

  echo "previous openbyte image restored"
}

echo "$GHCR_TOKEN" | docker login ghcr.io -u "$GHCR_USERNAME" --password-stdin
cd "$REMOTE_DIR"
docker network inspect traefik >/dev/null 2>&1 || docker network create traefik >/dev/null 2>&1 || { echo "failed to create traefik network"; exit 1; }
docker network inspect traefik >/dev/null 2>&1 || { echo "traefik network missing after create"; exit 1; }
traefik_network_cidrs=$(docker network inspect traefik --format '{{range .IPAM.Config}}{{if .Subnet}}{{printf "%s," .Subnet}}{{end}}{{end}}') || {
  echo "failed to inspect traefik network subnets"
  exit 1
}
traefik_network_cidrs=${traefik_network_cidrs%,}
[ -n "$traefik_network_cidrs" ] || { echo "traefik network has no configured subnet"; exit 1; }

TRUST_PROXY_HEADERS=true
TRUSTED_PROXY_CIDRS=$traefik_network_cidrs
export TRUST_PROXY_HEADERS TRUSTED_PROXY_CIDRS

run_compose "$DEPLOY_TAG" pull || { echo "docker compose pull failed"; exit 1; }

expected_image="ghcr.io/${OWNER_LC}/openbyte:${DEPLOY_TAG}"
expected_id=$(docker image inspect "$expected_image" -f '{{.Id}}' 2>/dev/null || true)
[ -n "$expected_id" ] || { echo "expected image missing after pull: $expected_image"; exit 1; }

previous_id=$(docker inspect -f '{{.Image}}' openbyte 2>/dev/null || true)
if [ -n "$previous_id" ]; then
  rollback_image_tag="rollback-${DEPLOY_TAG}"
  rollback_image_ref="ghcr.io/${OWNER_LC}/openbyte:${rollback_image_tag}"
  docker image rm "$rollback_image_ref" >/dev/null 2>&1 || true
  docker image tag "$previous_id" "$rollback_image_ref"
  rollback_enabled=1
fi

# A unique image tag changes openbyte's Compose config. Avoid force-recreating an unchanged
# Traefik container on every application deployment.
if ! run_compose "$DEPLOY_TAG" up --wait --wait-timeout 60; then
  echo "docker compose up failed"
  docker inspect openbyte --format '{{json .State}}' || true
  docker inspect traefik --format '{{json .State}}' || true
  restore_openbyte || echo "rollback failed"
  exit 1
fi

running_id=$(docker inspect -f '{{.Image}}' openbyte 2>/dev/null || true)
if [ -z "$running_id" ] || [ "$expected_id" != "$running_id" ]; then
  echo "deployed image mismatch expected=$expected_image expected_id=$expected_id running_id=$running_id"
  restore_openbyte || echo "rollback failed"
  exit 1
fi
