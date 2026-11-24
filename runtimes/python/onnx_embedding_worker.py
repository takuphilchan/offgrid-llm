#!/usr/bin/env python3
"""Offline CPU embeddings for the existing supervised provider-v1 boundary.

No install/download/cache discovery or custom operator loading is provided.
Synthetic protocol tests do not qualify semantic retrieval or native runtimes.
"""
from __future__ import annotations

import argparse
import base64
import hashlib
from importlib import metadata
import json
import math
import os
from pathlib import Path
import re
import stat
import sys
import threading
from typing import Any

PROTOCOL = "provider-v1"
MAX_FRAME = 1024 * 1024
MAX_VALUES = 32768
DEPENDENCIES = {"onnxruntime", "onnx", "numpy", "tokenizers"}


def decode_argument(value: str) -> Any:
    if len(value) > MAX_FRAME:
        raise ValueError("worker argument exceeds byte limit")
    return strict_json(base64.b64decode(value + "=" * (-len(value) % 4), altchars=b"-_", validate=True))


def constrain_python_paths(directory: str) -> None:
    root = Path(directory).resolve(strict=True)
    def inside(value):
        path = Path(value).resolve()
        if not path.is_relative_to(root):
            raise ValueError("Python installation escapes pinned runtime")
        return path
    for value in (sys.executable, __file__, sys.prefix, sys.base_prefix, *sys.path):
        if not value:
            raise ValueError("Python current-directory search is forbidden")
        inside(value)
    # -S disables .pth and sitecustomize execution. Dependencies are loaded only
    # from this fixed installation directory, already covered by the file lease.
    relative = Path("Lib") if os.name == "nt" else Path("lib") / f"python{sys.version_info.major}.{sys.version_info.minor}"
    dependencies = inside(str(Path(sys.prefix) / relative / "site-packages"))
    if not dependencies.is_dir():
        raise ValueError("pinned Python dependencies are unavailable")
    sys.path.append(str(dependencies))


def strict_json(raw: str | bytes) -> Any:
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError("repeated JSON key")
            result[key] = value
        return result

    def invalid(_value):
        raise ValueError("non-finite JSON number")

    value = json.loads(raw, object_pairs_hook=unique, parse_constant=invalid)
    json.dumps(value, ensure_ascii=False, allow_nan=False).encode("utf-8")
    return value


def positive(value, maximum):
    return type(value) is int and 1 <= value <= maximum


def validate_settings(value: Any) -> dict[str, Any]:
    required = {"schema_version", "family", "inputs", "output", "pooling", "normalize_l2",
                "dimensions", "max_tokens", "pad_token", "pad_token_id"}
    if not isinstance(value, dict) or set(value) != required:
        raise ValueError("incomplete embedding settings")
    if type(value["schema_version"]) is not int or value["schema_version"] != 1 or value["family"] != "bert-symmetric":
        raise ValueError("unsupported embedding settings or model family")
    if value["inputs"] not in (["input_ids", "attention_mask"], ["input_ids", "attention_mask", "token_type_ids"]):
        raise ValueError("unsupported embedding inputs")
    if not isinstance(value["output"], str) or not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_./-]{0,127}", value["output"]):
        raise ValueError("invalid embedding output")
    if value["pooling"] not in {"mean", "cls", "last", "none"} or type(value["normalize_l2"]) is not bool:
        raise ValueError("unsupported embedding pooling")
    if not positive(value["dimensions"], 4096) or not positive(value["max_tokens"], 8192):
        raise ValueError("embedding tensor bounds are invalid")
    if not isinstance(value["pad_token"], str) or not value["pad_token"] or len(value["pad_token"].encode()) > 128:
        raise ValueError("invalid padding token")
    if type(value["pad_token_id"]) is not int or not 0 <= value["pad_token_id"] < 2**32:
        raise ValueError("invalid padding token ID")
    return value


def verify_dependencies(pins: Any) -> None:
    if not isinstance(pins, dict) or not DEPENDENCIES <= set(pins) or len(pins) > 32:
        raise ValueError("incomplete dependency inventory")
    for name, version in pins.items():
        if not isinstance(name, str) or not re.fullmatch(r"[a-z0-9][a-z0-9.-]*", name) or not isinstance(version, str) or not re.fullmatch(r"[0-9][0-9A-Za-z.!+_-]*", version):
            raise ValueError("invalid dependency identity")
        if metadata.version(name) != version:
            raise ValueError("installed dependency identity changed")


