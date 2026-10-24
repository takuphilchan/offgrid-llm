"""First-party Hermes Agent provider for OffGrid."""

from providers import register_provider
from providers.base import ProviderProfile


class OffGridProfile(ProviderProfile):
    def build_api_kwargs_extras(self, *, reasoning_config=None, **context):
        """Default to one native call; Hermes merges explicit overrides last.

        No tool filtering or message rewriting. This is also safe for text-only
        auxiliary requests: cardinality alone never enables a tool or selects
        OffGrid's private governed-agent protocol.
        """
        body, kwargs = super().build_api_kwargs_extras(
            reasoning_config=reasoning_config, **context)
        return body, {"parallel_tool_calls": False, **kwargs}

    def fetch_models(self, *, api_key=None, base_url=None, timeout=8.0):
        """Discover chat models, not embedding/speech assets, without static IDs."""
        import json
        import urllib.request
        from hermes_cli.urllib_security import open_credentialed_url

        request = urllib.request.Request((base_url or self.base_url).rstrip("/") + "/models")
        request.add_header("Accept", "application/json")
        if api_key:
            request.add_header("Authorization", "Bearer " + api_key)
        try:
            with open_credentialed_url(request, timeout=timeout) as response:
                data = json.loads(response.read(2 * 1024 * 1024))
            items = data.get("data")
            if not isinstance(items, list):
                return None
            return [item["id"] for item in items if isinstance(item, dict)
                    and isinstance(item.get("id"), str) and item["id"]
                    and item.get("type") != "embedding"
                    and "chat" in (item.get("capabilities") or [])]
        except (OSError, ValueError, TypeError, AttributeError):
            return None


offgrid = OffGridProfile(
    name="offgrid",
    aliases=("offgrid-llm",),
    display_name="OffGrid",
    description="Private local inference through an OffGrid runtime",
    env_vars=("OFFGRID_API_KEY", "OFFGRID_BASE_URL"),
    base_url="http://127.0.0.1:11611/v1",
    auth_type="api_key",
    api_mode="chat_completions",
    supports_health_check=True,
)

register_provider(offgrid)
