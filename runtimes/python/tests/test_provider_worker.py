"""Protocol-only worker checks; these do not qualify an MLX/PyTorch model."""

import argparse
import hashlib
import io
import json
from pathlib import Path
import runpy
import sys
import threading
from types import ModuleType, SimpleNamespace

import pytest


worker_module = runpy.run_path(str(Path(__file__).resolve().parents[1] / "provider_worker.py"))
Worker = worker_module["Worker"]
verify_template_identity = worker_module["verify_template_identity"]
verify_dependency_versions = worker_module["verify_dependency_versions"]
strict_json_loads = worker_module["strict_json_loads"]
validate_control_message = worker_module["validate_control_message"]


def test_worker_requires_exact_installed_dependency_versions(monkeypatch):
    installed = {"mlx": "1.2.3", "mlx-lm": "0.28.0", "transformers": "5.7.0"}
    monkeypatch.setattr(worker_module["metadata"], "version", lambda name: installed[name])
    verify_dependency_versions("mlx", installed)
    with pytest.raises(ValueError, match="differs"):
        verify_dependency_versions("mlx", {**installed, "mlx-lm": "0.27.0"})
    with pytest.raises(ValueError, match="incomplete"):
        verify_dependency_versions("mlx", {"mlx": "1.2.3"})
    with pytest.raises(ValueError, match="invalid"):
        verify_dependency_versions("mlx", {**installed, "../../override": "1.0"})
    with pytest.raises(ValueError, match="unsupported"):
        verify_dependency_versions("unknown", installed)


def test_worker_rejects_ambiguous_control_json():
    assert strict_json_loads('{"operation":"health","request":{"id":"one"}}')["operation"] == "health"
    for raw in ('{"operation":"health","operation":"generate"}',
                '{"request":{"id":"one","id":"two"}}',
                '{"value":NaN}', '{"value":Infinity}',
                '{"value":"\\ud800"}'):
        with pytest.raises((ValueError, UnicodeEncodeError)):
            strict_json_loads(raw)


def test_worker_does_not_dispatch_ambiguous_control_message(monkeypatch):
    worker, events = fake_worker()
    worker.stream_generate = lambda *_args, **_kwargs: pytest.fail("ambiguous control dispatched")
    monkeypatch.setattr(sys, "stdin", io.StringIO('{"operation":"health","operation":"generate"}\n'))
    worker.run()
    assert [event[0] for event in events] == ["ready", "error"]
    assert events[-1][3]["error"]["code"] == "invalid_message"


def test_worker_control_messages_have_closed_operation_schemas():
    assert validate_control_message({"operation": "health"}) == ("health", None)
    assert validate_control_message({"operation": "cancel", "cancellation_id": "one"}) == ("cancel", None)
    valid = {"request_id": "one", "cancellation_id": "one", "model_digest": "sha256:test", "prompt": {"messages": []}}
    assert validate_control_message({"operation": "generate", "request": valid}) == ("generate", valid)
    for message in ([], {"operation": "health", "request": valid},
                    {"operation": "cancel", "cancellation_id": ""},
                    {"operation": "generate", "request": {**valid, "command": "run"}},
                    {"operation": "generate", "request": {"prompt": {}}},
                    {"operation": "install"}):
        with pytest.raises(ValueError):
            validate_control_message(message)


def test_loaded_tokenizer_template_must_match_reviewed_identity():
    expected = hashlib.sha256("reviewed template".encode("utf-8")).hexdigest()
    verify_template_identity("reviewed template", {"template_digest": expected})
    with pytest.raises(ValueError):
        verify_template_identity("package changed template", {"template_digest": expected})
    with pytest.raises(ValueError):
        verify_template_identity(None, {"template_digest": expected})
    with pytest.raises(ValueError):
        verify_template_identity("reviewed template", {})


