"""Human review identity and recovery boundaries."""
from types import SimpleNamespace
import pytest
from offgrid.synthesis_review import read, save, validate


def fixture():
    receipt = {"batch_id": "batch", "response_sha256": "a" * 64,
               "result": {"candidates": [{}]}}
    value = dict(analysis_id="job", batch=0, batch_id="batch",
                 response_sha256="a" * 64, revision=1, section_checked=True,
                 decisions=[dict(decision="keep", note="Checked")],
                 author="owner", updated_at="2026-11-13T12:00:00Z")
    return receipt, value


@pytest.mark.parametrize("field,value", [
    ("analysis_id", "other"), ("batch_id", "other"),
    ("response_sha256", "b" * 64), ("revision", True), ("author", ""),
    ("decisions", [dict(decision="unreviewed", note="")]),
    ("decisions", [dict(decision="keep", note="中" * 683)]),
])
def test_review_rejects_forged_identity_and_invalid_decisions(field, value):
    receipt, review = fixture()
    review[field] = value
    with pytest.raises(ValueError):
        validate(review, "job", 0, receipt)


def test_review_write_never_replays_and_historical_identity_is_checked():
    receipt, review = fixture()
    calls = []
    def request(*args, **kwargs):
        calls.append((args, kwargs))
        return review
    synthesis = SimpleNamespace(_client=SimpleNamespace(_request=request),
                                results=lambda *_: {"receipts": [receipt]})
    body = dict(id="job", batch=0, batch_id="batch", response_sha256="a" * 64,
                expected_revision=0, section_checked=True,
                decisions=[dict(decision="keep", note="Checked")])
    assert save(synthesis, body)["revision"] == 1
    assert calls[0][1]["retry"] is False
    assert read(synthesis, "job", 0, 1)["author"] == "owner"
    with pytest.raises(ValueError):
        read(synthesis, "job", 0, 2)
    review["decisions"] = [dict(decision="exclude", note="Different")]
    with pytest.raises(ValueError):
        save(synthesis, body)
    assert sum(args[0] == "POST" for args, _ in calls) == 2
