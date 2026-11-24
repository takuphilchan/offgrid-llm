"""Contract checks only; no ONNX model/native runtime qualification is implied."""
import io
import base64
import hashlib
import json
from pathlib import Path
import runpy
import tempfile
import threading
from types import SimpleNamespace
import unittest
from unittest.mock import patch

module = runpy.run_path(str(Path(__file__).resolve().parents[1] / "onnx_embedding_worker.py"))
Worker = module["Worker"]


def settings():
    return {"schema_version": 1, "family": "bert-symmetric", "inputs": ["input_ids", "attention_mask"],
            "output": "last_hidden_state", "pooling": "mean", "normalize_l2": True,
            "dimensions": 2, "max_tokens": 4, "pad_token": "[PAD]", "pad_token_id": 0}


class ONNXEmbeddingContracts(unittest.TestCase):
    def test_worker_arguments_accept_exact_go_base64url_encoding(self):
        value = {"provider": "onnx", "identity": "a>\u00ff"}
        encoded = base64.urlsafe_b64encode(json.dumps(value, ensure_ascii=False).encode()).rstrip(b"=").decode()
        self.assertEqual(module["decode_argument"](encoded), value)
        with self.assertRaises(ValueError):
            module["decode_argument"]("$invalid")

    def test_python_paths_stay_inside_pinned_installation(self):
        constrain = module["constrain_python_paths"]
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            relative = Path("Lib") if module["os"].name == "nt" else Path("lib") / f"python{module['sys'].version_info.major}.{module['sys'].version_info.minor}"
            site = root / relative / "site-packages"
            site.mkdir(parents=True)
            script = str(root / "worker.py")
            with patch.object(module["sys"], "executable", str(root / "python.exe")), \
                 patch.object(module["sys"], "prefix", str(root)), \
                 patch.object(module["sys"], "base_prefix", str(root)), \
                 patch.object(module["sys"], "path", [str(root / "Lib")]), \
                 patch.dict(constrain.__globals__, {"__file__": script}):
                constrain(str(root))
                # Windows temporary roots may use an 8.3 alias (RUNNER~1).
                # The worker deliberately admits the resolved installation path.
                self.assertEqual(module["sys"].path[-1], str(site.resolve(strict=True)))
                module["sys"].path.append(str(root.parent))
                with self.assertRaises(ValueError):
                    constrain(str(root))

    def test_model_snapshot_hashes_the_actual_parser_buffers(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            originals = {"model.onnx": b"graph snapshot", "tokenizer.json": b"tokenizer snapshot", "embedding.json": b"settings snapshot"}
            pins = {name: hashlib.sha256(raw).hexdigest() for name, raw in originals.items()}
            for name, raw in originals.items():
                (root / name).write_bytes(raw)
            snapshot = module["read_model_snapshot"](root, pins)
            (root / "model.onnx").write_bytes(b"changed after verification")
            self.assertEqual(snapshot, originals)
            with self.assertRaises(ValueError):
                module["read_model_snapshot"](root, pins)
            with self.assertRaises(ValueError):
                module["read_model_snapshot"](root, {**pins, "custom.py": "a" * 64})

    def test_settings_reject_implicit_or_unsupported_semantics(self):
        self.assertEqual(module["validate_settings"](settings()), settings())
        for change in ({"family": "e5-asymmetric"}, {"schema_version": True}, {"dimensions": True}, {"dimensions": 4097},
                       {"max_tokens": 0}, {"pad_token_id": -1}, {"pooling": "guess"},
                       {"normalize_l2": 1}, {"inputs": ["input_ids"]}, {"output": ""}, {"extra": 1}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                module["validate_settings"]({**settings(), **change})

    def test_control_json_is_unambiguous(self):
        for raw in ('{"a":1,"a":2}', '{"a":NaN}', '{"a":"\\ud800"}'):
            with self.subTest(raw=raw), self.assertRaises((ValueError, UnicodeEncodeError)):
                module["strict_json"](raw)

    def test_dependency_identity_is_exact_and_complete(self):
        pins = {name: "1.0.1" for name in module["DEPENDENCIES"]}
        with patch.object(module["metadata"], "version", lambda name: pins[name]):
            module["verify_dependencies"](pins)
            with self.assertRaises(ValueError):
                module["verify_dependencies"]({**pins, "onnxruntime": "1.0.2"})
        with self.assertRaises(ValueError):
            module["verify_dependencies"]({"onnxruntime": "1.0.1"})

    def test_graph_rejects_external_data_and_custom_domains_recursively(self):
        def message(name, children=(), **fields):
            field = SimpleNamespace(type=11, TYPE_MESSAGE=11, is_repeated=True)
            return SimpleNamespace(DESCRIPTOR=SimpleNamespace(full_name="onnx." + name),
                                   ListFields=lambda: [(field, children)] if children else [], **fields)
        safe = message("ModelProto", [message("NodeProto", domain=""), message("TensorProto", external_data=[], data_location=0)])
        module["reject_external_graph"](safe)
        for bad in (message("TensorProto", external_data=["../secret"], data_location=1),
                    message("NodeProto", domain="custom.execute"), message("FunctionProto", domain="custom.execute")):
            with self.assertRaises(ValueError):
                module["reject_external_graph"](message("ModelProto", [message("GraphProto", [bad])]))

    def test_artifact_read_has_exact_finite_bounds(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory)
            (path / "artifact").write_bytes(b"four")
            self.assertEqual(module["read_artifact"](path, "artifact", 4), b"four")
            with self.assertRaises(ValueError):
                module["read_artifact"](path, "artifact", 3)

    def test_token_overflow_is_not_truncated(self):
        worker = Worker.__new__(Worker)
        worker.settings = settings()
        worker.tokenizer = SimpleNamespace(encode_batch=lambda *_args, **_kwargs: [SimpleNamespace(ids=[1, 2, 3, 4, 5])])
        with self.assertRaisesRegex(ValueError, "not truncated"):
            worker.encode(["input"])

    def test_pooling_masks_padding_and_rejects_bad_vectors(self):
        try:
            import numpy as np
        except ImportError:
            self.skipTest("NumPy unavailable: arithmetic contract requires installed dependency; not model qualification")
        worker = Worker.__new__(Worker)
        worker.settings, worker.np = settings(), np
        mask = np.asarray([[1, 1, 0], [1, 0, 0]], dtype=np.int64)
        values = np.asarray([[[1, 0], [0, 1], [100, 100]], [[0, 2], [100, 100], [100, 100]]], dtype=np.float32)
        result = worker.vectors([values], mask)
        self.assertAlmostEqual(result[0][0], 2 ** -0.5, places=6)
        self.assertEqual(result[1], [0.0, 1.0])
        for bad in (np.zeros_like(values), values[:, :, :1], np.full_like(values, np.nan)):
            with self.assertRaises(ValueError):
                worker.vectors([bad], mask)

    def test_cancel_withholds_vectors_and_clears_only_its_own_admission(self):
        worker = Worker.__new__(Worker)
        stop, options = threading.Event(), SimpleNamespace(terminate=False)
        worker.active_lock = threading.Lock()
        worker.active = ("cancel", stop, options)
        worker.encode = lambda _texts: ({}, 3)
        events = []
        worker.send = lambda *args, **kwargs: events.append((args, kwargs))
        stop.set()
        worker.generate({"request_id": "one", "prompt": {"input": ["text"]}}, stop, options)
        self.assertEqual(events, [(("done", "one", 1), {"finish_reason": "cancelled"})])
        self.assertIsNone(worker.active)

    def test_control_rejects_extra_authority_and_duplicate_request(self):
        worker = Worker.__new__(Worker)
        worker.settings, worker.descriptor = settings(), {"model_digest": "sha256:fixture"}
        worker.active_lock, worker.active, worker.seen = threading.Lock(), None, {"one"}
        request = {"request_id": "one", "cancellation_id": "cancel", "model_digest": "sha256:fixture", "context_length": 4, "prompt": {"input": ["text"]}}
        for value in ({"operation": "install"}, {"operation": "health", "command": "run"},
                      {"operation": "generate", "request": {**request, "tools": []}},
                      {"operation": "generate", "request": request}):
            with self.assertRaises(ValueError):
                worker.dispatch(value)


if __name__ == "__main__":
    unittest.main()
