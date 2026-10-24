"""Offline matrix-harness checks; never load a model or installed agent."""
import importlib.util
import io
import json
from pathlib import Path
import unittest
import urllib.error
import urllib.request

spec = importlib.util.spec_from_file_location("matrix", Path(__file__).with_name("external_model_matrix.py"))
matrix = importlib.util.module_from_spec(spec)
spec.loader.exec_module(matrix)


class Opener:
    def __init__(self, data):
        self.data = data

    def open(self, request, timeout):
        return io.BytesIO(self.data)


class MatrixTests(unittest.TestCase):
    def test_origins_and_redirects(self):
        self.assertEqual(matrix.service_origin("http://127.0.0.1:123/"), "http://127.0.0.1:123")
        for url in ("http://u:p@localhost", "http://localhost/v1", "http://localhost?key=x", "http://localhost#x", "file:///tmp"):
            with self.assertRaises(ValueError):
                matrix.service_origin(url)
        with self.assertRaises(urllib.error.HTTPError) as raised:
            matrix.NoRedirect().redirect_request(urllib.request.Request("http://localhost"), None, 302, "", {}, "http://elsewhere")
        raised.exception.close()

    def test_complete_stream_and_cancellation(self):
        chunk = {"choices": [{"delta": {"content": "hello"}, "finish_reason": None}]}
        data = b": keepalive\n\n" + b"data: " + json.dumps(chunk).encode() + b"\n\ndata: [DONE]\n"
        result = matrix.streamed(Opener(data), None, 10)
        self.assertTrue(result["done"])
        self.assertEqual(result["content"], "hello")
        self.assertTrue(matrix.streamed(Opener(data), None, 10, True)["cancelled_after_content"])
        for broken in (b'data: {"error":"failed"}\n', b"data: bad-json\n", b": keepalive\n"):
            with self.assertRaises(ValueError):
                matrix.streamed(Opener(broken), None, 10)

    def test_tool_contract(self):
        call = {"id": "c", "type": "function", "function": {"name": "lookup_fixture", "arguments": '{"id":"fixture-1"}'}}
        result = {"choices": [{"finish_reason": "tool_calls", "message": {"tool_calls": [call]}}]}
        self.assertEqual(matrix.valid_call(result), call)
        result["choices"][0]["message"]["tool_calls"].append(call)
        self.assertIsNone(matrix.valid_call(result))


if __name__ == "__main__":
    unittest.main()
