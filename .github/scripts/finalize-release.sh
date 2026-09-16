#!/usr/bin/env bash
set -euo pipefail

version="${1:?release tag required}"
repo="${2:?GitHub repository required}"
if [[ ! "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "::error::Expected a vX.Y.Z release tag, got ${version}"
  exit 1
fi

package_version="${version#v}"
# Keep this list in sync with the build-bundles matrix and Electron targets in
# release-unified.yml. Never publish a checksum file for a partial release.
expected=(
  "offgrid-${version}-linux-amd64-cpu-avx2.tar.gz"
  "offgrid-${version}-linux-amd64-cpu-avx512.tar.gz"
  "offgrid-${version}-linux-amd64-vulkan-avx2.tar.gz"
  "offgrid-${version}-linux-amd64-vulkan-avx512.tar.gz"
  "offgrid-${version}-linux-arm64-cpu-neon.tar.gz"
  "offgrid-${version}-darwin-amd64-cpu-avx2.tar.gz"
  "offgrid-${version}-darwin-arm64-metal-apple-silicon.tar.gz"
  "offgrid-${version}-windows-amd64-cpu-avx2.zip"
  "OffGrid.LLM.Desktop-${package_version}-amd64.deb"
  "OffGrid.LLM.Desktop-${package_version}-x86_64.AppImage"
  "OffGrid.LLM.Desktop-${package_version}-arm64.zip"
  "OffGrid.LLM.Desktop-${package_version}-x64.zip"
  "OffGrid.LLM.Desktop-${package_version}-Portable.exe"
  "OffGrid.LLM.Desktop-Setup-${package_version}.exe"
)

gh release view "${version}" --repo "${repo}" >/dev/null
max_attempts="${OFFGRID_FINALIZE_ATTEMPTS:-18}"
retry_seconds="${OFFGRID_FINALIZE_RETRY_SECONDS:-10}"
if [[ ! "${max_attempts}" =~ ^[1-9][0-9]*$ || ! "${retry_seconds}" =~ ^[0-9]+$ ]]; then
  echo "::error::Invalid finalizer retry configuration"
  exit 1
fi

declare -A asset_states=()
declare -A asset_sizes=()
declare -A asset_digests=()
ready=false
problems=()
for ((attempt = 1; attempt <= max_attempts; attempt++)); do
  asset_states=()
  asset_sizes=()
  asset_digests=()
  asset_rows="$(gh release view "${version}" --repo "${repo}" --json assets \
    --jq '.assets[] | [.name, .state, .size, (.digest // "")] | @tsv')"
  while IFS=$'\t' read -r name state size digest; do
    [[ -n "${name}" ]] || continue
    asset_states["${name}"]="${state}"
    asset_sizes["${name}"]="${size}"
    asset_digests["${name}"]="${digest#sha256:}"
  done <<< "${asset_rows}"

  problems=()
  for name in "${expected[@]}"; do
    state="${asset_states[${name}]:-missing}"
    size="${asset_sizes[${name}]:-0}"
    digest="${asset_digests[${name}]:-}"
    if [[ "${state}" == "missing" ]]; then
      problems+=("Missing required release asset: ${name}")
    elif [[ "${state}" != "uploaded" || ! "${size}" =~ ^[1-9][0-9]*$ || ! "${digest}" =~ ^[0-9a-f]{64}$ ]]; then
      problems+=("Invalid release asset ${name}: state=${state}, size=${size}, digest=${digest:-pending}")
    fi
  done

  if ((${#problems[@]} == 0)); then
    ready=true
    break
  fi
  if ((attempt < max_attempts)); then
    echo "Release assets are not fully indexed yet (attempt ${attempt}/${max_attempts}); retrying in ${retry_seconds}s"
    printf '  - %s\n' "${problems[@]}"
    sleep "${retry_seconds}"
  fi
done

if [[ "${ready}" != true ]]; then
  printf '::error::%s\n' "${problems[@]}"
  exit 1
fi

checksum_file="checksums-${version}.sha256"
: > "${checksum_file}"
for name in "${expected[@]}"; do
  printf '%s  %s\n' "${asset_digests[${name}]}" "${name}" >> "${checksum_file}"
done
sort -k2 -o "${checksum_file}" "${checksum_file}"
echo "Verified ${#expected[@]} required release assets with SHA-256 digests"

notes_path="docs/releases/release-notes-${version}.md"
if [[ ! -s "${notes_path}" ]]; then
  echo "::error::Missing release notes: ${notes_path}"
  exit 1
fi

gh release upload "${version}" "${checksum_file}" --repo "${repo}" --clobber
gh release edit "${version}" --repo "${repo}" --notes-file "${notes_path}" --draft=false
published_assets="$(gh release view "${version}" --repo "${repo}" \
  --json assets --jq '.assets[].name')"
if ! grep -Fxq "${checksum_file}" <<< "${published_assets}"; then
  echo "::error::Checksum file was not attached to ${version}"
  exit 1
fi
echo "Published ${version} with ${checksum_file}"