def read_artifact(directory: Path, name: str, limit: int) -> bytes:
    path = directory / name
    before = path.lstat()
    if not stat.S_ISREG(before.st_mode) or not 0 < before.st_size <= limit:
        raise ValueError("local embedding artifact is unavailable")
    flags = os.O_RDONLY | getattr(os, "O_NONBLOCK", 0) | getattr(os, "O_NOFOLLOW", 0) | getattr(os, "O_BINARY", 0)
    descriptor = os.open(path, flags)
    with os.fdopen(descriptor, "rb") as source:
        opened = os.fstat(source.fileno())
        if not stat.S_ISREG(opened.st_mode) or not os.path.samestat(before, opened):
            raise ValueError("embedding artifact identity changed")
        raw = source.read(limit + 1)
        after = os.fstat(source.fileno())
        if after.st_size != opened.st_size or after.st_mtime_ns != opened.st_mtime_ns or len(raw) != opened.st_size:
            raise ValueError("embedding artifact changed while reading")
    if not raw or len(raw) > limit:
        raise ValueError("embedding artifact exceeds byte limit")
    return raw


def read_model_snapshot(directory: Path, pins: Any) -> dict[str, bytes]:
    limits = {"model.onnx": 512 * 1024 * 1024, "tokenizer.json": 16 * 1024 * 1024, "embedding.json": 65536}
    if not isinstance(pins, dict) or set(pins) != set(limits):
        raise ValueError("incomplete model artifact pins")
    snapshots = {}
    for name, limit in limits.items():
        if not isinstance(pins[name], str) or not re.fullmatch(r"[a-f0-9]{64}", pins[name]):
            raise ValueError("invalid model artifact pin")
        raw = read_artifact(directory, name, limit)
        if hashlib.sha256(raw).hexdigest() != pins[name]:
            raise ValueError("model artifact identity changed")
        snapshots[name] = raw
    return snapshots


def reject_external_graph(model: Any) -> None:
    """Walk protobuf messages, including nested graphs and sparse tensors."""
    pending, count = [(model, 0)], 0
    while pending:
        message, depth = pending.pop()
        count += 1
        if count > 1000000 or depth > 128:
            raise ValueError("embedding graph metadata exceeds bounds")
        name = message.DESCRIPTOR.full_name
        if name.endswith("TensorProto") and (getattr(message, "external_data", ()) or getattr(message, "data_location", 0) != 0):
            raise ValueError("external graph data is unsupported")
        if name.endswith(("NodeProto", "FunctionProto", "OperatorSetIdProto")) and getattr(message, "domain", "") not in {"", "ai.onnx"}:
            raise ValueError("custom graph execution is unsupported")
        for field, value in message.ListFields():
            if field.type != field.TYPE_MESSAGE:
                continue
            repeated = getattr(field, "is_repeated", None)
            if repeated is None:
                repeated = field.label == field.LABEL_REPEATED
            children = value if repeated else (value,)
            for child in children:
                if len(pending) > 1000000 - count:
                    raise ValueError("embedding graph metadata exceeds bounds")
                pending.append((child, depth + 1))


