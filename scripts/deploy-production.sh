#!/usr/bin/env bash
set -Eeuo pipefail

readonly REPOSITORY_DIR="/Project/workspaces/dovelora/dujiao-next"
readonly COMPOSE_FILE="/Project/compose.yml"
readonly DEPLOY_ENV="/Project/deployments/dujiao/deploy.env"
readonly IMAGE_REPOSITORY="dovelora/dujiao-next"

if [[ "$#" -ne 1 ]]; then
  echo "Expected exactly one Git commit SHA." >&2
  exit 2
fi

commit_sha="$1"
if [[ ! "${commit_sha}" =~ ^[0-9a-f]{40}$ ]]; then
  echo "Expected one full 40-character Git commit SHA." >&2
  exit 2
fi

short_sha="${commit_sha:0:12}"
candidate_image="${IMAGE_REPOSITORY}:gh-${short_sha}"
build_dir="$(mktemp -d /tmp/dovelora-production.XXXXXX)"

cleanup() {
  rm -rf "${build_dir}"
}
trap cleanup EXIT

read_current_image() {
  awk -F= '$1 == "DUJIAO_IMAGE" { print substr($0, index($0, "=") + 1); exit }' "${DEPLOY_ENV}"
}

write_image_pointer() {
  local image="$1"
  local pointer_dir
  local temporary_pointer
  pointer_dir="$(dirname "${DEPLOY_ENV}")"
  mkdir -p "${pointer_dir}"
  temporary_pointer="$(mktemp "${pointer_dir}/.deploy.env.XXXXXX")"
  printf 'DUJIAO_IMAGE=%s\n' "${image}" > "${temporary_pointer}"
  chmod 640 "${temporary_pointer}"
  mv "${temporary_pointer}" "${DEPLOY_ENV}"
}

recreate_storefront() {
  docker compose \
    --env-file "${DEPLOY_ENV}" \
    -f "${COMPOSE_FILE}" \
    up -d --no-deps --force-recreate dujiao
}

wait_for_local_health() {
  local attempt
  for attempt in $(seq 1 30); do
    if curl --fail --silent --show-error --max-time 5 \
      http://127.0.0.1:18081/api/v1/public/config >/dev/null; then
      return 0
    fi
    sleep 2
  done
  return 1
}

wait_for_public_health() {
  local attempt
  for attempt in $(seq 1 8); do
    if curl --fail --silent --show-error --max-time 10 \
      https://www.dovelora.com/ >/dev/null; then
      return 0
    fi
    sleep 3
  done
  return 1
}

rollback() {
  local previous_image="$1"
  echo "Deployment failed; restoring ${previous_image}." >&2
  write_image_pointer "${previous_image}"
  recreate_storefront
  wait_for_local_health
}

if [[ ! -f "${DEPLOY_ENV}" ]]; then
  echo "Missing deployment pointer: ${DEPLOY_ENV}" >&2
  exit 1
fi

previous_image="$(read_current_image)"
if [[ -z "${previous_image}" ]]; then
  echo "Deployment pointer does not contain DUJIAO_IMAGE." >&2
  exit 1
fi

payment_before="$(docker inspect -f '{{.Id}}|{{.State.StartedAt}}' epusdt)"

git -C "${REPOSITORY_DIR}" fetch --quiet origin "${commit_sha}"
git -C "${REPOSITORY_DIR}" cat-file -e "${commit_sha}^{commit}"
git -C "${REPOSITORY_DIR}" archive "${commit_sha}" | tar -x -C "${build_dir}"

docker build \
  --build-arg "APP_VERSION=gh-${short_sha}" \
  --tag "${candidate_image}" \
  "${build_dir}"

write_image_pointer "${candidate_image}"
if ! recreate_storefront || ! wait_for_local_health || ! wait_for_public_health; then
  rollback "${previous_image}"
  exit 1
fi

payment_after="$(docker inspect -f '{{.Id}}|{{.State.StartedAt}}' epusdt)"
if [[ "${payment_before}" != "${payment_after}" ]]; then
  echo "Payment container identity changed outside the deployment action." >&2
  rollback "${previous_image}"
  exit 1
fi

deployed_image="$(docker inspect -f '{{.Config.Image}}' dujiao)"
if [[ "${deployed_image}" != "${candidate_image}" ]]; then
  echo "Storefront is running ${deployed_image}, expected ${candidate_image}." >&2
  rollback "${previous_image}"
  exit 1
fi

echo "Deployed ${candidate_image} from ${commit_sha}."
echo "Payment container remained ${payment_after}."
