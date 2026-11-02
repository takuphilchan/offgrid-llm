"""Explicit maintainer OCR preparation; never called by an end-user import.

--assets stages a verified offline build. --build-from-source explicitly acquires
pinned source/compiler archives and runs the isolated Linux source build.
Neither path installs system dependencies or changes a workspace language pack.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
ENGINE = json.loads((ROOT / "runtimes/ocr/engine.lock.json").read_text())
SOURCE = json.loads((ROOT / "runtimes/ocr/source.lock.json").read_text())


def checked(path, expected, limit):
    if path.is_symlink() or not path.is_file() or not 0 < path.stat().st_size <= limit:
        raise ValueError(f"Invalid OCR build asset: {path.name}")
    data = path.read_bytes()
    if hashlib.sha256(data).hexdigest() != expected:
        raise ValueError(f"OCR build asset digest mismatch: {path.name}")
    return data


def publish(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.parent.is_symlink() or path.is_symlink():
        raise ValueError("OCR staging paths must not be symlinks")
    handle, temporary = tempfile.mkstemp(prefix=".ocr-", dir=path.parent)
    try:
        with os.fdopen(handle, "wb") as out:
            out.write(data)
            out.flush()
            os.fsync(out.fileno())
        os.replace(temporary, path)
    finally:
        Path(temporary).unlink(missing_ok=True)


def stage(source, destination):
    # Check the complete pair first. A rejected import leaves existing assets intact.
    wasm = checked(source / "ocr.wasm", ENGINE["wasm_sha256"], 32 << 20)
    notices = checked(source / "NOTICE.txt", ENGINE["notices_sha256"], 2 << 20)
    publish(destination / "ocr.wasm", wasm)
    publish(destination / "NOTICE.txt", notices)


def acquire(path, record, limit):
    if path.exists():
        checked(path, record["archive_sha256"], limit)
        return
    url = record["archive_url"]
    if not url.startswith("https://"):
        raise ValueError("Build source must use HTTPS")
    digest, size = hashlib.sha256(), 0
    path.parent.mkdir(parents=True, exist_ok=True)
    handle, temporary = tempfile.mkstemp(prefix=".download-", dir=path.parent)
    try:
        with os.fdopen(handle, "wb") as out, urllib.request.urlopen(url, timeout=120) as response:
            if not response.url.startswith("https://"):
                raise ValueError("Build download downgraded transport")
            while block := response.read(1 << 20):
                size += len(block)
                if size > limit:
                    raise ValueError("Build archive exceeds size limit")
                digest.update(block)
                out.write(block)
        if digest.hexdigest() != record["archive_sha256"]:
            raise ValueError("Build archive digest mismatch")
        os.replace(temporary, path)
    finally:
        Path(temporary).unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--assets", type=Path)
    mode.add_argument("--build-from-source", type=Path, metavar="CACHE")
    parser.add_argument("--output", type=Path, default=ROOT / "runtimes/ocr/worker/assets")
    args = parser.parse_args()
    source = args.assets
    if args.build_from_source:
        if not sys.platform.startswith("linux"):
            parser.error("Source compilation requires the pinned Linux toolchain; use --assets for a verified offline build")
        cache = args.build_from_source.resolve()
        archives = cache / "archives"
        for name in ("tesseract", "leptonica"):
            acquire(archives / f"{name}.tar.gz", SOURCE[name], 64 << 20)
        compiler = archives / "wasm-binaries.tar.xz"
        acquire(compiler, SOURCE["toolchain"], 512 << 20)
        source = cache / "output"
        subprocess.run([sys.executable, str(ROOT / "scripts/build/build-ocr-runtime.py"),
                        "--archives", str(archives), "--toolchain", str(compiler),
                        "--work", str(cache / "work"), "--output", str(source / "ocr.wasm"),
                        "--jobs", "4"], check=True)
    stage(source.resolve(), args.output.resolve())
    print(f"Prepared pinned OCR engine {ENGINE['revision']}; qualification unchanged.")


if __name__ == "__main__":
    main()
