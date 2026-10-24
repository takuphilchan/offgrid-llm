"""Offline hooks with pinned Hermes 0.21.2 payloads; no model or user home."""
import importlib.util
import json
import os
import sys
import types
from pathlib import Path
import unittest
from unittest.mock import patch

sys.dont_write_bytecode = True

SOURCE = Path(__file__).resolve().parents[2] / "internal/integrations/plugins/hermes/offgrid-verifier/__init__.py"
spec = importlib.util.spec_from_file_location("offgrid_verifier", SOURCE)
plugin = importlib.util.module_from_spec(spec)
spec.loader.exec_module(plugin)


class Context:
    def __init__(self): self.hooks = {}
    def register_hook(self, name, callback): self.hooks[name] = callback


class Response:
    def __init__(self, allowed): self.allowed = allowed
    def __enter__(self): return self
    def __exit__(self, *args): pass
    def read(self, size): return json.dumps(dict(allowed=self.allowed, code="fixture", message="Specific shared rejection.")).encode()


class HooksTest(unittest.TestCase):
    def setup_hooks(self, reject=False, unavailable=False, active=True):
        context, calls = Context(), []
        class Opener:
            def open(self, req, timeout):
                if unavailable: raise OSError("offline")
                calls.append((req.full_url, json.loads(req.data)))
                return Response(not (reject and req.full_url.endswith("/before")))
        env = {"OFFGRID_VERIFY_URL": "http://127.0.0.1:12345", "OFFGRID_VERIFY_TOKEN": "a" * 64,
               "OFFGRID_VERIFY_RUN": "b" * 48, "OFFGRID_VERIFY_ADAPTER": plugin.ADAPTER} if active else {}
        self.enterContext(patch.dict(sys.modules, {"tools.terminal_scope": types.SimpleNamespace(terminal_env=lambda _key, default: default)}))
        with patch.dict(os.environ, env, clear=True), patch.object(plugin.urllib.request, "build_opener", return_value=Opener()):
            plugin.register(context)
            self.assertFalse(any(k.startswith("OFFGRID_VERIFY_") for k in os.environ))
        return context.hooks, calls

    def test_inactive(self):
        hooks, calls = self.setup_hooks(active=False)
        self.assertEqual(hooks, {})

    def test_invocation_context_preserves_results_and_requires_bound_session(self):
        hooks,_=self.setup_hooks()
        self.assertIsNone(hooks["pre_llm_call"](session_id="foreign"))
        hooks["on_session_start"](session_id="s")
        value=hooks["pre_llm_call"](session_id="s")["context"]
        self.assertIn("1|1|hello",value)
        self.assertIn("independent read-back",value)
        self.assertIsNone(hooks["pre_llm_call"](session_id="foreign"))

    def test_real_read_shape_and_lifecycle(self):
        hooks, calls = self.setup_hooks()
        ids = dict(session_id="s", turn_id="t", tool_call_id="c")
        hooks["on_session_start"](session_id="s")
        args = dict(path="source.txt")
        self.assertIsNone(hooks["pre_tool_call"](tool_name="read_file", args=args, **ids))
        result = json.dumps(dict(content="1| café 日本語", total_lines=1, file_size=18,
                                 truncated=False, is_binary=False, is_image=False))
        self.assertIsNone(hooks["post_tool_call"](tool_name="read_file", args=args, result=result, status="ok", **ids))
        hooks["on_session_end"](session_id="s", completed=True)
        self.assertTrue(calls[2][1]["complete_read"])
        self.assertNotIn("content", calls[2][1])
        self.assertTrue(calls[-1][0].endswith("/end"))
        self.assertEqual(args, dict(path="source.txt"))

    def test_partial_deduplicated_and_unknown_results(self):
        for extra in [dict(truncated=True), dict(content_returned=False), dict(is_binary=True), dict(content=None)]:
            hooks, calls = self.setup_hooks()
            ids = dict(session_id="s", turn_id="t", tool_call_id="c")
            hooks["on_session_start"](session_id="s")
            hooks["pre_tool_call"](tool_name="read_file", args=dict(path="source"), **ids)
            value = dict(content="x", total_lines=1, file_size=1, truncated=False, is_binary=False, is_image=False)
            value.update(extra)
            hooks["post_tool_call"](tool_name="read_file", result=json.dumps(value), status="ok", **ids)
            self.assertFalse(calls[-1][1]["complete_read"])

    def test_explicit_limit_covering_the_entire_file_is_complete(self):
        for limit,total,truncated,complete in [(1000,0,False,True),(2000,2,False,True),(1,2,True,False),(0,0,False,False)]:
            hooks,calls=self.setup_hooks()
            ids=dict(session_id="s",turn_id="t",tool_call_id="c")
            hooks["on_session_start"](session_id="s")
            hooks["pre_tool_call"](tool_name="read_file",args=dict(path="source",limit=limit),**ids)
            value=dict(content="1|text",total_lines=total,file_size=4,truncated=truncated,is_binary=False,is_image=False)
            hooks["post_tool_call"](tool_name="read_file",result=json.dumps(value),status="ok",**ids)
            self.assertEqual(calls[-1][1]["complete_read"],complete)

    def test_veto_no_rewriting_or_double_completion(self):
        hooks, calls = self.setup_hooks(reject=True)
        ids = dict(session_id="s", turn_id="t", tool_call_id="c")
        hooks["on_session_start"](session_id="s")
        args = dict(path="copy", content="1| café 日本語")
        decision = hooks["pre_tool_call"](tool_name="write_file", args=args, **ids)
        self.assertEqual(decision["action"], "block")
        self.assertIn("Specific shared rejection.", decision["message"])
        self.assertNotIn("absolute local", decision["message"])
        self.assertEqual(calls[-1][1]["status"], "blocked")
        hooks["post_tool_call"](tool_name="write_file", result='{"error":"veto"}', status="blocked", **ids)
        self.assertEqual(len(calls), 3)
        self.assertEqual(args["content"], "1| café 日本語")

    def test_missing_failed_and_foreign_hooks_cannot_close(self):
        for unavailable in (False, True):
            hooks, calls = self.setup_hooks(unavailable=unavailable)
            ids = dict(session_id="foreign", turn_id="t", tool_call_id="c")
            hooks["on_session_start"](session_id="s")
            self.assertEqual(hooks["pre_tool_call"](tool_name="write_file", args=dict(path="x",content="y"), **ids)["action"], "block")
            self.assertIsNone(hooks["pre_tool_call"](tool_name="web_search", args=dict(private="kept"), **ids))
            hooks["on_session_end"](session_id="s", completed=True)
            self.assertFalse(any(url.endswith("/end") for url, _ in calls))


if __name__ == "__main__": unittest.main()
