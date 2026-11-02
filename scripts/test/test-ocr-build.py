"""Offline integrity tests for the OCR build publication boundary."""
import hashlib
import importlib.util
from pathlib import Path
import unittest

REPO = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("ocr_build", REPO / "scripts/build/build-ocr-runtime.py")
BUILD = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(BUILD)


class BuildIntegrity(unittest.TestCase):
    def test_engine_and_notices_must_both_match(self):
        wasm, notice = b"reviewed engine", b"reviewed notices"
        lock = {"wasm_sha256": hashlib.sha256(wasm).hexdigest(),
                "notices_sha256": hashlib.sha256(notice).hexdigest()}
        BUILD.verify_outputs(wasm, notice, lock)
        for candidate, licenses in ((wasm + b"changed", notice), (wasm, notice + b"changed")):
            with self.subTest(engine=candidate == wasm), self.assertRaises(ValueError):
                BUILD.verify_outputs(candidate, licenses, lock)

    def test_staged_real_assets_match_reviewed_lock(self):
        assets = REPO / "runtimes/ocr/worker/assets"
        if not (assets / "ocr.wasm").is_file():
            self.skipTest("build the pinned OCR assets before packaging qualification")
        BUILD.verify_outputs((assets / "ocr.wasm").read_bytes(), (assets / "NOTICE.txt").read_bytes())


if __name__ == "__main__":
    unittest.main()
