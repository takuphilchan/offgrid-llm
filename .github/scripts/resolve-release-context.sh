#!/usr/bin/env bash
set -euo pipefail

event_name="${1:?GitHub event name required}"
ref="${2:?GitHub ref required}"
requested_version="${3:-}"

case "${event_name}:${ref}" in
  workflow_dispatch:*)
    version="${requested_version}"
    ;;
  push:refs/tags/v*)
    version="${ref#refs/tags/}"
    ;;
  push:refs/heads/release-build/v*)
    version="${ref#refs/heads/release-build/}"
    ;;
  *)
    echo "::error::Unsupported release trigger: event=${event_name}, ref=${ref}"
    exit 1
    ;;
esac

if [[ ! "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "::error::Expected a vX.Y.Z release version, got ${version:-<empty>}"
  exit 1
fi

output="${GITHUB_OUTPUT:-/dev/stdout}"
{
  echo "version=${version}"
  # Builds always use the immutable tag. A repair branch supplies only the
  # current workflow definition and must never change packaged source code.
  echo "source_ref=${version}"
} >> "${output}"
