"""Concurrent synthetic requests must not corrupt the qualification evidence."""
from concurrent.futures import ThreadPoolExecutor
import json
from pathlib import Path
import sys
import tempfile
import unittest
from urllib.request import Request, urlopen

sys.path.insert(0, str(Path(__file__).resolve().parent))
from external_agent_fixture import start_fixture


class FixtureEvidenceTests(unittest.TestCase):
    def test_concurrent_requests_publish_complete_evidence(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            server, _ = start_fixture(root, "fixture-model")
            try:
                def request(index):
                    data = json.dumps({"model": "fixture-model", "messages": [{"role": "user", "content": "hello"}]}).encode()
                    req = Request(f"http://127.0.0.1:{server.server_port}/v1/chat/completions", data=data,
                                  headers={"Content-Type": "application/json"})
                    with urlopen(req, timeout=5) as response:
                        self.assertEqual(response.status, 200)
                        json.load(response)
                    snapshot = json.loads((root / "fixture-events.json").read_text())
                    self.assertGreaterEqual(len(snapshot), 1)
                with ThreadPoolExecutor(max_workers=6) as pool:
                    list(pool.map(request, range(24)))
                self.assertEqual(len(json.loads((root / "fixture-events.json").read_text())), 24)
            finally:
                server.shutdown()
                server.server_close()


if __name__ == "__main__": unittest.main()