def test_worker_resource_report_distinguishes_observed_device_and_unknown_memory():
    worker = Worker.__new__(Worker)
    worker.args = argparse.Namespace(provider="transformers", backend="cuda")
    worker.torch = SimpleNamespace(cuda=SimpleNamespace(get_device_name=lambda: "Test GPU", memory_allocated=lambda: 4096,
                                                       get_device_properties=lambda: SimpleNamespace(total_memory=8192)))
    assert worker.resources() == {"backend": "cuda", "device": "Test GPU", "memory_bytes": 4096,
                                  "memory_limit_bytes": 8192, "memory_measured": True}
    worker.args.backend = "cpu"
    assert worker.resources() == {"backend": "cpu", "device": "CPU", "memory_bytes": 0,
                                  "memory_limit_bytes": 0, "memory_measured": False}


def test_mlx_and_mps_reports_use_backend_memory_interfaces(monkeypatch):
    package = ModuleType("mlx")
    core = ModuleType("mlx.core")
    core.get_active_memory = lambda: 2048
    core.gpu = object()
    core.default_device = lambda: core.gpu
    core.metal = SimpleNamespace(is_available=lambda: True, device_info=lambda: {
        "device_name": "Apple M3", "max_recommended_working_set_size": 8192})
    package.core = core
    monkeypatch.setitem(sys.modules, "mlx", package)
    monkeypatch.setitem(sys.modules, "mlx.core", core)
    worker = Worker.__new__(Worker)
    worker.args = argparse.Namespace(provider="mlx", backend="metal")
    assert worker.resources() == {"backend": "metal", "device": "Apple M3", "memory_bytes": 2048,
                                  "memory_limit_bytes": 8192, "memory_measured": True}
    worker.args = argparse.Namespace(provider="transformers", backend="mps")
    worker.torch = SimpleNamespace(mps=SimpleNamespace(current_allocated_memory=lambda: 4096,
                                                      recommended_max_memory=lambda: 16384))
    assert worker.resources() == {"backend": "mps", "device": "Apple MPS", "memory_bytes": 4096,
                                  "memory_limit_bytes": 16384, "memory_measured": True}


def test_mlx_worker_requires_available_metal_before_loading(monkeypatch, tmp_path):
    calls = []
    package = ModuleType("mlx")
    core = ModuleType("mlx.core")
    available = [False]
    core.metal = SimpleNamespace(is_available=lambda: available[0])
    core.gpu = object()
    current_device = [object()]
    def select_device(device):
        calls.append(("device", device))
        current_device[0] = device
    core.set_default_device = select_device
    core.default_device = lambda: current_device[0]
    package.core = core
    monkeypatch.setitem(sys.modules, "mlx", package)
    monkeypatch.setitem(sys.modules, "mlx.core", core)
    monkeypatch.setitem(sys.modules, "mlx_lm", SimpleNamespace(
        load=lambda *_args, **kwargs: calls.append(("load", kwargs)) or (object(), Tokenizer()),
        stream_generate=lambda *_args, **_kwargs: iter(()),
    ))
    monkeypatch.setattr(worker_module["platform"], "system", lambda: "Darwin")
    monkeypatch.setattr(worker_module["platform"], "machine", lambda: "arm64")
    worker = Worker.__new__(Worker)
    worker.args = argparse.Namespace(provider="mlx", backend="metal")
    worker.model_path = tmp_path
    with pytest.raises(RuntimeError, match="unavailable"):
        worker._load()
    assert calls == []
    available[0] = True
    worker._load()
    assert calls == [("device", core.gpu),
                     ("load", {"tokenizer_config": {"trust_remote_code": False}, "trust_remote_code": False})]
    available[0] = False
    with pytest.raises(RuntimeError, match="unavailable"):
        worker.resources()


