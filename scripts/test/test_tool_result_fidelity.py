import json
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
from tool_result_fidelity import cases, outcome, SOURCE, TARGET


def call(name, args, finish="tool_calls"):
    return {"choices": [{"finish_reason": finish, "message": {"tool_calls": [{"id": "fixture", "type": "function", "function": {"name": name, "arguments": json.dumps(args)}}]}}]}


class ResultFidelityTests(unittest.TestCase):
    def test_exact_bytes_and_genuine_prefixes(self):
        for case in cases():
            if case["kind"] != "write":
                continue
            self.assertTrue(outcome(case, call("write_file", {"path": TARGET, "content": case["expected"]})))
            self.assertFalse(outcome(case, call("write_file", {"path": TARGET, "content": "1|" + case["expected"]})))
            self.assertFalse(outcome(case, call("write_file", {"path": SOURCE, "content": case["expected"]})))
            self.assertFalse(outcome(case, call("write_file", {"path": TARGET, "content": case["expected"]}, "length")))

    def test_truncation_never_qualifies_partial_write(self):
        case = next(c for c in cases() if c["kind"] == "continue")
        self.assertTrue(outcome(case, call("read_file", {"path": SOURCE, "offset": 2})))
        self.assertFalse(outcome(case, call("write_file", {"path": TARGET, "content": "first visible line"})))

    def test_receipt_cannot_override_mismatch(self):
        case = next(c for c in cases() if c["id"] == "mismatch_receipt")
        response = {"choices": [{"finish_reason": "stop", "message": {"content": "MATCH"}}]}
        self.assertFalse(outcome(case, response))
        response["choices"][0]["message"]["content"] = "MISMATCH"
        self.assertTrue(outcome(case, response))

    def test_greeting_cannot_write(self):
        self.assertFalse(outcome(cases()[0], call("write_file", {"path": TARGET, "content": "Hello!"})))


if __name__ == "__main__":
    unittest.main()
