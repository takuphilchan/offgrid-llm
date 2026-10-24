"""Opt-in deterministic installed-hook contract test, not a model workflow test.

The Go test supplies a disposable HERMES_HOME and workspace. No user config,
external account, model, desktop or network service is used except the verifier.
"""
import json
import os
from pathlib import Path
import sys

sys.path.insert(0, os.environ["OFFGRID_HERMES_TEST_ROOT"])
from hermes_cli.lifecycle import invoke_hook
from hermes_cli.plugins import get_plugin_manager
import tools.file_tools  # registers the actual reviewed file tool implementations
from model_tools import handle_function_call

workspace = Path.cwd()
assert workspace.name == "workspace", "requires the Go test's disposable workspace"
manager = get_plugin_manager()
manager.discover_and_load()
assert manager.has_hook("pre_tool_call"), "verifier plugin was not loaded"
invoke_hook("on_session_start", session_id="fixture-session")
content = (workspace / "source.txt").read_text(encoding="utf-8")
for call, name, args in [
    ("read-source", "read_file", {"path": str(workspace / "source.txt")}),
    ("bad-write", "write_file", {"path": str(workspace / "copy.txt"), "content": "wrong"}),
    ("good-write", "write_file", {"path": str(workspace / "copy.txt"), "content": content}),
    ("read-copy", "read_file", {"path": str(workspace / "copy.txt")}),
]:
    raw = handle_function_call(name, args, task_id="fixture-task", session_id="fixture-session",
                              turn_id="fixture-turn", tool_call_id=call)
    result = json.loads(raw)
    if call == "bad-write":
        assert not (workspace / "copy.txt").exists(), "veto failed"
    else:
        assert not result.get("error"), (call, result.get("error"))
invoke_hook("on_session_end", session_id="fixture-session", completed=True, failed=False, interrupted=False)
assert (workspace / "copy.txt").read_bytes() == (workspace / "source.txt").read_bytes()
print("installed Hermes discovery, typed veto, native file tools and lifecycle passed")
