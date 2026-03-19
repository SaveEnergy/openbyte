#!/usr/bin/env bash
# Required for deploy jobs: SSH_HOST, SSH_HOST_FINGERPRINT, SSH_USER, REMOTE_DIR,
# GHCR_USERNAME, SSH_KEY, GHCR_TOKEN (set by workflow env).
set -euo pipefail
[[ -n "${SSH_HOST:-}" ]] || { echo "SSH_HOST not set"; exit 1; }
[[ -n "${SSH_HOST_FINGERPRINT:-}" ]] || { echo "SSH_HOST_FINGERPRINT not set"; exit 1; }
[[ -n "${SSH_USER:-}" ]] || { echo "SSH_USER not set"; exit 1; }
[[ -n "${REMOTE_DIR:-}" ]] || { echo "REMOTE_DIR not set"; exit 1; }
[[ -n "${GHCR_USERNAME:-}" ]] || { echo "GHCR_USERNAME not set"; exit 1; }
[[ -n "${SSH_KEY:-}" ]] || { echo "SSH_KEY not set"; exit 1; }
[[ -n "${GHCR_TOKEN:-}" ]] || { echo "GHCR_TOKEN not set"; exit 1; }
