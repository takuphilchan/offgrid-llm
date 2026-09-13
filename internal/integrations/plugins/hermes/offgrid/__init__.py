"""First-party Hermes Agent provider for OffGrid."""

from providers import register_provider
from providers.base import ProviderProfile


offgrid = ProviderProfile(
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
