"""Opt-in installed-model checks via OffGrid's authenticated, scheduled API.

No synthetic tool is executed. Optional external workflows use the existing
disposable-home/file-only harness. No downloads, default changes or replay.
Raw test responses stay in a private temporary directory, not ordinary logs.
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request


MARKER = "OFFGRID_MATRIX_OK"
TOOL = {"type": "function", "function": {"name": "lookup_fixture", "description": "Read a synthetic test record; this test does not execute the function.",
        "parameters": {"type": "object", "properties": {"id": {"type": "string"}}, "required": ["id"], "additionalProperties": False}}}


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise urllib.error.HTTPError(req.full_url, code, "Redirect refused", headers, fp)


def service_origin(value):
    base = value.rstrip("/")
    parsed = urllib.parse.urlparse(base)
    if (parsed.scheme not in ("http", "https") or not parsed.hostname or parsed.username
            or parsed.password or parsed.path or parsed.query or parsed.fragment):
        raise ValueError("--server must be an HTTP(S) origin without credentials, path, query or fragment")
    return base


def streamed(opener, request, timeout, cancel_on_content=False):
    started = time.monotonic()
    first = None
    chunks, content, finish, usage = [], "", None, None
    size = 0
    with opener.open(request, timeout=min(timeout, 30)) as response:
        for line in response:
            if time.monotonic() - started > timeout:
                raise TimeoutError("Stream deadline exceeded")
            size += len(line)
            if size > 4 * 1024 * 1024:
                raise ValueError("Oversized stream")
            if not line.startswith(b"data:"):
                continue
            data = line[5:].strip()
            if data == b"[DONE]":
                return {"content": content, "finish_reason": finish, "usage": usage,
                        "first_content_seconds": first, "done": True, "chunks": chunks}
            chunk = json.loads(data)
            chunks.append(chunk)
            if chunk.get("error"):
                raise ValueError("Stream reported an error")
            usage = chunk.get("usage") or usage
            for choice in chunk.get("choices", []):
                text = choice.get("delta", {}).get("content") or ""
                if text and first is None:
                    first = round(time.monotonic() - started, 3)
                content += text
                finish = choice.get("finish_reason") or finish
                if text and cancel_on_content:
                    return {"first_content_seconds": first, "cancelled_after_content": True, "done": False}
    raise ValueError("Stream ended without DONE")


def valid_call(result):
    choices = result.get("choices", [])
    if len(choices) != 1 or choices[0].get("finish_reason") != "tool_calls":
        return None
    calls = choices[0].get("message", {}).get("tool_calls", [])
    if len(calls) != 1 or not calls[0].get("id") or calls[0].get("type") != "function":
        return None
    call = calls[0]
    try:
        if call["function"]["name"] == "lookup_fixture" and json.loads(call["function"]["arguments"]) == {"id": "fixture-1"}:
            return call
    except (KeyError, TypeError, ValueError):
        pass
    return None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--server", default="http://127.0.0.1:11611")
    parser.add_argument("--model", action="append", help="Optional installed IDs; otherwise discover all chat models")
    parser.add_argument("--run", action="store_true", help="Explicitly allow model loading and synthetic inference")
    parser.add_argument("--binary", type=Path, help="Also run both eligible external runtimes with disposable file workflows")
    parser.add_argument("--timeout", type=int, default=180)
    args = parser.parse_args()
    if not args.run:
        parser.error("--run is required; this test loads installed models and consumes local inference")
    if not 10 <= args.timeout <= 900:
        parser.error("--timeout must be 10–900 seconds")
    try:
        base = service_origin(args.server)
    except ValueError as error:
        parser.error(str(error))
    os.umask(0o077)
    root = Path(tempfile.mkdtemp(prefix="offgrid-model-matrix-"))
    print(f"Private evidence: {root}", flush=True)
    headers = {"Content-Type": "application/json"}
    if os.environ.get("OFFGRID_API_KEY"):
        headers["Authorization"] = "Bearer " + os.environ["OFFGRID_API_KEY"]
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def request(path, payload=None, timeout=None):
        call = urllib.request.Request(base + path, data=None if payload is None else json.dumps(payload).encode(), headers=headers)
        with opener.open(call, timeout=timeout or args.timeout) as response:
            return json.load(response)

    def idle():
        state = request("/health", timeout=5).get("inference", {})
        return state and not state.get("active") and not state.get("queued") and not state.get("switching")

    if not idle():
        raise SystemExit("Service is busy or has no verifiable idle state; no test started")
    catalog = request("/v1/models")["data"]
    models = [m["id"] for m in catalog if "chat" in (m.get("capabilities") or []) and m.get("type") != "embedding"]
    if args.model:
        if set(args.model) - set(models):
            raise SystemExit("An explicitly requested model is not an installed chat model")
        models = list(dict.fromkeys(args.model))
    if not models:
        raise SystemExit("No installed chat models; no qualification performed")
    records = []

    def record(model, stage, started, result, passed):
        entry = {"model": model, "stage": stage, "seconds": round(time.monotonic()-started, 3), "passed": passed, "result": result}
        records.append(entry)
        (root / "results.json").write_text(json.dumps(records, indent=2, ensure_ascii=False), encoding="utf-8")
        print(json.dumps({k: v for k, v in entry.items() if k != "result"}), flush=True)

    for model in models:
        if not idle():
            raise SystemExit("Service became busy; remaining cases were not run")
        started = time.monotonic()
        try:
            prepared = request("/v1/inference/prepare", {"model": model}, 300)
            if prepared.get("model") != model or not prepared.get("prepared"):
                raise ValueError("selected model preparation not confirmed")
            record(model, "prepare", started, prepared, True)
        except (OSError, ValueError) as error:
            record(model, "prepare", started, {"error_type": type(error).__name__}, False)
            continue
        messages = [{"role": "user", "content": f"Reply with exactly {MARKER}. Do not call any tools."}]
        base_request = {"model": model, "max_tokens": 192, "temperature": 0, "parallel_tool_calls": False, "tools": [TOOL]}
        for sample in ("prepared_greeting", "repeat_greeting"):
            started = time.monotonic()
            try:
                result = request("/v1/chat/completions", dict(base_request, messages=messages))
                choice = result["choices"][0]
                passed = choice.get("finish_reason") == "stop" and choice["message"].get("content", "").strip() == MARKER and not choice["message"].get("tool_calls")
                record(model, sample, started, result, passed)
            except (OSError, ValueError, KeyError, IndexError, TypeError) as error:
                record(model, sample, started, {"error_type": type(error).__name__}, False)
        started = time.monotonic()
        try:
            payload = dict(base_request, messages=messages, stream=True, stream_options={"include_usage": True})
            call = urllib.request.Request(base + "/v1/chat/completions", data=json.dumps(payload).encode(), headers=headers)
            result = streamed(opener, call, args.timeout)
            passed = result["content"].strip() == MARKER and result["finish_reason"] == "stop"
            record(model, "warm_stream", started, result, passed)
        except (OSError, ValueError, KeyError, TypeError) as error:
            record(model, "warm_stream", started, {"error_type": type(error).__name__}, False)
        started = time.monotonic()
        try:
            brief = [{"role": "user", "content": "Call lookup_fixture exactly once with id fixture-1."}]
            result = request("/v1/chat/completions", dict(base_request, messages=brief, tool_choice="required"))
            call = valid_call(result)
            record(model, "synthetic_tool", started, result, call is not None)
            if call:
                assistant_call = result["choices"][0]["message"]
                for stage, tool_result in (("tool_result", MARKER), ("unicode_tool_result", "OffGrid test — café 日本語 12345")):
                    started = time.monotonic()
                    try:
                        followup = brief + [assistant_call, {"role": "tool", "tool_call_id": call["id"], "content": tool_result},
                                             {"role": "user", "content": "Return the tool result exactly, without another tool call."}]
                        result = request("/v1/chat/completions", dict(base_request, messages=followup))
                        choice = result["choices"][0]
                        passed = choice.get("finish_reason") == "stop" and not choice["message"].get("tool_calls") and choice["message"].get("content", "").strip() == tool_result
                        record(model, stage, started, result, passed)
                    except (OSError, ValueError, KeyError, IndexError, TypeError) as error:
                        record(model, stage, started, {"error_type": type(error).__name__}, False)
        except (OSError, ValueError, KeyError, IndexError, TypeError) as error:
            record(model, "synthetic_tool_or_result", started, {"error_type": type(error).__name__}, False)
        started = time.monotonic()
        try:
            payload = {"model": model, "stream": True, "max_tokens": 1024, "temperature": 0,
                       "messages": [{"role": "user", "content": "Count from one to one hundred, with each number on its own line."}]}
            call = urllib.request.Request(base + "/v1/chat/completions", data=json.dumps(payload).encode(), headers=headers)
            result = streamed(opener, call, args.timeout, cancel_on_content=True)
            cancelled = time.monotonic()
            while not idle() and time.monotonic() - cancelled < 10:
                time.sleep(0.2)
            result["idle_after_disconnect"] = bool(idle())
            result["disconnect_to_idle_seconds"] = round(time.monotonic() - cancelled, 3)
            record(model, "cancel_stream", started, result, result.get("cancelled_after_content") and result["idle_after_disconnect"])
        except (OSError, ValueError, KeyError, TypeError) as error:
            record(model, "cancel_stream", started, {"error_type": type(error).__name__}, False)
        for agent in ("hermes", "openclaw"):
            started = time.monotonic()
            setup = request(f"/v1/integrations/{agent}/setup?" + urllib.parse.urlencode({"model": model}))
            eligible = setup["integration"]["ready"]
            record(model, agent+"_eligibility", started, setup["integration"], eligible)
            if args.binary and eligible:
                # The child harness owns its process-group cancellation and private state.
                completed = subprocess.run([sys.executable, str(Path(__file__).with_name("test-external-agents.py")), str(args.binary.resolve()),
                                            "--server", base, "--agent", agent, "--model", model, "--workflow"], check=False)
                record(model, agent+"_workflow", started, {"exit_code": completed.returncode}, completed.returncode == 0)
    print("Results are per tested model/runtime; this is not universal model qualification.")
    return 0 if records and all(r["passed"] for r in records if not r["stage"].endswith("_eligibility")) else 1


if __name__ == "__main__":
    raise SystemExit(main())
