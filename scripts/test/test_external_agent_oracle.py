import os
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
from external_agent_oracle import changed_paths, verify_copy, workspace_snapshot


class ExternalAgentOracleTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.expected = "OffGrid test — café 日本語 12345".encode("utf-8")
        (self.root / "source.txt").write_bytes(self.expected)
        self.before = workspace_snapshot(self.root)

    def test_exact_copy_preserves_source(self):
        (self.root / "copy.txt").write_bytes(self.expected)
        self.assertTrue(verify_copy(self.root, self.expected, self.before)["passed"])

    def test_mutating_both_files_is_not_success(self):
        for name in ("source.txt", "copy.txt"):
            (self.root / name).write_bytes(b"same but incorrect")
        result = verify_copy(self.root, self.expected, self.before)
        self.assertFalse(result["passed"])
        self.assertFalse(result["source_preserved"])

    def test_partial_prefixed_content_and_missing_file_fail(self):
        self.assertFalse(verify_copy(self.root, self.expected, self.before)["passed"])
        (self.root / "copy.txt").write_text("1|OffGrid test — café ", encoding="utf-8")
        self.assertFalse(verify_copy(self.root, self.expected, self.before)["passed"])

    def test_other_changes_fail_even_when_copy_matches(self):
        (self.root / "copy.txt").write_bytes(self.expected)
        (self.root / "SOUL.md").write_text("hi")
        result = verify_copy(self.root, self.expected, self.before)
        self.assertFalse(result["passed"])
        self.assertEqual(result["unexpected_changes"], ["SOUL.md"])

    def test_greeting_detects_creation_modification_and_removal(self):
        self.assertEqual(changed_paths(self.before, workspace_snapshot(self.root)), [])
        (self.root / "source.txt").unlink()
        (self.root / "SOUL.md").write_text("hi")
        self.assertEqual(changed_paths(self.before, workspace_snapshot(self.root)), ["SOUL.md", "source.txt"])

    def test_symlink_is_not_independent_copy(self):
        try:
            os.symlink(self.root / "source.txt", self.root / "copy.txt")
        except (OSError, NotImplementedError):
            self.skipTest("Creating symbolic links is unavailable")
        result = verify_copy(self.root, self.expected, self.before)
        self.assertFalse(result["destination_exact"])
        self.assertFalse(result["passed"])


if __name__ == "__main__":
    unittest.main()
