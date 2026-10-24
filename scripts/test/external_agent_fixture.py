"""Deterministic inference fixture, never a real-model qualification result."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
import re
import select
import socket
import threading
import time


def start_fixture(root, model):
    events = []
    events_lock = threading.Lock()
    generation = threading.Event()
    cancelled = threading.Event()
    source = root / "workspace/source.txt"
    target = root / "workspace/copy.txt"

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def send(self, payload, code=200):
            data = json.dumps(payload).encode()
            self.send_response(code)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def do_GET(self):
            base = f"http://127.0.0.1:{self.server.server_port}/v1"
            if self.path.startswith("/v1/models"):
                self.send({"object": "list", "data": [{"id": model, "object": "model", "type": "llm", "capabilities": ["chat", "tools"], "context_window": 65536}]})
                return
            match = re.match(r"/v1/integrations/(hermes|openclaw)/setup", self.path)
            if match:
                agent = match.group(1)
                profile = {"id": agent, "name": agent, "provider_id": "offgrid", "minimum_context": 64000 if agent == "hermes" else 8192,
                           "recommended_context": 65536, "context_window": 65536, "model_id": model, "ready": True, "warnings": [],
                           "contract_revision": "deterministic-fixture-v1"}
                content = json.dumps({"models": {"providers": {"offgrid": {"baseUrl": base, "apiKey": "offgrid-local", "api": "openai-completions",
                    "models": [{"id": model, "name": model, "input": ["text"], "contextWindow": 65536, "maxTokens": 4096}]}}}})
                self.send({"integration": profile, "setup": {"content": content}})
                return
            self.send({"error": "fixture endpoint unavailable"}, 404)

        def do_POST(self):
            if self.path == "/v1/inference/prepare":
                body = json.loads(self.rfile.read(min(int(self.headers.get("Content-Length", 0)), 4096)))
                if body != {"model": model}:
                    self.send({"error": "unexpected preparation identity"}, 400)
                else:
                    self.send({"model": model, "prepared": True, "context_window": 65536, "elapsed_ms": 0})
                return
            if self.path != "/v1/chat/completions":
                self.send({"error": "fixture endpoint unavailable"}, 404)
                return
            body = json.loads(self.rfile.read(min(int(self.headers.get("Content-Length", 0)), 2 << 20)))
            generation.set()
            messages = body.get("messages", [])
            tools = {item["function"]["name"]: item["function"] for item in body.get("tools", []) if item.get("type") == "function"}
            calls = [call["function"]["name"] for msg in messages for call in msg.get("tool_calls", [])]
            # Auxiliary/title requests can overlap the main response. Preserve a
            # complete evidence snapshot for readers even during cancellation.
            with events_lock:
                events.append({"tools": list(tools), "prior_calls": calls, "response_format": body.get("response_format"),
                               "parallel_tool_calls_present": "parallel_tool_calls" in body,
                               "parallel_tool_calls": body.get("parallel_tool_calls")})
                temporary = root / "fixture-events.pending"
                temporary.write_text(json.dumps(events, indent=2))
                deadline = time.monotonic() + 2
                while True:
                    try:
                        temporary.replace(root / "fixture-events.json")
                        break
                    except PermissionError:
                        # Windows readers briefly deny replacing an open file.
                        # Keep the complete previous snapshot until they close.
                        if os.name != "nt" or time.monotonic() >= deadline:
                            raise
                        time.sleep(0.01)
            message = {"role": "assistant", "content": "Fixture model reachable."}
            finish = "stop"
            text = json.dumps(messages, ensure_ascii=False)
            if "FIXTURE_CANCEL_WAIT" in text:
                # Observe cancellation at the inference connection, not just
                # the outer CLI's exit. A detached worker must not keep retrying.
                deadline = time.monotonic() + 20
                while time.monotonic() < deadline:
                    ready, _, _ = select.select([self.connection], [], [], 0.1)
                    if ready and not self.connection.recv(1, socket.MSG_PEEK):
                        cancelled.set()
                        return
            elif body.get("response_format", {}).get("type") == "json_schema":
                message["content"] = json.dumps({"title": "Isolated fixture greeting"})
            elif "HERMES_OFFGRID_OK" in text:
                message["content"] = "HERMES_OFFGRID_OK"
            elif "FIXTURE_FALSE_COMPLETION" in text:
                message["content"] = "I copied the file and verified an exact match."
            elif str(source) in text:
                read = next((name for name in tools if name in ["read", "read_file"]), None)
                write = next((name for name in tools if name in ["write", "write_file"]), None)
                read_count = calls.count(read) if read else 0
                name = read if read_count == 0 else write if write not in calls else read if read_count == 1 else None
                if name:
                    properties = tools[name].get("parameters", {}).get("properties", {})
                    path_key = next((key for key in ["path", "file_path", "filename"] if key in properties), "path")
                    arguments = {path_key: str(source if read_count == 0 else target)}
                    if name == write:
                        arguments["content"] = source.read_text(encoding="utf-8")
                    message = {"role": "assistant", "content": None, "tool_calls": [{"id": f"fixture_{len(calls)}", "type": "function", "function": {"name": name, "arguments": json.dumps(arguments, ensure_ascii=False)}}]}
                    finish = "tool_calls"
                else:
                    message["content"] = "Fixture finished; independent read-back determines success."
            result = {"id": "chatcmpl-fixture", "object": "chat.completion", "created": int(time.time()), "model": model,
                      "choices": [{"index": 0, "message": message, "finish_reason": finish}], "usage": {"prompt_tokens": 128, "completion_tokens": 32, "total_tokens": 160}}
            try:
                if body.get("stream"):
                    self.send_response(200)
                    self.send_header("Content-Type", "text/event-stream")
                    self.end_headers()
                    delta = dict(message)
                    if "tool_calls" in delta:
                        delta["tool_calls"] = [dict(call, index=i) for i, call in enumerate(delta["tool_calls"])]
                    for value, reason in [(delta, None), ({}, finish)]:
                        chunk = dict(result, object="chat.completion.chunk", choices=[{"index": 0, "delta": value, "finish_reason": reason}])
                        self.wfile.write(("data: " + json.dumps(chunk) + "\n\n").encode())
                    self.wfile.write(b"data: [DONE]\n\n")
                else:
                    self.send(result)
            except (BrokenPipeError, ConnectionResetError):
                pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.cancelled = cancelled
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server, generation
