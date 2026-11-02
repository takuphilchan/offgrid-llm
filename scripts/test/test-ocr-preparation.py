"""Build asset admission tests; no downloads or system changes."""
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("prepare_ocr", ROOT / "scripts/build/prepare-ocr-runtime.py")
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


class OCRPreparation(unittest.TestCase):
    def test_invalid_pair_leaves_destination_intact(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source, target = root / "source", root / "target"
            source.mkdir()
            target.mkdir()
            (source / "ocr.wasm").write_bytes(b"forged")
            (source / "NOTICE.txt").write_bytes(b"forged")
            (target / "ocr.wasm").write_bytes(b"existing")
            with self.assertRaisesRegex(ValueError, "digest mismatch"):
                prepare.stage(source, target)
            self.assertEqual((target / "ocr.wasm").read_bytes(), b"existing")
            self.assertEqual(list(target.iterdir()), [target / "ocr.wasm"])

    def test_verified_offline_assets_need_no_network(self):
        source = ROOT / "runtimes/ocr/worker/assets"
        if not (source / "ocr.wasm").is_file():
            self.skipTest("requires prepared pinned engine")
        with tempfile.TemporaryDirectory() as directory, patch.object(prepare.urllib.request, "urlopen", side_effect=AssertionError("network forbidden")):
            target = Path(directory)
            prepare.stage(source, target)
            for name in ("ocr.wasm", "NOTICE.txt"):
                self.assertEqual((target / name).read_bytes(), (source / name).read_bytes())
            self.assertEqual(len(list(target.iterdir())), 2)

    def test_corrupt_cached_download_is_not_replaced_or_retried(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(prepare.urllib.request, "urlopen", side_effect=AssertionError("network forbidden")):
            path = Path(directory) / "archive"
            path.write_bytes(b"corrupt")
            with self.assertRaisesRegex(ValueError, "digest mismatch"):
                prepare.acquire(path, {"archive_sha256": "0" * 64}, 100)
            self.assertEqual(path.read_bytes(), b"corrupt")


if __name__ == "__main__":
    unittest.main()
