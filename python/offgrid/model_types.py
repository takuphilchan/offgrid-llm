"""Typed projections of the v2 model API (no runtime dependencies)."""
from typing import Any, Dict, List, Literal, TypedDict

ModelCategory = Literal["language", "embeddings", "speech_recognition", "speech_generation"]


class ModelReadiness(TypedDict):
    installed: bool
    integrity: str
    runtime_compatible: bool
    smoke_tested: bool
    qualified: bool


class TypedModel(TypedDict, total=False):
    id: str
    revision: str
    name: str
    kind: str
    category: ModelCategory
    capabilities: List[str]
    readiness: ModelReadiness
    provenance: Dict[str, str]
    package: Dict[str, Any]
    variants: List[Dict[str, Any]]


class ModelResolution(TypedDict):
    id: str
    expires_at: str
    resolution: Dict[str, Any]
    preflight: Dict[str, Any]


class ModelOperation(TypedDict, total=False):
    id: str
    request_id: str
    actor_id: str
    action: Literal["install", "repair"]
    state: Literal["queued", "downloading", "verifying", "activating", "cancelling", "complete", "cancelled", "failed", "interrupted"]
    target: Dict[str, Any]
    provenance: Dict[str, str]
    artifacts: List[Dict[str, Any]]
    bytes_done: int
    bytes_total: int
    retained_bytes: int
    discarded: bool
    message: str
    error_code: str
