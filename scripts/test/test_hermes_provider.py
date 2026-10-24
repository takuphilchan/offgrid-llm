"""Offline contract test: no installed Hermes, network or user state required."""
import io
import json
from pathlib import Path
import runpy
import sys
import types
import unittest
from unittest.mock import patch


class Profile:
    def __init__(self, **kwargs):
        self.__dict__.update(kwargs)

    def build_api_kwargs_extras(self, **context):
        return {}, {}


class ProviderTest(unittest.TestCase):
    def test_sequential_default_preserves_base_and_caller_overrides(self):
        registered = []
        modules = {
            "providers": types.SimpleNamespace(register_provider=registered.append),
            "providers.base": types.SimpleNamespace(ProviderProfile=Profile),
        }
        with patch.dict(sys.modules, modules):
            runpy.run_path(str(Path(__file__).resolve().parents[2] / "internal/integrations/plugins/hermes/offgrid/__init__.py"))
        provider = registered[0]
        self.assertEqual(provider.build_api_kwargs_extras(), ({}, {"parallel_tool_calls": False}))
        base_body, base_kwargs = {"fixture": "kept"}, {"reasoning_effort": "none"}
        with patch.object(Profile, "build_api_kwargs_extras", return_value=(base_body, base_kwargs)) as original:
            body, kwargs = provider.build_api_kwargs_extras(model="another-local-model", reasoning_config={"effort": "none"})
            self.assertIs(body, base_body)
            self.assertEqual(kwargs, {"reasoning_effort": "none", "parallel_tool_calls": False})
            self.assertEqual(base_kwargs, {"reasoning_effort": "none"})
            original.assert_called_once_with(model="another-local-model", reasoning_config={"effort": "none"})
            # Hermes transport applies request_overrides after provider kwargs.
            # Verify that a default, not a persistent configuration edit or
            # special request class, is returned for that supported merge.
            for explicit in (True, False, None):
                combined = {**kwargs, **{"parallel_tool_calls": explicit}}
                self.assertIs(combined["parallel_tool_calls"], explicit)
        with patch.object(Profile, "build_api_kwargs_extras", return_value=({}, {"parallel_tool_calls": True})):
            self.assertTrue(provider.build_api_kwargs_extras()[1]["parallel_tool_calls"])

    def test_catalog_is_dynamic_scoped_and_chat_only(self):
        registered = []
        requests = []
        payload = {"data": [{"id": "new-download", "capabilities": ["chat"]},
                            {"id": "embed", "type": "embedding", "capabilities": ["embeddings"]},
                            {"id": "speech", "capabilities": ["speech"]}, {"id": "unknown"}]}

        def open_url(request, timeout):
            requests.append((request, timeout))
            return io.BytesIO(json.dumps(payload).encode())

        modules = {
            "providers": types.SimpleNamespace(register_provider=registered.append),
            "providers.base": types.SimpleNamespace(ProviderProfile=Profile),
            "hermes_cli.urllib_security": types.SimpleNamespace(open_credentialed_url=open_url),
        }
        with patch.dict(sys.modules, modules):
            runpy.run_path(str(Path(__file__).resolve().parents[2] / "internal/integrations/plugins/hermes/offgrid/__init__.py"))
            provider = registered[0]
            self.assertEqual(provider.fetch_models(base_url="http://localhost:1234/v1/", api_key="fixture", timeout=2), ["new-download"])
            self.assertEqual(requests[0][0].full_url, "http://localhost:1234/v1/models")
            self.assertEqual(requests[0][0].get_header("Authorization"), "Bearer fixture")
            payload["data"] = "bad"
            self.assertIsNone(provider.fetch_models())


if __name__ == "__main__":
    unittest.main()
