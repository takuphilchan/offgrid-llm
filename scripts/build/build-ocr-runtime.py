"""Build the pinned OCR WASM module from already acquired archives (Linux).

No downloads or system installation. The output still needs worker/corpus and
redistribution qualification before packaging; compilation alone is not a gate.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile

REPO = Path(__file__).resolve().parents[2]
LOCK = json.loads((REPO / "runtimes/ocr/source.lock.json").read_text())
ENGINE_LOCK = json.loads((REPO / "runtimes/ocr/engine.lock.json").read_text())


def verify_outputs(wasm, notice, engine_lock=ENGINE_LOCK):
    # Validate both before replacing either staged asset. Updating source pins
    # alone cannot silently publish a different engine under the old revision.
    if hashlib.sha256(wasm).hexdigest() != engine_lock["wasm_sha256"]:
        raise ValueError("OCR engine differs from the reviewed engine lock")
    if hashlib.sha256(notice).hexdigest() != engine_lock["notices_sha256"]:
        raise ValueError("OCR notices differ from the reviewed engine lock")


def unpack(archive, expected, destination):
    digest = hashlib.sha256()
    with archive.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    if digest.hexdigest() != expected:
        raise ValueError(f"Archive digest mismatch: {archive.name}")
    marker = destination / ".verified-archive"
    if marker.is_file() and marker.read_text() == expected:
        return
    if destination.exists() and any(destination.iterdir()):
        raise ValueError("Incomplete extraction; select a fresh build directory")
    destination.mkdir(parents=True, exist_ok=True)
    with tarfile.open(archive) as source:
        members = source.getmembers()
        if len(members) > 50000 or sum(m.size for m in members) > 2 * 1024**3:
            raise ValueError("Build archive exceeds limits")
        if any(m.isdev() or m.isfifo() for m in members):
            raise ValueError("Special file in build archive")
        source.extractall(destination, filter="data")
    marker.write_text(expected)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--archives", type=Path, required=True)
    parser.add_argument("--toolchain", type=Path, required=True)
    parser.add_argument("--work", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--jobs", type=int, default=2, choices=range(1, 9))
    args = parser.parse_args()
    if not sys.platform.startswith("linux"):
        parser.error("The pinned compiler archive targets Linux; use the isolated Linux build environment")
    work = args.work.resolve()
    marker = work / ".offgrid-ocr-build"
    identity = json.dumps(LOCK, sort_keys=True)
    if work.exists() and any(work.iterdir()) and (not marker.is_file() or marker.read_text() != identity):
        raise ValueError("Build directory is not owned by this source revision")
    work.mkdir(parents=True, exist_ok=True)
    marker.write_text(identity)
    unpack(args.toolchain, LOCK["toolchain"]["archive_sha256"], work / "sdk")
    for name in ("leptonica", "tesseract"):
        unpack(args.archives / f"{name}.tar.gz", LOCK[name]["archive_sha256"], work / name)
    sdk = work / "sdk/install"
    emscripten = sdk / "emscripten"
    if not (emscripten / "emcc.py").is_file():
        raise ValueError("Unexpected pinned toolchain layout")
    node = shutil.which("node")
    if not node:
        raise ValueError("The build requires Node.js; it is never installed automatically")
    config = work / "emscripten-config.py"
    config.write_text(f"LLVM_ROOT = {str(sdk / 'bin')!r}\nBINARYEN_ROOT = {str(sdk)!r}\nNODE_JS = {node!r}\n")
    env = dict(os.environ, EM_CONFIG=str(config), EM_CACHE=str(work / "cache"))
    # Keep source/build directory names out of compiled assertions and __FILE__.
    remap = f"-ffile-prefix-map={work}=/offgrid/ocr -ffile-prefix-map={REPO}=/offgrid/src"
    def run(*command):
        subprocess.run([str(c) for c in command], cwd=work, env=env, check=True)
    def configure(name, *flags):
        source = work / name / f"{name}-{LOCK[name]['revision']}"
        run(sys.executable, emscripten / "emcmake.py", "cmake", "-G", "Ninja", "-S", source,
            "-B", work / f"{name}-build", "-DCMAKE_BUILD_TYPE=Release", "-DBUILD_SHARED_LIBS=OFF",
            f"-DCMAKE_INSTALL_PREFIX={work / 'prefix'}", f"-DCMAKE_C_FLAGS=-O2 {remap}", f"-DCMAKE_CXX_FLAGS=-O2 {remap} -DTESSERACT_DISABLE_DEBUG_FONTS", *flags)
    configure("leptonica", "-DBUILD_PROG=OFF", "-DSW_BUILD=OFF", *[f"-DENABLE_{c}=OFF" for c in ("ZLIB","PNG","GIF","JPEG","TIFF","WEBP","OPENJPEG")])
    run("cmake", "--build", work / "leptonica-build", "--parallel", args.jobs)
    run("cmake", "--install", work / "leptonica-build")
    configure("tesseract", f"-DCMAKE_PREFIX_PATH={work / 'prefix'}", "-DBUILD_TRAINING_TOOLS=OFF",
              "-DBUILD_TESTS=OFF", "-DOPENMP_BUILD=OFF", "-DGRAPHICS_DISABLED=ON", "-DDISABLED_LEGACY_ENGINE=ON",
              "-DDISABLE_ARCHIVE=ON", "-DDISABLE_CURL=ON", "-DDISABLE_TIFF=ON", "-DENABLE_NATIVE=OFF", "-DINSTALL_CONFIGS=OFF")
    run("cmake", "--build", work / "tesseract-build", "--target", "libtesseract", "--parallel", args.jobs)
    exports = ["_malloc", "_free", "_offgrid_ocr_version", "_offgrid_ocr_create", "_offgrid_ocr_recognize",
               "_offgrid_ocr_result_size", "_offgrid_ocr_free_result", "_offgrid_ocr_destroy"]
    run(sys.executable, emscripten / "em++.py", "-O2", "-std=c++17", "-DTESSERACT_DISABLE_DEBUG_FONTS",
        *remap.split(),
        f"-I{work / 'prefix/include'}", f"-I{work / 'tesseract' / ('tesseract-' + LOCK['tesseract']['revision']) / 'include'}",
        f"-I{work / 'tesseract-build/include'}", REPO / "runtimes/ocr/bridge.cpp",
        work / "tesseract-build/libtesseract.a", work / "prefix/lib/libleptonica.a",
        "--no-entry", "-sSTANDALONE_WASM=1", "-sALLOW_MEMORY_GROWTH=1", "-sMAXIMUM_MEMORY=536870912",
        "-sSTACK_SIZE=1048576", "-sFILESYSTEM=0", "-sEXPORTED_FUNCTIONS=" + json.dumps(exports), "-o", work / "ocr.wasm")
    notices = [
        ("Tesseract", work / "tesseract" / ("tesseract-" + LOCK["tesseract"]["revision"]) / "LICENSE"),
        ("Leptonica", work / "leptonica" / ("leptonica-" + LOCK["leptonica"]["revision"]) / "leptonica-license.txt"),
        ("Emscripten", emscripten / "LICENSE"),
    ]
    notices += [(str(p.relative_to(emscripten)), p) for p in sorted((emscripten / "system").rglob("*"))
                if p.is_file() and p.name in ("LICENSE", "LICENSE.TXT", "COPYRIGHT")]
    notice = "OffGrid OCR worker — pinned local build\n" + json.dumps(LOCK, sort_keys=True, indent=2) + "\n"
    for label, path in notices:
        notice += f"\n--- {label} ---\n" + path.read_text(encoding="utf-8") + "\n"
    wasm = (work / "ocr.wasm").read_bytes()
    notice_bytes = notice.encode("utf-8")
    verify_outputs(wasm, notice_bytes)
    output = args.output.resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_bytes(wasm)
    (output.parent / "NOTICE.txt").write_bytes(notice_bytes)
    print("Built OCR WASM", hashlib.sha256(output.read_bytes()).hexdigest(), flush=True)


if __name__ == "__main__":
    main()
