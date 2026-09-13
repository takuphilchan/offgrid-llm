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
asset_rows="$(gh api "repos/${repo}/releases/tags/${version}" \
  --jq '.assets[] | [.name, .state, .size, .digest] | @tsv')"
declare -A asset_digests=()
while IFS=$'\t' read -r name state size digest; do
  [[ -n "${name}" ]] || continue
  if [[ "${state}" != "uploaded" || ! "${size}" =~ ^[1-9][0-9]*$ || ! "${digest}" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    echo "::error::Invalid release asset ${name}: state=${state}, size=${size}, digest=${digest}"
    exit 1
  fi
  asset_digests["${name}"]="${digest#sha256:}"
done <<< "${asset_rows}"

checksum_file="checksums-${version}.sha256"
: > "${checksum_file}"
for name in "${expected[@]}"; do
  if [[ -z "${asset_digests[${name}]:-}" ]]; then
    echo "::error::Missing required release asset: ${name}"
    exit 1
  fi
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
