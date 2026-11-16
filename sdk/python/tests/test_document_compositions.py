"""Composition status, response identity and non-replayed writes."""
from copy import deepcopy
from types import SimpleNamespace
import pytest
from offgrid.document_compositions import DocumentCompositions, _status


def status():
    return dict(id="composition", analysis_id="parent", plan_id="tree", source_plan_id="source",
                model=dict(model_id="local", language="en", protocol="document-synthesis-candidates-v1"),
                state="awaiting_review", attempt=1, completed_steps=1, total_steps=1,
                reviews_current=True, created_at="2026-11-13T12:00:00Z", updated_at="2026-11-13T12:00:00Z")


@pytest.mark.parametrize("field,value", [("id", "wrong"), ("completed_steps", True),
    ("completed_steps", 0), ("total_steps", 65537), ("reviews_current", "true"),
    ("state", "approved"), ("source_plan_id", "")])
def test_reject_invalid_status(field, value):
    result = status()
    result[field] = value
    with pytest.raises(ValueError):
        _status(result, "composition")


def test_create_keeps_explicit_model_language_and_disables_replay():
    calls = []
    def request(method, path, data=None, **kwargs):
        calls.append((method, path, data, kwargs))
        return status()
    api = DocumentCompositions(SimpleNamespace(_client=SimpleNamespace(_request=request)))
    assert api.create("key", "parent", "local", "en")["id"] == "composition"
    assert calls[0][3]["retry"] is False
    with pytest.raises(ValueError):
        api.create("key", "other", "local", "en")
    assert len(calls) == 2


def test_report_retains_semantic_review_and_rejects_wrong_source():
    job = status()
    parent = dict(id="parent", completed_batches=1, plan=dict(id="source", units=[{}]))
    result = dict(composition=job, source_units=1, sections=[], gaps=[], extraction_gaps=[],
                  candidate_gaps=[], unrepresented_batches=1, semantic_review_required=True)
    def request(method, path, *args, **kwargs):
        return deepcopy(result if "/report?" in path else job)
    api = DocumentCompositions(SimpleNamespace(_client=SimpleNamespace(_request=request), get=lambda _: parent))
    assert api.report("composition")["unrepresented_batches"] == 1
    result["semantic_review_required"] = False
    with pytest.raises(ValueError):
        api.report("composition")
    result["semantic_review_required"] = True
    result["composition"]["source_plan_id"] = "other"
    with pytest.raises(ValueError):
        api.report("composition")
