#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ ! "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "::error::Usage: verify-release-containers.sh vX.Y.Z"
  exit 1
fi

attempts="${OFFGRID_CONTAINER_VERIFY_ATTEMPTS:-18}"
retry_seconds="${OFFGRID_CONTAINER_VERIFY_RETRY_SECONDS:-10}"
if [[ ! "${attempts}" =~ ^[1-9][0-9]*$ ]] || [[ ! "${retry_seconds}" =~ ^[0-9]+$ ]]; then
  echo "::error::Invalid container verification retry configuration"
  exit 1
fi

image_name="${OFFGRID_CONTAINER_IMAGE:-takuphilchan/offgrid-llm}"
hub_api="${OFFGRID_DOCKER_HUB_API:-https://hub.docker.com/v2/repositories}"
image_version="${version#v}"

fetch_tag() {
  local tag="$1"
  curl --fail --silent --show-error --max-time 30 --retry 3 --retry-all-errors \
    "${hub_api}/${image_name}/tags/${tag}/"
}

for ((attempt = 1; attempt <= attempts; attempt++)); do
  cpu_json="$(fetch_tag "${image_version}" 2>/dev/null || true)"
  gpu_json="$(fetch_tag "${image_version}-gpu" 2>/dev/null || true)"

  cpu_ready=false
  gpu_ready=false
  if jq -e '.images | any(.[]; .os == "linux" and .architecture == "amd64") and any(.[]; .os == "linux" and .architecture == "arm64")' \
    <<< "${cpu_json}" >/dev/null 2>&1; then
    cpu_ready=true
  fi
  if jq -e '.images | any(.[]; .os == "linux" and .architecture == "amd64")' \
    <<< "${gpu_json}" >/dev/null 2>&1; then
    gpu_ready=true
  fi

  if [[ "${cpu_ready}" == true && "${gpu_ready}" == true ]]; then
    echo "Verified ${image_name}:${image_version} for Linux AMD64 and ARM64"
    echo "Verified ${image_name}:${image_version}-gpu for Linux AMD64"
    exit 0
  fi

  echo "Container registry is not fully indexed (${attempt}/${attempts}): cpu=${cpu_ready}, gpu=${gpu_ready}"
  if ((attempt < attempts)); then
    sleep "${retry_seconds}"
  fi
done

echo "::error::Docker Hub release ${image_version} is incomplete after bounded retries"
exit 1