class Worker:
    def __init__(self, args, descriptor):
        self.args, self.descriptor = args, descriptor
        self.lock, self.active_lock = threading.Lock(), threading.Lock()
        self.active = None
        self.seen = set()
        self.thread = None
        if descriptor.get("protocol_revision") != PROTOCOL or descriptor.get("provider") != "onnx" or descriptor.get("format") != "onnx" or descriptor.get("backend") != "cpu" or descriptor.get("capabilities") != ["embeddings"]:
            raise ValueError("ONNX worker identity or CPU capability mismatch")
        if args.provider != "onnx" or args.backend != "cpu" or not descriptor.get("model_digest") or not descriptor.get("adapter_revision"):
            raise ValueError("ONNX model and adapter identity are required")
        verify_dependencies(args.dependencies)
        directory = Path(args.model).resolve(strict=True)
        snapshots = read_model_snapshot(directory, args.artifacts)
        raw_settings = snapshots["embedding.json"]
        self.settings = validate_settings(strict_json(raw_settings))
        if descriptor.get("preprocess_digest") != "sha256:" + hashlib.sha256(raw_settings).hexdigest() or descriptor.get("context_length") != self.settings["max_tokens"]:
            raise ValueError("embedding preprocessing identity changed")
        # Parse exactly the immutable buffers whose hashes matched the trusted
        # service lease. Later filesystem mutations cannot change these inputs.
        graph, tokenizer = snapshots["model.onnx"], snapshots["tokenizer.json"]
        import numpy as np
        import onnx
        import onnxruntime as ort
        from tokenizers import Tokenizer
        model = onnx.load_model_from_string(graph)
        reject_external_graph(model)
        onnx.checker.check_model(model)
        self.np, self.ort = np, ort
        self.tokenizer = Tokenizer.from_str(tokenizer.decode("utf-8"))
        self.tokenizer.no_truncation()
        self.tokenizer.no_padding()
        if self.tokenizer.token_to_id(self.settings["pad_token"]) != self.settings["pad_token_id"]:
            raise ValueError("tokenizer padding identity differs from settings")
        options = ort.SessionOptions()
        options.intra_op_num_threads = args.threads
        options.inter_op_num_threads = 1
        options.execution_mode = ort.ExecutionMode.ORT_SEQUENTIAL
        options.enable_profiling = False
        options.log_severity_level = 4
        ort.disable_telemetry_events()
        self.session = ort.InferenceSession(graph, sess_options=options, providers=["CPUExecutionProvider"])
        self.session.disable_fallback()
        if self.session.get_providers() != ["CPUExecutionProvider"]:
            raise ValueError("ONNX runtime changed execution provider")
        inputs = self.session.get_inputs()
        if {item.name for item in inputs} != set(self.settings["inputs"]) or any(item.type != "tensor(int64)" or len(item.shape) != 2 for item in inputs):
            raise ValueError("ONNX input tensor contract mismatch")
        outputs = [item for item in self.session.get_outputs() if item.name == self.settings["output"]]
        rank = 2 if self.settings["pooling"] == "none" else 3
        if len(outputs) != 1 or outputs[0].type != "tensor(float)" or len(outputs[0].shape) != rank or isinstance(outputs[0].shape[-1], int) and outputs[0].shape[-1] != self.settings["dimensions"]:
            raise ValueError("ONNX output tensor contract mismatch")

    def resources(self):
        return {"backend": "cpu", "device": "CPU", "memory_bytes": 0,
                "memory_limit_bytes": 0, "memory_measured": False}

    def send(self, kind, request="", sequence=0, **fields):
        value = {"protocol_revision": PROTOCOL, "type": kind, "request_id": request, "sequence": sequence, **fields}
        encoded = json.dumps(value, ensure_ascii=False, allow_nan=False, separators=(",", ":")).encode("utf-8")
        if len(encoded) >= MAX_FRAME:
            raise ValueError("embedding result exceeds protocol limit")
        with self.lock:
            sys.stdout.buffer.write(encoded + b"\n")
            sys.stdout.buffer.flush()

    def encode(self, texts):
        if not isinstance(texts, list) or not 1 <= len(texts) <= 32 or len(texts) * self.settings["dimensions"] > MAX_VALUES:
            raise ValueError("embedding batch exceeds bounds")
        if any(not isinstance(text, str) or not text.strip() or len(text.encode("utf-8")) > 32768 for text in texts):
            raise ValueError("embedding input exceeds text bounds")
        encoded = self.tokenizer.encode_batch(texts, add_special_tokens=True)
        lengths = [len(item.ids) for item in encoded]
        if len(encoded) != len(texts) or any(not 1 <= length <= self.settings["max_tokens"] for length in lengths):
            raise ValueError("embedding token limit exceeded; input was not truncated")
        length = max(lengths)
        for item in encoded:
            item.pad(length, direction="right", pad_id=self.settings["pad_token_id"], pad_type_id=0, pad_token=self.settings["pad_token"])
        fields = {"input_ids": [item.ids for item in encoded], "attention_mask": [item.attention_mask for item in encoded], "token_type_ids": [item.type_ids for item in encoded]}
        tensors = {name: self.np.asarray(fields[name], dtype=self.np.int64) for name in self.settings["inputs"]}
        return tensors, sum(lengths)

    def vectors(self, outputs, mask):
        np, settings = self.np, self.settings
        if len(outputs) != 1:
            raise ValueError("embedding output count mismatch")
        value = outputs[0]
        expected = (mask.shape[0], settings["dimensions"])
        if settings["pooling"] == "none":
            if value.shape != expected:
                raise ValueError("embedding sentence tensor shape mismatch")
        else:
            if value.shape != (*mask.shape, settings["dimensions"]):
                raise ValueError("embedding token tensor shape mismatch")
            if settings["pooling"] == "mean":
                value = (value * mask[:, :, None]).sum(axis=1) / mask.sum(axis=1)[:, None]
            elif settings["pooling"] == "cls":
                value = value[:, 0, :]
            else:
                value = value[np.arange(mask.shape[0]), mask.sum(axis=1) - 1, :]
        if value.shape != expected or not np.isfinite(value).all():
            raise ValueError("invalid embedding values")
        norms = np.linalg.norm(value.astype(np.float64), axis=1)
        if not np.isfinite(norms).all() or (norms <= 0).any():
            raise ValueError("invalid embedding norms")
        if settings["normalize_l2"]:
            value = value / norms[:, None]
        result = value.astype(np.float32).tolist()
        if any(not math.isfinite(number) for vector in result for number in vector):
            raise ValueError("embedding conversion overflow")
        return result

    def generate(self, request, stop, options):
        request_id = request["request_id"]
        sequence = 1

        def terminal(kind, **fields):
            # The parent can issue its next request as soon as this frame is
            # visible. Keep admission locked until the previous active identity
            # is cleared, and never let this thread's finally clear its successor.
            with self.active_lock:
                self.send(kind, request_id, sequence, **fields)
                if self.active is not None and self.active[1] is stop:
                    self.active = None

        try:
            tensors, tokens = self.encode(request["prompt"]["input"])
            if stop.is_set():
                terminal("done", finish_reason="cancelled")
                return
            outputs = self.session.run([self.settings["output"]], tensors, options)
            vectors = self.vectors(outputs, tensors["attention_mask"])
            if stop.is_set():
                terminal("done", finish_reason="cancelled")
                return
            self.send("embeddings", request_id, 1, embeddings=vectors)
            sequence = 2
            if stop.is_set():
                terminal("done", finish_reason="cancelled")
                return
            terminal("done", finish_reason="stop", usage={"prompt_tokens": tokens, "completion_tokens": 0, "total_tokens": tokens}, resources=self.resources())
        except Exception:
            if stop.is_set():
                terminal("done", finish_reason="cancelled")
            else:
                terminal("error", error={"code": "embedding_failed", "message": "Embedding inference or tensor validation failed", "operational": True})
        finally:
            with self.active_lock:
                if self.active is not None and self.active[1] is stop:
                    self.active = None

    def dispatch(self, message):
        if not isinstance(message, dict):
            raise ValueError("invalid provider control")
        operation = message.get("operation")
        if operation == "health" and set(message) == {"operation"}:
            self.send("ready", resources=self.resources())
            return
        if operation == "cancel" and set(message) == {"operation", "cancellation_id"} and isinstance(message["cancellation_id"], str):
            with self.active_lock:
                if self.active is not None and self.active[0] == message["cancellation_id"]:
                    self.active[1].set()
                    self.active[2].terminate = True
            return
        if operation != "generate" or set(message) != {"operation", "request"}:
            raise ValueError("unsupported provider control")
        request = message["request"]
        allowed = {"request_id", "cancellation_id", "model_digest", "prompt", "context_length"}
        if not isinstance(request, dict) or set(request) - allowed or not {"request_id", "cancellation_id", "model_digest", "prompt"} <= set(request):
            raise ValueError("invalid embedding request")
        if any(not isinstance(request[key], str) or not 1 <= len(request[key]) <= 256 for key in ("request_id", "cancellation_id")) or request["model_digest"] != self.descriptor["model_digest"]:
            raise ValueError("embedding request identity mismatch")
        if request.get("context_length") != self.settings["max_tokens"] or not isinstance(request["prompt"], dict) or set(request["prompt"]) != {"input"}:
            raise ValueError("embedding request context mismatch")
        with self.active_lock:
            if self.active is not None or request["request_id"] in self.seen or len(self.seen) >= 10000:
                raise ValueError("embedding request already active or used")
            self.seen.add(request["request_id"])
            stop, options = threading.Event(), self.ort.RunOptions()
            self.active = (request["cancellation_id"], stop, options)
            self.thread = threading.Thread(target=self.generate, args=(request, stop, options), daemon=True)
            self.thread.start()

    def run(self):
        self.send("ready", descriptor=self.descriptor, resources=self.resources())
        try:
            while True:
                raw = sys.stdin.buffer.readline(MAX_FRAME + 1)
                if not raw:
                    return
                if len(raw) > MAX_FRAME or not raw.endswith(b"\n"):
                    raise ValueError("embedding control frame exceeds bounds")
                self.dispatch(strict_json(raw))
        finally:
            with self.active_lock:
                if self.active is not None:
                    self.active[1].set()
                    self.active[2].terminate = True
            if self.thread is not None:
                self.thread.join(timeout=2)
                if self.thread.is_alive():
                    os._exit(1)


def main():
    parser = argparse.ArgumentParser()
    for name in ("provider", "backend", "model", "descriptor", "dependencies", "runtime-root", "artifacts"):
        parser.add_argument("--" + name, required=True)
    parser.add_argument("--threads", type=int, default=1)
    args = parser.parse_args()
    if not 1 <= args.threads <= 256:
        raise ValueError("invalid embedding CPU thread count")
    constrain_python_paths(args.runtime_root)
    args.dependencies = decode_argument(args.dependencies)
    args.artifacts = decode_argument(args.artifacts)
    descriptor = decode_argument(args.descriptor)
    Worker(args, descriptor).run()


if __name__ == "__main__":
    try:
        main()
    except Exception:
        # Never put model content, paths or dependency diagnostics on stdout.
        sys.exit(1)
