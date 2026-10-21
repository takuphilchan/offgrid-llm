#!/usr/bin/env python3
"""Pinned, offline, text-only MLX/Transformers worker (protocol provider-v1).

The service supplies an absolute local model directory and a checked descriptor.
This process has no package-install, network, credential, or desktop commands.
It must still be launched inside a service-managed restricted environment.
"""

from __future__ import annotations

import argparse
import base64
import hashlib
from importlib import metadata
import json
import os
from pathlib import Path
import platform
import queue
import re
import sys
import threading
from typing import Any


PROTOCOL = "provider-v1"
os.environ["HF_HUB_OFFLINE"] = "1"
os.environ["TRANSFORMERS_OFFLINE"] = "1"


def strict_json_loads(raw: str) -> Any:
    """Reject ambiguous control JSON before Python can collapse duplicate keys."""
    def unique_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
        result: dict[str, Any] = {}
        for key, value in pairs:
            if key in result:
                raise ValueError("repeated provider control key")
            result[key] = value
        return result

    def invalid_constant(value: str) -> None:
        raise ValueError("non-finite provider control number: " + value)

    result = json.loads(raw, object_pairs_hook=unique_object, parse_constant=invalid_constant)
    # json.loads permits lone escaped surrogates that cannot represent valid
    # UTF-8 text. The Go side rejects these before dispatch as well.
    json.dumps(result, ensure_ascii=False).encode("utf-8")
    return result


def validate_control_message(message: Any) -> tuple[str, dict[str, Any] | None]:
    """Keep the worker protocol to its reviewed operations and fields."""
    if not isinstance(message, dict):
        raise ValueError("provider control message must be an object")
    operation = message.get("operation")
    if operation == "health":
        if set(message) != {"operation"}:
            raise ValueError("unexpected health control fields")
        return operation, None
    if operation == "cancel":
        if (set(message) != {"operation", "cancellation_id"}
                or not isinstance(message.get("cancellation_id"), str)
                or not message["cancellation_id"]):
            raise ValueError("invalid cancellation control fields")
        return operation, None
    if operation == "generate":
        if set(message) != {"operation", "request"} or not isinstance(message.get("request"), dict):
            raise ValueError("invalid generation control fields")
        request = message["request"]
        if set(request) - {"request_id", "cancellation_id", "model_digest", "prompt", "tools", "context_length"}:
            raise ValueError("unreviewed generation request field")
        if not {"request_id", "cancellation_id", "model_digest", "prompt"} <= set(request):
            raise ValueError("incomplete generation request")
        return operation, request
    raise ValueError("unsupported worker operation")


def verify_template_identity(template: Any, descriptor: dict[str, Any]) -> None:
    """The loaded tokenizer must use the reviewed, evidence-bound template."""
    expected = descriptor.get("template_digest")
    if not isinstance(expected, str) or len(expected) != 64 or not isinstance(template, str) or not template:
        raise ValueError("qualified native text requires one pinned tokenizer chat template")
    actual = hashlib.sha256(template.encode("utf-8")).hexdigest()
    if actual != expected:
        raise ValueError("loaded tokenizer chat template differs from the reviewed adapter")


def verify_dependency_versions(provider: str, dependencies: Any) -> None:
    """Reject changed native libraries before importing or loading model code."""
    if provider == "mlx":
        required = {"mlx", "mlx-lm", "transformers"}
    elif provider == "transformers":
        required = {"safetensors", "torch", "transformers"}
    else:
        raise ValueError("unsupported native worker provider")
    if not isinstance(dependencies, dict) or not required <= set(dependencies):
        raise ValueError("native worker dependency pins are incomplete")
    for name, expected in dependencies.items():
        if (not isinstance(name, str) or not re.fullmatch(r"[a-z0-9][a-z0-9.-]*", name)
                or not isinstance(expected, str) or not re.fullmatch(r"[0-9][0-9A-Za-z.!+_-]*", expected)):
            raise ValueError("native worker dependency pin is invalid")
        if metadata.version(name) != expected:
            raise ValueError("native worker dependency version differs from reviewed runtime")


