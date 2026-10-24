"""Build-patch admission checks; no models, downloads, or installed runtime."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


@unittest.skipUnless(os.name == "posix", "Build patch uses the POSIX build toolchain")
class PatchAdmissionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.script = Path(__file__).resolve().parents[2] / "runtimes/llama-cpp/apply-patches.sh"
        (self.root / "tests").mkdir()
        # A command fixture exercises fail-closed admission before git apply.
        stub = self.root / "git"
        stub.write_text('#!/bin/sh\ncase "$3" in\n'
                        'rev-parse) printf "%s\\n" "$TEST_REVISION";;\n'
                        'diff) exit "$TEST_DIRTY";;\n'
                        '*) echo unexpected-patch-attempt >&2; exit 99;;\nesac\n')
        stub.chmod(0o700)

    def run_patch(self, revision, dirty=0):
        return subprocess.run(["sh", str(self.script), str(self.root)],
                              env={**os.environ, "PATH": str(self.root) + os.pathsep + os.environ["PATH"],
                                   "TEST_REVISION": revision, "TEST_DIRTY": str(dirty)},
                              capture_output=True, text=True, timeout=5)

    def test_wrong_revision_never_attempts_patch(self):
        result = self.run_patch("unexpected-revision")
        self.assertEqual(result.returncode, 1)
        self.assertIn("requires", result.stderr)
        self.assertNotIn("unexpected-patch-attempt", result.stderr)

    def test_dirty_preimage_never_attempts_patch(self):
        result = self.run_patch("b95502ba9aa0eb73a2f4fc8878d7fbe6a847a0b9", dirty=1)
        self.assertEqual(result.returncode, 1)
        self.assertNotIn("unexpected-patch-attempt", result.stderr)
        self.assertFalse((self.root / "tests/offgrid-native-sequential.h").exists())

    def test_existing_test_header_is_not_overwritten(self):
        target = self.root / "tests/offgrid-native-sequential.h"
        target.write_text("preserve")
        result = self.run_patch("b95502ba9aa0eb73a2f4fc8878d7fbe6a847a0b9")
        self.assertEqual(result.returncode, 1)
        self.assertNotIn("unexpected-patch-attempt", result.stderr)
        self.assertEqual(target.read_text(), "preserve")


if __name__ == "__main__":
    unittest.main()