def test_mlx_handshake_rejects_missing_device_identity(monkeypatch):
    package = ModuleType("mlx")
    core = ModuleType("mlx.core")
    core.gpu = object()
    core.default_device = lambda: core.gpu
    core.metal = SimpleNamespace(is_available=lambda: True, device_info=lambda: {})
    package.core = core
    monkeypatch.setitem(sys.modules, "mlx", package)
    monkeypatch.setitem(sys.modules, "mlx.core", core)
    worker = Worker.__new__(Worker)
    worker.args = argparse.Namespace(provider="mlx", backend="metal")
    with pytest.raises(RuntimeError, match="device identity"):
        worker.resources()


def test_mlx_handshake_rejects_cpu_default_device(monkeypatch):
    package = ModuleType("mlx")
    core = ModuleType("mlx.core")
    core.gpu = object()
    core.default_device = lambda: object()
    core.metal = SimpleNamespace(is_available=lambda: True,
                                 device_info=lambda: {"device_name": "Apple M3"})
    package.core = core
    monkeypatch.setitem(sys.modules, "mlx", package)
    monkeypatch.setitem(sys.modules, "mlx.core", core)
    worker = Worker.__new__(Worker)
    worker.args = argparse.Namespace(provider="mlx", backend="metal")
    with pytest.raises(RuntimeError, match="not using"):
        worker.resources()


def test_transformers_load_requires_offline_safetensors_without_remote_code(monkeypatch, tmp_path):
    loaded = []

    class FakeModel:
        def to(self, backend):
            assert backend == "cpu"

        def eval(self):
            pass

    def from_pretrained(path, **kwargs):
        loaded.append((path, kwargs))
        return FakeModel()

    monkeypatch.setitem(sys.modules, "torch", ModuleType("torch"))
    monkeypatch.setitem(sys.modules, "transformers", SimpleNamespace(
        AutoTokenizer=SimpleNamespace(from_pretrained=from_pretrained),
        AutoModelForCausalLM=SimpleNamespace(from_pretrained=from_pretrained),
    ))
    worker = Worker.__new__(Worker)
    worker.args = argparse.Namespace(provider="transformers", backend="cpu")
    worker.model_path = tmp_path
    worker._load()
    assert len(loaded) == 2
    assert all(path == str(tmp_path) and kwargs["local_files_only"] is True and kwargs["trust_remote_code"] is False
               for path, kwargs in loaded)
    assert loaded[1][1]["use_safetensors"] is True
    assert loaded[1][1]["weights_only"] is True


class Tokenizer:
    def apply_chat_template(self, messages, tokenize=False, add_generation_prompt=False):
        assert tokenize is False and add_generation_prompt is True
        return "\n".join(message["content"] for message in messages)

    def encode(self, text, add_special_tokens=True):
        assert add_special_tokens is False
        return list(range(len(text.split())))


def fake_worker():
    worker = Worker.__new__(Worker)
    worker.args = argparse.Namespace(provider="mlx", backend="metal")
    worker.tokenizer = Tokenizer()
    worker.descriptor = {"context_length": 8192}
    worker.model = object()
    worker.resources = lambda: {"backend": "metal", "device": "fixture", "memory_bytes": 0,
                                "memory_limit_bytes": 0, "memory_measured": False}
    worker.active_lock = threading.Lock()
    worker.active = ("request-1", "cancel-1", threading.Event())
    def stream_generate(_model, _tokenizer, prompt_ids, **kwargs):
        assert prompt_ids == [0]
        assert kwargs["max_tokens"] > 0
        return iter((SimpleNamespace(text="Hello", generation_tokens=1),
                     SimpleNamespace(text=" world", generation_tokens=2,
                                     finish_reason="length" if kwargs["max_tokens"] == 2 else "stop")))

    worker.stream_generate = stream_generate
    events = []
    worker.send = lambda kind, request_id="", sequence=0, **fields: events.append((kind, request_id, sequence, fields))
    return worker, events


