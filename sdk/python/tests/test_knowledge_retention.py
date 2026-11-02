"""Retention receipts and no blind replay through the authenticated SDK."""
import json
import pytest
from offgrid import OffGridError
from .test_knowledge_files import service


def test_retention_get_and_revision_checked_update(service):
    client, state = service
    state["body"] = json.dumps(dict(version=1, revision=1, delete_after=0, legal_hold=False)).encode()
    assert client.kb.retention("private&id")["revision"] == 1
    assert state["requests"][0][1] == "/v1/documents/retention?id=private%26id"
    assert state["requests"][0][2]["Authorization"] == "Bearer isolated-key"
    state["body"] = json.dumps(dict(version=1, revision=2, delete_after=0, legal_hold=True)).encode()
    assert client.kb.set_retention("private&id", revision=1, delete_after=0, legal_hold=True)["legal_hold"] is True
    assert json.loads(state["requests"][1][3]) == dict(version=1, revision=1, delete_after=0, legal_hold=True)


@pytest.mark.parametrize("status,body", [
    (409, {"error": "Changed"}), (503, {"error": "Unavailable"}),
    (200, dict(version=1, revision=2, delete_after=0)),
    (200, dict(version=1, revision=2, legal_hold=True)),
    (200, dict(version=1, revision=1, delete_after=0, legal_hold=True)),
    (200, dict(version=1, revision=2, delete_after=0, legal_hold=False)),
    (200, dict(version=True, revision=2, delete_after=0, legal_hold=True)),
])
def test_retention_failure_is_not_success_or_retried(service, status, body):
    client, state = service
    state["status"], state["body"] = status, json.dumps(body).encode()
    with pytest.raises((ValueError, OffGridError)):
        client.kb.set_retention("one", revision=1, delete_after=0, legal_hold=True)
    assert len(state["requests"]) == 1


@pytest.mark.parametrize("changes", [dict(revision=True), dict(delete_after=-1), dict(delete_after=253402300800), dict(legal_hold=1)])
def test_invalid_retention_does_not_send(service, changes):
    client, state = service
    args = dict(revision=1, delete_after=0, legal_hold=False)
    args.update(changes)
    with pytest.raises(ValueError):
        client.kb.set_retention("one", **args)
    assert state["requests"] == []
