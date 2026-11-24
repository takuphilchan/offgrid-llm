#!/usr/bin/env python3
"""Explicit administrator-only disposable protocol fixture; never auto-run."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import sys
import tempfile

if os.geteuid() != 0 or len(sys.argv) != 3:
    raise SystemExit("usage as administrator: provision-linux-fixture.py NONROOT_UID BUILT_FIXTURE")
uid = int(sys.argv[1])
if uid <= 0:
    raise SystemExit("fixture service must be non-root")
source = Path(sys.argv[2]).resolve(strict=True)
info = source.stat()
if not stat.S_ISREG(info.st_mode) or info.st_size > 64 * 1024 * 1024:
    raise SystemExit("source must be a bounded built fixture executable")
launcher = Path(shutil.which("unshare") or "").resolve(strict=True)
if not launcher.is_file():
    raise SystemExit("explicit util-linux unshare installation is required")
root = Path(tempfile.mkdtemp(prefix="offgrid-onnx-fixture-", dir="/opt"))
runtime, state = root / "runtime", root / "state"
runtime.mkdir(mode=0o755)
state.mkdir(mode=0o700)
os.chown(state, uid, -1)
shutil.copyfile(source, runtime / "fixture")
(runtime / "fixture").chmod(0o555)
(runtime / "worker.py").write_text("Isolated protocol fixture marker; not ONNX or Python execution.\n")
(runtime / "worker.py").chmod(0o444)
config = {
    "schema_version": 1, "runner_revision": "worker-v1",
    "runtime_directory": str(runtime), "python": "fixture", "script": "worker.py",
    "runtime_files": {path.name: "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest() for path in runtime.iterdir()},
    "dependency_versions": {name: "1.0" for name in ("onnxruntime", "onnx", "numpy", "tokenizers")},
    "state_directory": str(state), "namespace_launcher": str(launcher),
    "namespace_launcher_sha256": "sha256:" + hashlib.sha256(launcher.read_bytes()).hexdigest(),
}
config_path = root / "runtime.json"
config_path.write_text(json.dumps(config, indent=2) + "\n")
config_path.chmod(0o444)
runtime.chmod(0o555)
root.chmod(0o755)
print(config_path)
print("Run tests as the named ordinary user; no host sysctl/cgroup/installation was changed.")
