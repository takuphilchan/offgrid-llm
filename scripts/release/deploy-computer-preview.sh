#!/usr/bin/env bash
# Local, operator-authorized deployment only; never used for release publishing.
set -euo pipefail
umask 077
image=${1:-offgrid-llm:computer-recovery-gpu-20260920}
# Optional, explicit local runtime override. Never silently lower cache precision.
cache_env=()
case ${2:-} in
  '') ;;
  f16|q8_0|q4_0) cache_env=(-e "OFFGRID_KV_CACHE_TYPE=$2") ;;
  *) printf 'Unsupported KV cache type\n' >&2; exit 2 ;;
esac
test "$(docker inspect -f '{{.Name}}' offgrid)" = /offgrid
test "$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/var/lib/offgrid/data"}}{{.Name}}{{end}}{{end}}' offgrid)" = offgrid-data
test "$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/var/lib/offgrid/models"}}{{.Name}}{{end}}{{end}}' offgrid)" = offgrid-models
docker image inspect "$image" >/dev/null
version=$(docker image inspect -f '{{index .Config.Labels "org.opencontainers.image.version"}}' "$image")
test -n "$version"
phase=untouched
check=
previous=
recover() {
  result=$?
  if [ "$result" -ne 0 ] && [ "$phase" = clone ]; then
    if [ -n "$check" ]; then docker stop "$check" >/dev/null 2>&1 || true; fi
    if [ -n "$previous" ]; then docker rename "$previous" offgrid || true; fi
    docker start offgrid >/dev/null || true
    printf 'Clone validation/deployment failed before live data activation. Original restart attempted. Backup: %s\n' "$backup" >&2
  fi
  if [ "$result" -ne 0 ] && [ "$phase" = live ]; then
    docker stop offgrid >/dev/null 2>&1 || true
    printf 'Live activation failed. Restore matched data before rollback. Backup: %s\n' "$backup" >&2
  fi
}
trap recover EXIT
backup=$(mktemp -d /home/phil/offgrid-computer-backup-XXXXXXXX)
docker inspect offgrid > "$backup/container.json"
docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' offgrid > "$backup/container.env"
printf 'Backup directory: %s\n' "$backup"
docker stop --time 45 offgrid
phase=clone
mkdir "$backup/data"
docker cp offgrid:/var/lib/offgrid/data/. "$backup/data"
tar -C "$backup" -czf "$backup/workspace.tar.gz" data
sha256sum "$backup/workspace.tar.gz" > "$backup/workspace.sha256"
sha256sum -c "$backup/workspace.sha256"
cp -a "$backup/data" "$backup/check-data"
check=offgrid-computer-check-$(date +%s)
docker run -d --name "$check" --init --gpus all --security-opt no-new-privileges=true --cap-drop ALL \
  --env-file "$backup/container.env" "${cache_env[@]}" -e OFFGRID_VERSION="$version" \
  -p 127.0.0.1:11612:11611 \
  -v "$backup/check-data:/var/lib/offgrid/data" \
  -v offgrid-models:/var/lib/offgrid/models:ro "$image" >/dev/null
ready=false
for attempt in $(seq 1 30); do
  if curl --max-time 3 -fsS http://127.0.0.1:11612/health > "$backup/check-health.json"; then ready=true; break; fi
  sleep 1
done
if ! "$ready" || ! curl --max-time 10 -fsS http://127.0.0.1:11612/v1/agents/tasks > "$backup/check-tasks.json"; then
  docker stop "$check" >/dev/null || true
  # Validation touched only the clone; the original workspace is still unchanged.
  docker start offgrid >/dev/null
  phase=untouched
  printf 'Validation failed. Original service restarted; diagnostics retained in %s\n' "$backup" >&2
  exit 1
fi
curl --max-time 10 -fsS http://127.0.0.1:11612/api/v2/computer/capabilities > "$backup/check-capabilities.json"
curl --max-time 10 -fsS http://127.0.0.1:11612/ui/ > "$backup/check-ui.html"
docker stop "$check" >/dev/null
# Keep the stopped test container and clone as evidence; no volumes are deleted.
previous=offgrid-before-computer-$(date +%s)
docker rename offgrid "$previous"
phase=live
docker run -d --name offgrid --init --restart unless-stopped --gpus all \
  --security-opt no-new-privileges=true --cap-drop ALL \
  --env-file "$backup/container.env" "${cache_env[@]}" -e OFFGRID_VERSION="$version" \
  -p 127.0.0.1:11611:11611 \
  -v offgrid-data:/var/lib/offgrid/data \
  -v offgrid-models:/var/lib/offgrid/models "$image" >/dev/null
for attempt in $(seq 1 30); do
  if curl --max-time 3 -fsS http://127.0.0.1:11611/health > "$backup/live-health.json"; then
    curl --max-time 10 -fsS http://127.0.0.1:11611/v1/agents/tasks > "$backup/live-tasks.json"
    curl --max-time 10 -fsS http://127.0.0.1:11611/api/v2/computer/capabilities > "$backup/live-capabilities.json"
    phase=done
    printf 'Ready. Previous container: %s\nBackup: %s\n' "$previous" "$backup"
    exit 0
  fi
  sleep 1
done
docker stop offgrid >/dev/null || true
printf 'New service failed health verification. Do not start the old binary on migrated data; restore the matched backup in %s first.\n' "$backup" >&2
exit 1