def emit(event: dict[str, Any], lock: threading.Lock) -> None:
    with lock:
        sys.stdout.write(json.dumps(event, ensure_ascii=False, separators=(",", ":")) + "\n")
        sys.stdout.flush()


class Worker:
    def __init__(self, args: argparse.Namespace, descriptor: dict[str, Any]) -> None:
        self.args = args
        self.descriptor = descriptor
        self.lock = threading.Lock()
        self.active_lock = threading.Lock()
        self.active: tuple[str, str, threading.Event] | None = None
        self.poisoned = False
        self.model_path = Path(args.model).resolve(strict=True)
        if not self.model_path.is_dir():
            raise ValueError("model path must be a local directory")
        if descriptor.get("protocol_revision") != PROTOCOL or descriptor.get("provider") != args.provider or descriptor.get("backend") != args.backend:
            raise ValueError("provider descriptor does not match the worker")
        expected_format = "mlx_safetensors" if args.provider == "mlx" else "safetensors"
        if descriptor.get("format") != expected_format or not descriptor.get("model_digest") or not descriptor.get("adapter_revision"):
            raise ValueError("provider model, format, or adapter identity is missing")
        if not list(self.model_path.glob("*.safetensors")):
            raise ValueError("local safetensors weights are missing")
        verify_dependency_versions(args.provider, args.dependencies)
        self._load()
        verify_template_identity(getattr(self.tokenizer, "chat_template", None), descriptor)

    def resources(self) -> dict[str, Any]:
        """Report observed placement; unknown memory is not represented as zero use."""
        backend = self.args.backend
        device = "CPU"
        memory = 0
        limit = 0
        if self.args.provider == "mlx":
            import mlx.core as mx
            if not mx.metal.is_available():
                raise RuntimeError("MLX Metal backend is unavailable")
            if mx.default_device() != mx.gpu:
                raise RuntimeError("MLX is not using the admitted Metal device")
            details = mx.metal.device_info()
            if not isinstance(details, dict):
                raise RuntimeError("MLX Metal device identity is unavailable")
            device = details.get("device_name")
            if not isinstance(device, str) or not device.strip():
                raise RuntimeError("MLX Metal device identity is unavailable")
            try:
                memory = int(mx.get_active_memory())
                limit = int(details.get("max_recommended_working_set_size", 0))
            except (AttributeError, RuntimeError, TypeError, ValueError):
                memory = 0
                limit = 0
        elif backend == "cuda":
            device = str(self.torch.cuda.get_device_name())
            memory = int(self.torch.cuda.memory_allocated())
            limit = int(self.torch.cuda.get_device_properties().total_memory)
        elif backend == "mps":
            device = "Apple MPS"
            if hasattr(self.torch.mps, "current_allocated_memory"):
                memory = int(self.torch.mps.current_allocated_memory())
            if hasattr(self.torch.mps, "recommended_max_memory"):
                limit = int(self.torch.mps.recommended_max_memory())
        # Allocator measurements do not include every driver or cached byte.
        # Unknown/zero usage is never presented as a free-memory claim.
        if memory <= 0:
            limit = 0
        return {"backend": backend, "device": device, "memory_bytes": memory,
                "memory_limit_bytes": max(0, limit), "memory_measured": memory > 0}

    def _load(self) -> None:
        if self.args.provider == "mlx":
            if platform.system() != "Darwin" or platform.machine() != "arm64" or self.args.backend != "metal":
                raise RuntimeError("MLX text worker requires qualified Apple Silicon Metal")
            import mlx.core as mx
            if not mx.metal.is_available():
                raise RuntimeError("MLX Metal backend is unavailable")
            # MLX-LM creates its generation stream at import time. Select the
            # admitted Metal device before importing it or loading weights.
            mx.set_default_device(mx.gpu)
            if mx.default_device() != mx.gpu:
                raise RuntimeError("MLX did not select the admitted Metal device")
            from mlx_lm import load, stream_generate

            # Pin both model-class loading and tokenizer loading to built-in
            # implementations. The library currently defaults the former to
            # false, but this boundary must not depend on that default.
            self.model, self.tokenizer = load(
                str(self.model_path), tokenizer_config={"trust_remote_code": False}, trust_remote_code=False
            )
            self.stream_generate = stream_generate
            return

        import torch
        from transformers import AutoModelForCausalLM, AutoTokenizer

        if self.args.backend not in {"cpu", "cuda", "mps"}:
            raise RuntimeError("Transformers backend is not supported by this worker")
        if self.args.backend == "cuda" and not torch.cuda.is_available():
            raise RuntimeError("CUDA backend is unavailable")
        if self.args.backend == "mps" and not torch.backends.mps.is_available():
            raise RuntimeError("MPS backend is unavailable")
        self.torch = torch
        self.tokenizer = AutoTokenizer.from_pretrained(str(self.model_path), local_files_only=True, trust_remote_code=False)
        self.model = AutoModelForCausalLM.from_pretrained(
            str(self.model_path), local_files_only=True, trust_remote_code=False,
            use_safetensors=True, weights_only=True,
        )
        self.model.to(self.args.backend)
        self.model.eval()

    def send(self, kind: str, request_id: str = "", sequence: int = 0, **fields: Any) -> None:
        emit({"protocol_revision": PROTOCOL, "type": kind, "request_id": request_id, "sequence": sequence, **fields}, self.lock)

    def fail(self, request_id: str, code: str, detail: str, sequence: int = 1) -> None:
        self.send("error", request_id, sequence, error={"code": code, "message": detail, "operational": True})

    def run(self) -> None:
        self.send("ready", descriptor=self.descriptor, resources=self.resources())
        for line in sys.stdin:
            try:
                message = strict_json_loads(line)
                operation, request = validate_control_message(message)
                if operation == "health":
                    with self.active_lock:
                        poisoned = self.poisoned
                    if poisoned:
                        self.fail("", "worker_unhealthy", "provider generation did not stop cleanly", 0)
                    else:
                        self.send("ready", resources=self.resources())
                elif operation == "cancel":
                    with self.active_lock:
                        if self.active is not None and self.active[1] == message.get("cancellation_id"):
                            self.active[2].set()
                elif operation == "generate":
                    if not isinstance(request, dict) or request.get("model_digest") != self.descriptor["model_digest"]:
                        raise ValueError("request model identity does not match loaded model")
                    request_id, cancellation_id = request.get("request_id"), request.get("cancellation_id")
                    if not isinstance(request_id, str) or not request_id or not isinstance(cancellation_id, str) or not cancellation_id:
                        raise ValueError("request and cancellation identities are required")
                    with self.active_lock:
                        if self.poisoned:
                            self.fail(request_id, "worker_unhealthy", "provider generation did not stop cleanly")
                            continue
                        if self.active is not None:
                            self.fail(request_id, "busy", "provider has an active generation")
                            continue
                        cancelled = threading.Event()
                        self.active = (request_id, cancellation_id, cancelled)
                    threading.Thread(target=self.generate, args=(request, cancelled), daemon=True).start()
                else:
                    raise ValueError("unsupported worker operation")
            except (ValueError, TypeError, json.JSONDecodeError):
                # Invalid control messages never become executable commands.
                self.fail("", "invalid_message", "invalid provider request")

    def _prompt(self, request: dict[str, Any]) -> tuple[str, int, list[int]]:
        if request.get("tools"):
            raise ValueError("tool decisions are not qualified for this worker")
        prompt = request.get("prompt")
        if (not isinstance(prompt, dict) or set(prompt) - {"messages", "max_tokens"}
                or not isinstance(prompt.get("messages"), list) or not prompt["messages"]):
            raise ValueError("text chat messages are required")
        messages = prompt["messages"]
        if any(not isinstance(m, dict) or set(m) != {"role", "content"}
               or m.get("role") not in {"system", "user", "assistant"}
               or not isinstance(m.get("content"), str) for m in messages):
            raise ValueError("multimodal and tool-role messages are not qualified")
        if not hasattr(self.tokenizer, "apply_chat_template"):
            raise ValueError("model tokenizer has no qualified chat template")
        explicit_max_tokens = prompt.get("max_tokens")
        if explicit_max_tokens is not None and (type(explicit_max_tokens) is not int or not 1 <= explicit_max_tokens <= 8192):
            raise ValueError("max_tokens is outside the worker limit")
        text = self.tokenizer.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
        if not isinstance(text, str):
            raise ValueError("adapter did not produce a text prompt")
        context_length = self.descriptor.get("context_length")
        requested_context = request.get("context_length", context_length)
        if type(context_length) is not int or context_length < 1 or type(requested_context) is not int or not 1 <= requested_context <= context_length:
            raise ValueError("request context does not match the qualified model")
        # Count after applying the reviewed template: counting raw message text
        # misses role markers and generation delimiters. Fail before generation
        # rather than allowing a native runtime to truncate the user's context.
        encoded = self.tokenizer.encode(text, add_special_tokens=False)
        if not isinstance(encoded, list) or any(type(token) is not int or token < 0 for token in encoded):
            raise ValueError("formatted prompt contains invalid token IDs")
        available = requested_context - len(encoded)
        if available < 1:
            raise ValueError("formatted prompt leaves no qualified response context")
        max_tokens = explicit_max_tokens if explicit_max_tokens is not None else min(512, available)
        if max_tokens > available:
            raise ValueError("formatted prompt and response exceed the qualified context")
        return text, max_tokens, encoded

    def generate(self, request: dict[str, Any], cancelled: threading.Event) -> None:
        request_id = request["request_id"]
        sequence = 0
        try:
            _prompt, max_tokens, prompt_ids = self._prompt(request)
            completion_tokens = 0
            mlx_finish_reason = None
            if cancelled.is_set():
                # A queued Stop must not start native generation merely to
                # discover cancellation at the first yielded token.
                pass
            elif self.args.provider == "mlx":
                # MLX-LM re-tokenizes strings and may infer extra BOS tokens.
                # Pass the exact reviewed-template token IDs admitted above.
                iterator = self.stream_generate(self.model, self.tokenizer, prompt_ids, max_tokens=max_tokens)
                try:
                    for part in iterator:
                        if cancelled.is_set():
                            break
                        reported_tokens = part.generation_tokens
                        if type(reported_tokens) is not int or not completion_tokens <= reported_tokens <= max_tokens:
                            raise RuntimeError("MLX returned an invalid generation count")
                        reported_prompt = getattr(part, "prompt_tokens", len(prompt_ids))
                        if type(reported_prompt) is not int or reported_prompt != len(prompt_ids):
                            raise RuntimeError("MLX changed the admitted prompt token count")
                        reported_finish = getattr(part, "finish_reason", None)
                        if reported_finish not in (None, "stop", "length") or mlx_finish_reason is not None:
                            raise RuntimeError("MLX returned an invalid terminal stream")
                        completion_tokens = reported_tokens
                        if part.text:
                            sequence += 1
                            self.send("delta", request_id, sequence, text=part.text)
                        if reported_finish is not None:
                            mlx_finish_reason = reported_finish
                finally:
                    close = getattr(iterator, "close", None)
                    if close is not None:
                        try:
                            close()
                        except Exception as exc:
                            # Reusing this process after an unconfirmed native
                            # stream teardown could overlap generations.
                            with self.active_lock:
                                self.poisoned = True
                            raise RuntimeError("MLX stream did not close cleanly") from exc
            else:
                sequence, completion_tokens = self._generate_transformers(prompt_ids, max_tokens, cancelled, request_id, sequence)
            if type(completion_tokens) is not int or not 0 <= completion_tokens <= max_tokens:
                raise RuntimeError("native provider returned an invalid generation count")
            if self.args.provider == "mlx" and not cancelled.is_set() and mlx_finish_reason is None:
                raise RuntimeError("MLX stream ended without a terminal reason")
            if cancelled.is_set():
                finish_reason = "cancelled"
            elif mlx_finish_reason is not None:
                finish_reason = mlx_finish_reason
            elif completion_tokens >= max_tokens:
                finish_reason = "length"
            else:
                finish_reason = "stop"
            sequence += 1
            self.send("usage", request_id, sequence,
                      usage={"prompt_tokens": len(prompt_ids), "completion_tokens": completion_tokens,
                             "total_tokens": len(prompt_ids) + completion_tokens})
            sequence += 1
            self.send("done", request_id, sequence,
                      finish_reason=finish_reason,
                      resources=self.resources())
        except Exception:
            self.fail(request_id, "generation_failed", "local provider generation failed", sequence + 1)
        finally:
            with self.active_lock:
                if self.active is not None and self.active[0] == request_id:
                    self.active = None

    def _generate_transformers(self, prompt_ids: list[int], max_tokens: int, cancelled: threading.Event, request_id: str, sequence: int) -> tuple[int, int]:
        from transformers import StoppingCriteria, StoppingCriteriaList, TextIteratorStreamer

        torch = self.torch

        class StopWhenCancelled(StoppingCriteria):
            def __call__(self, input_ids: Any, scores: Any, **kwargs: Any) -> Any:
                return torch.tensor([cancelled.is_set()], device=input_ids.device, dtype=torch.bool)

        # Use the precise sequence admitted by _prompt. A second tokenizer
        # invocation could add or change control tokens after context admission.
        input_ids = torch.tensor([prompt_ids], dtype=torch.long, device=self.args.backend)
        attention_mask = torch.ones_like(input_ids)
        streamer = TextIteratorStreamer(self.tokenizer, skip_prompt=True, skip_special_tokens=True, timeout=0.25)
        errors: list[Exception] = []
        generated: list[Any] = []

        def produce() -> None:
            try:
                with torch.inference_mode():
                    generated.append(self.model.generate(input_ids=input_ids, attention_mask=attention_mask, max_new_tokens=max_tokens,
                                                         streamer=streamer, stopping_criteria=StoppingCriteriaList([StopWhenCancelled()])))
            except Exception as exc:
                errors.append(exc)

        producer = threading.Thread(target=produce, daemon=True)
        producer.start()
        while True:
            try:
                part = next(streamer)
            except queue.Empty:
                if not producer.is_alive():
                    break
                continue
            except StopIteration:
                break
            if part and not cancelled.is_set():
                sequence += 1
                self.send("delta", request_id, sequence, text=part)
        self._wait_for_transformers_producer(producer, cancelled)
        if errors:
            raise RuntimeError("Transformers generation did not finish cleanly")
        if len(generated) != 1:
            raise RuntimeError("Transformers generation returned no token sequence")
        output = getattr(generated[0], "sequences", generated[0])
        completion_tokens = int(output.shape[-1]) - len(prompt_ids)
        if completion_tokens < 0 or completion_tokens > max_tokens:
            raise RuntimeError("Transformers generation returned an invalid token count")
        return sequence, completion_tokens

    def _wait_for_transformers_producer(self, producer: threading.Thread, cancelled: threading.Event) -> None:
        producer.join(timeout=2)
        if producer.is_alive():
            # A native generation may remain active after its stream fails.
            # Never admit another request on this model process; the Go
            # supervisor observes the failed health check and replaces it.
            cancelled.set()
            with self.active_lock:
                self.poisoned = True
            raise RuntimeError("Transformers generation did not stop cleanly")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--provider", choices=("mlx", "transformers"), required=True)
    parser.add_argument("--model", required=True)
    parser.add_argument("--backend", required=True)
    parser.add_argument("--descriptor", required=True)
    parser.add_argument("--dependencies", required=True)
    args = parser.parse_args()
    try:
        descriptor = strict_json_loads(base64.urlsafe_b64decode(args.descriptor + "=" * (-len(args.descriptor) % 4)).decode("utf-8"))
        args.dependencies = strict_json_loads(base64.urlsafe_b64decode(args.dependencies + "=" * (-len(args.dependencies) % 4)).decode("utf-8"))
        if not isinstance(descriptor, dict):
            raise ValueError("descriptor must be an object")
        Worker(args, descriptor).run()
        return 0
    except Exception:
        emit({"protocol_revision": PROTOCOL, "type": "error", "request_id": "", "sequence": 0,
              "error": {"code": "startup_failed", "message": "local provider startup failed", "operational": True}}, threading.Lock())
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
