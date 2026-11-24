"""Explicit administrator-only isolated fixture provisioning; never run by tests.

Usage: sudo python3 provision-linux-fixture.py SERVICE_UID BUILT_FIXTURE_BINARY
Prints a newly created /opt fixture record. Contains no actual model weights.
No existing installation, ownership, host policy or model store is modified.
"""
import hashlib
import json
import os
from pathlib import Path
import pwd
import shutil
import sys
import tempfile

if os.geteuid() != 0 or len(sys.argv) != 3:
    raise SystemExit("requires explicit administrator invocation, service UID and fixture binary")
uid = int(sys.argv[1])
if uid <= 0:
    raise SystemExit("service UID must be non-root")
account = pwd.getpwuid(uid)
binary = Path(sys.argv[2])
if not binary.is_absolute() or binary.is_symlink() or not binary.is_file() or binary.stat().st_size > 512 * 1024 * 1024:
    raise SystemExit("provide a bounded absolute isolated Go fixture executable")
launcher = Path(shutil.which("unshare") or "").resolve(strict=True)
if not launcher.is_file():
    raise SystemExit("util-linux unshare is required")
root = Path(tempfile.mkdtemp(prefix="offgrid-linux-provider-test.", dir="/opt"))
os.chmod(root, 0o755)
runtime, models, state = root / "runtime", root / "models", root / "state"
for directory in (runtime, models / "blobs", models / "manifests/registry.ollama.ai/library/fixture", state):
    directory.mkdir(parents=True, mode=0o755, exist_ok=True)
os.chmod(state, 0o700)
os.chown(state, uid, account.pw_gid)
target = runtime / "fixture"
shutil.copyfile(binary, target)
os.chmod(target, 0o555)

def digest(data):
    return "sha256:" + hashlib.sha256(data).hexdigest()

config, weights = b'{"model_format":"gguf"}', b"isolated protocol fixture, not model weights"
for data in (config, weights):
    output = models / "blobs" / digest(data).replace(":", "-", 1)
    output.write_bytes(data)
    os.chmod(output, 0o444)
manifest = json.dumps({"schemaVersion": 2, "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
    "config": {"mediaType": "application/vnd.docker.container.image.v1+json", "digest": digest(config), "size": len(config)},
    "layers": [{"mediaType": "application/vnd.ollama.image.model", "digest": digest(weights), "size": len(weights)}]}, separators=(",", ":")).encode()
manifest_path = models / "manifests/registry.ollama.ai/library/fixture/tag"
manifest_path.write_bytes(manifest)
os.chmod(manifest_path, 0o444)
record = {"model_digest": digest(manifest), "managed_ollama": {
    "runtime_directory": str(runtime), "executable": "fixture", "runtime_files": {"fixture": digest(target.read_bytes())},
    "model_directory": str(models), "state_directory": str(state), "trusted_single_user_host": True,
    "allow_unauthenticated_loopback": True, "namespace_launcher": str(launcher),
    "namespace_launcher_sha256": digest(launcher.read_bytes())}}
output = root / "fixture.json"
output.write_text(json.dumps(record, indent=2) + "\n", encoding="utf-8")
os.chmod(output, 0o444)
print(output)
