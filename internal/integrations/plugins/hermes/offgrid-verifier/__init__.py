"""Reviewed Hermes 0.21.2 hooks. No tools, credentials or results are rewritten.

This is outcome verification, not a sandbox or an authority provider. The CLI
checks the installed bundle and runtime revision, and owns final verification.
"""
import json
import os
import re
import threading
import urllib.request

ADAPTER = "hermes-0.21.2-files-v1"
TOOLS = {"read_file": "read", "write_file": "write"}


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise ValueError("verification redirects are forbidden")


def register(ctx):
    # Consume credentials so tool subprocesses do not inherit them. They are
    # still accessible to the trusted host process: this is not OS isolation.
    env = {k: os.environ.pop("OFFGRID_VERIFY_" + k, "")
           for k in ("URL", "TOKEN", "RUN", "ADAPTER")}
    if not any(env.values()):
        return
    lock = threading.RLock()
    state = {"session": "", "failed": False}
    blocked = set()
    pending = {}
    valid = (env["ADAPTER"] == ADAPTER and
             re.fullmatch(r"http://127\.0\.0\.1:[1-9][0-9]{0,4}", env["URL"]) and
             re.fullmatch(r"[0-9a-f]{64}", env["TOKEN"]) and
             re.fullmatch(r"[0-9a-f]{48}", env["RUN"]))
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def send(route, **fields):
        if not valid or state["failed"]:
            raise ValueError("verification unavailable")
        payload = dict(version=1, adapter=ADAPTER, run=env["RUN"], **fields)
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        if len(body) > 8 * 1024 * 1024:
            raise ValueError("verification event too large")
        req = urllib.request.Request(env["URL"] + route, data=body, headers={
            "Authorization": "Bearer " + env["TOKEN"], "Content-Type": "application/json"})
        with opener.open(req, timeout=3) as response:
            data = response.read(4097)
            if len(data) > 4096:
                raise ValueError("verification response too large")
            result = json.loads(data)
        if not isinstance(result, dict) or type(result.get("allowed")) is not bool:
            raise ValueError("invalid verification response")
        return result

    def start(session_id="", **kwargs):
        with lock:
            try:
                if state["session"]:
                    if state["session"] != session_id:
                        raise ValueError("foreign session")
                    return
                if not send("/bind", session=session_id)["allowed"]:
                    raise ValueError("binding rejected")
                state["session"] = session_id
            except Exception:
                state["failed"] = True

    def identity(kwargs):
        values = {"session": kwargs.get("session_id", ""),
                  "turn": kwargs.get("turn_id", ""), "call": kwargs.get("tool_call_id", "")}
        if not state["session"] or values["session"] != state["session"] or not all(
                isinstance(v, str) and 0 < len(v) <= 256 for v in values.values()):
            raise ValueError("uncorrelated hook")
        return values

    def before(tool_name="", args=None, **kwargs):
        if tool_name not in TOOLS:
            return None
        with lock:
            code = "verification_unavailable"
            message = "Verification is unavailable. Inspect the contract; do not claim verification succeeded."
            try:
                ids = identity(kwargs)
                from tools.terminal_scope import terminal_env
                if terminal_env("TERMINAL_ENV", "local") != "local":
                    raise ValueError("nonlocal file backend is unverified")
                params = args if isinstance(args, dict) else {}
                path = params.get("path", "")
                content = params.get("content", "") if tool_name == "write_file" else ""
                if not isinstance(path, str) or not isinstance(content, str):
                    raise ValueError("invalid file arguments")
                key = tuple(ids.values())
                decision = send("/before", **ids, kind=TOOLS[tool_name], path=path, content=content)
                if decision["allowed"]:
                    pending[key] = (tool_name, dict(params))
                    return None
                code = decision.get("code", code)
                if isinstance(decision.get("message"), str) and len(decision["message"]) <= 1024:
                    message = decision["message"]
                # This hook itself vetoes before dispatch. Some runtime veto
                # paths emit no post hook; record our own bounded veto once.
                send("/after", **ids, status="blocked")
                blocked.add(key)
            except Exception:
                state["failed"] = True
            return {"action": "block", "message": "OffGrid file contract rejected this operation: " + code +
                    ". " + message}

    def after(tool_name="", result=None, status="", **kwargs):
        if tool_name not in TOOLS:
            return
        with lock:
            try:
                ids = identity(kwargs)
                key = tuple(ids.values())
                if key in blocked:
                    blocked.remove(key)
                    return
                observed_name, params = pending.pop(key)
                if observed_name != tool_name:
                    raise ValueError("different completion tool")
                value = json.loads(result) if isinstance(result, str) else result
                complete = False
                outcome = "unknown"
                if status == "blocked":
                    outcome = "blocked"
                elif status == "error" or (isinstance(value, dict) and value.get("error")):
                    outcome = "failed"
                elif status == "ok" and isinstance(value, dict):
                    if tool_name == "write_file" and type(value.get("bytes_written")) is int:
                        outcome = "completed"
                    elif tool_name == "read_file" and isinstance(value.get("content"), str):
                        outcome = "completed"
                        complete = (value.get("truncated") is False and
                                    value.get("content_returned") is not False and
                                    value.get("is_binary") is False and value.get("is_image") is False and
                                    params.get("offset", 1) == 1 and
                                    ("limit" not in params or
                                     (type(params["limit"]) is int and params["limit"] > 0 and
                                      type(value.get("total_lines")) is int and params["limit"] >= value["total_lines"])) and
                                    type(value.get("total_lines")) is int and type(value.get("file_size")) is int)
                send("/after", **ids, status=outcome, complete_read=complete)
            except Exception:
                # Never raise into Hermes' fail-open observer path. A missing
                # completion remains unverified and future file calls are vetoed.
                state["failed"] = True

    def end(session_id="", completed=False, failed=False, interrupted=False, **kwargs):
        with lock:
            try:
                if session_id != state["session"] or not completed or failed or interrupted or pending:
                    raise ValueError("incomplete lifecycle")
                send("/end", session=session_id, status="completed")
            except Exception:
                state["failed"] = True

    def context(session_id="", **kwargs):
        with lock:
            if state["failed"] or session_id != state["session"]:
                return None
            return {"context": (
                "OffGrid file-contract instructions for this invocation: follow the requested task; "
                "do not create files for a greeting. For exact copies, Hermes read_file adds a display "
                "line number that is NOT a source byte. Example: returned content 1|hello means the "
                "file text is hello. Returned content 1|1|hello means the file text is 1|hello. "
                "Preserve all actual Unicode, spaces and line endings. Never copy the outer display "
                "prefix or JSON wrapper into write_file.content. A blocked write created nothing; "
                "correct the rejected proposal before reading its destination. After an accepted "
                "write, this explicit contract requires independent read-back even if the ordinary "
                "write tool says verified:true. Do not claim success until the contract passes.")}

    ctx.register_hook("on_session_start", start)
    ctx.register_hook("pre_llm_call", context)
    ctx.register_hook("pre_tool_call", before)
    ctx.register_hook("post_tool_call", after)
    ctx.register_hook("on_session_end", end)