def test_text_worker_streams_ordered_events_and_terminal_status():
    worker, events = fake_worker()
    worker.generate({"request_id": "request-1", "prompt": {"messages": [{"role": "user", "content": "hello"}], "max_tokens": 4}}, threading.Event())
    assert [(kind, sequence) for kind, _, sequence, _ in events] == [("delta", 1), ("delta", 2), ("usage", 3), ("done", 4)]
    assert events[-2][3]["usage"] == {"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3}
    assert events[-1][3]["finish_reason"] == "stop"
    assert events[-1][3]["resources"]["backend"] == "metal"
    assert worker.active is None


def test_text_worker_marks_token_limit_without_claiming_normal_stop():
    worker, events = fake_worker()
    worker.generate({"request_id": "request-1", "prompt": {"messages": [{"role": "user", "content": "hello"}], "max_tokens": 2}}, threading.Event())
    assert events[-1][3]["finish_reason"] == "length"


def test_mlx_terminal_reason_wins_when_eos_arrives_at_token_limit():
    worker, events = fake_worker()
    worker.stream_generate = lambda *_args, **_kwargs: iter((SimpleNamespace(
        text="done", generation_tokens=2, prompt_tokens=1, finish_reason="stop"),))
    worker.generate({"request_id": "request-1", "prompt": {"messages": [{"role": "user", "content": "hello"}], "max_tokens": 2}}, threading.Event())
    assert events[-1][3]["finish_reason"] == "stop"


@pytest.mark.parametrize("parts", [
    (SimpleNamespace(text="one", generation_tokens=2), SimpleNamespace(text="two", generation_tokens=1, finish_reason="stop")),
    (SimpleNamespace(text="one", generation_tokens=1, prompt_tokens=2, finish_reason="stop"),),
    (SimpleNamespace(text="one", generation_tokens=1),),
    (SimpleNamespace(text="one", generation_tokens=1, finish_reason="stop"), SimpleNamespace(text="two", generation_tokens=2)),
])
def test_mlx_rejects_inconsistent_or_unterminated_stream(parts):
    worker, events = fake_worker()
    worker.stream_generate = lambda *_args, **_kwargs: iter(parts)
    worker.generate({"request_id": "request-1", "prompt": {"messages": [{"role": "user", "content": "hello"}], "max_tokens": 4}}, threading.Event())
    assert events[-1][0] == "error"
    assert not any(event[0] == "done" for event in events)


@pytest.mark.parametrize("prompt", [
    {"messages": [{"role": "tool", "content": "untrusted"}]},
    {"messages": [{"role": "user", "content": [{"type": "image"}]}]},
    {"messages": [], "tools": [{"name": "desktop"}]},
    {"messages": [], "max_tokens": 100000},
])
def test_text_worker_rejects_unqualified_tools_and_multimodal_input(prompt):
    worker, _ = fake_worker()
    with pytest.raises(ValueError):
        worker._prompt({"prompt": prompt})


@pytest.mark.parametrize("prompt", [
    {"messages": []},
    {"messages": [{"role": "user", "content": "hello", "tool_calls": []}]},
    {"messages": [{"role": "user", "content": "hello"}], "temperature": 0.7},
    {"messages": [{"role": "user", "content": "hello"}], "tools": []},
])
def test_text_worker_rejects_unreviewed_prompt_fields(prompt):
    worker, _ = fake_worker()
    with pytest.raises(ValueError):
        worker._prompt({"prompt": prompt})


def test_text_worker_counts_formatted_prompt_before_generation():
    worker, _ = fake_worker()
    message = {"role": "user", "content": "one two three four"}
    with pytest.raises(ValueError, match="formatted prompt"):
        worker._prompt({"prompt": {"messages": [message], "max_tokens": 5}, "context_length": 8})
    assert worker._prompt({"prompt": {"messages": [message], "max_tokens": 5}, "context_length": 9}) == (message["content"], 5, [0, 1, 2, 3])


def test_text_worker_uses_remaining_context_for_implicit_output_limit():
    worker, _ = fake_worker()
    message = {"role": "user", "content": "one two three four"}
    assert worker._prompt({"prompt": {"messages": [message]}, "context_length": 7}) == (message["content"], 3, [0, 1, 2, 3])
    with pytest.raises(ValueError, match="no qualified response context"):
        worker._prompt({"prompt": {"messages": [message]}, "context_length": 4})


def test_text_worker_cannot_raise_qualified_context_by_request():
    worker, _ = fake_worker()
    prompt = {"messages": [{"role": "user", "content": "hello"}]}
    with pytest.raises(ValueError, match="qualified model"):
        worker._prompt({"prompt": prompt, "context_length": 8193})
    with pytest.raises(ValueError, match="qualified model"):
        worker._prompt({"prompt": prompt, "context_length": True})
    with pytest.raises(ValueError, match="worker limit"):
        worker._prompt({"prompt": {**prompt, "max_tokens": True}})


def test_cancelled_worker_reports_cancelled_and_does_not_emit_more_text():
    worker, events = fake_worker()
    worker.stream_generate = lambda *_args, **_kwargs: pytest.fail("cancelled request started MLX generation")
    cancelled = threading.Event()
    cancelled.set()
    worker.generate({"request_id": "request-1", "prompt": {"messages": [{"role": "user", "content": "hello"}]}}, cancelled)
    assert [(kind, sequence) for kind, _, sequence, _ in events] == [("usage", 1), ("done", 2)]
    assert events[-1][3]["finish_reason"] == "cancelled"


def test_mlx_cancel_closes_stream_before_worker_accepts_another_request():
    worker, events = fake_worker()
    cancelled = threading.Event()
    closed = []

    class Stream:
        def __iter__(self):
            return self

        def __next__(self):
            return SimpleNamespace(text="next", generation_tokens=1)

        def close(self):
            closed.append(True)

    worker.stream_generate = lambda *_args, **_kwargs: Stream()
    send = worker.send

    def cancel_after_delta(*args, **kwargs):
        send(*args, **kwargs)
        if args[0] == "delta":
            cancelled.set()

    worker.send = cancel_after_delta
    worker.generate({"request_id": "request-1", "prompt": {"messages": [{"role": "user", "content": "hello"}]}}, cancelled)
    assert closed == [True]
    assert [event[0] for event in events] == ["delta", "usage", "done"]
    assert events[-1][3]["finish_reason"] == "cancelled"
    assert worker.active is None


def test_mlx_failed_stream_close_poisons_worker_without_completion():
    worker, events = fake_worker()
    worker.poisoned = False

    class Stream:
        def __iter__(self):
            return self

        def __next__(self):
            if hasattr(self, "sent"):
                raise StopIteration
            self.sent = True
            return SimpleNamespace(text="done", generation_tokens=1, finish_reason="stop")

        def close(self):
            raise RuntimeError("native cleanup failed")

    worker.stream_generate = lambda *_args, **_kwargs: Stream()
    worker.generate({"request_id": "request-1", "prompt": {"messages": [{"role": "user", "content": "hello"}]}}, threading.Event())
    assert [event[0] for event in events] == ["delta", "error"]
    assert worker.poisoned is True
    assert worker.active is None


def test_worker_rejects_invalid_native_generation_count_without_completion():
    worker, events = fake_worker()
    worker.stream_generate = lambda *_args, **_kwargs: iter((SimpleNamespace(text="bad", generation_tokens=9000),))
    worker.generate({"request_id": "request-1", "prompt": {"messages": [{"role": "user", "content": "hello"}], "max_tokens": 4}}, threading.Event())
    assert [event[0] for event in events] == ["error"]
    assert not any(event[0] == "done" for event in events)


def test_transformers_generation_uses_the_counted_template_tokens(monkeypatch):
    class InferenceMode:
        def __enter__(self):
            return self

        def __exit__(self, *_):
            return False

    class FakeModel:
        def generate(self, **kwargs):
            assert kwargs["max_new_tokens"] == 4
            assert kwargs["input_ids"] == [[3, 4]]
            assert kwargs["attention_mask"] == [[1, 1]]
            return SimpleNamespace(shape=(1, 4))

    class FakeStreamer:
        def __init__(self, *_args, **_kwargs):
            pass

        def __iter__(self):
            return self

        def __next__(self):
            raise StopIteration

    class StoppingCriteria:
        pass

    monkeypatch.setitem(sys.modules, "transformers", SimpleNamespace(
        StoppingCriteria=StoppingCriteria,
        StoppingCriteriaList=list,
        TextIteratorStreamer=FakeStreamer,
    ))
    worker = Worker.__new__(Worker)
    worker.args = argparse.Namespace(provider="transformers", backend="cpu")
    worker.tokenizer = Tokenizer()
    worker.model = FakeModel()
    worker.torch = SimpleNamespace(inference_mode=InferenceMode, long="long",
                                    tensor=lambda values, **kwargs: values,
                                    ones_like=lambda values: [[1 for _ in row] for row in values])
    assert worker._generate_transformers([3, 4], 4, threading.Event(), "request-1", 0) == (0, 2)


def test_transformers_generation_rejects_invalid_runtime_token_count(monkeypatch):
    class FakeStreamer:
        def __init__(self, *_args, **_kwargs):
            pass

        def __iter__(self):
            return self

        def __next__(self):
            raise StopIteration

    class FakeModel:
        def generate(self, **_kwargs):
            return SimpleNamespace(shape=(1, 12))

    class InferenceMode:
        def __enter__(self):
            return self

        def __exit__(self, *_):
            return False

    monkeypatch.setitem(sys.modules, "transformers", SimpleNamespace(
        StoppingCriteria=object, StoppingCriteriaList=list, TextIteratorStreamer=FakeStreamer,
    ))
    worker = Worker.__new__(Worker)
    worker.args = argparse.Namespace(provider="transformers", backend="cpu")
    worker.tokenizer = Tokenizer()
    worker.model = FakeModel()
    worker.torch = SimpleNamespace(inference_mode=InferenceMode, long="long", tensor=lambda values, **kwargs: values,
                                    ones_like=lambda values: [[1 for _ in row] for row in values])
    with pytest.raises(RuntimeError, match="invalid token count"):
        worker._generate_transformers([3, 4], 4, threading.Event(), "request-1", 0)


def test_stuck_transformers_producer_poisoned_before_another_request():
    class StuckProducer:
        def join(self, timeout):
            assert timeout == 2

        def is_alive(self):
            return True

    worker = Worker.__new__(Worker)
    worker.active_lock = threading.Lock()
    worker.poisoned = False
    cancelled = threading.Event()
    with pytest.raises(RuntimeError, match="did not stop cleanly"):
        worker._wait_for_transformers_producer(StuckProducer(), cancelled)
    assert cancelled.is_set()
    assert worker.poisoned is True


def test_poisoned_worker_rejects_health_and_generation(monkeypatch):
    worker = Worker.__new__(Worker)
    worker.descriptor = {"model_digest": "sha256:model"}
    worker.active_lock = threading.Lock()
    worker.active = None
    worker.poisoned = True
    worker.resources = lambda: {"backend": "cpu", "device": "fixture", "memory_measured": False}
    events = []
    worker.send = lambda kind, request_id="", sequence=0, **fields: events.append((kind, request_id, sequence, fields))
    controls = [
        {"operation": "health"},
        {"operation": "generate", "request": {"model_digest": "sha256:model", "request_id": "next", "cancellation_id": "next", "prompt": {"messages": [{"role": "user", "content": "hello"}]}}},
    ]
    monkeypatch.setattr(sys, "stdin", io.StringIO("".join(json.dumps(control) + "\n" for control in controls)))
    worker.run()
    assert events[0][0] == "ready"
    assert [(kind, request_id, fields["error"]["code"]) for kind, request_id, _, fields in events[1:]] == [
        ("error", "", "worker_unhealthy"), ("error", "next", "worker_unhealthy"),
    ]
