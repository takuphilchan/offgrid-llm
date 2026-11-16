"""Revision-pinned review and no mutation replay through the authenticated SDK."""
import json
import pytest
from offgrid import OffGridError
from .test_knowledge_files import service


def test_structure_and_attributed_correction(service):
    client, state = service
    state["body"] = json.dumps(dict(document_id="private&id", revision="old", offset=0, blocks=[])).encode()
    assert client.kb.structure("private&id")["revision"] == "old"
    assert "id=private%26id" in state["requests"][0][1]
    assert state["requests"][0][2]["Authorization"] == "Bearer isolated-key"
    state["body"] = json.dumps(dict(success=True, document=dict(id="private&id", extraction_revision="new"))).encode()
    edits = [dict(block_id="clause", expected_text_sha256="a" * 64, text="Corrected clause")]
    assert client.kb.correct("private&id", expected_revision="old", edits=edits)["extraction_revision"] == "new"
    assert json.loads(state["requests"][1][3]) == dict(document_id="private&id", expected_revision="old", edits=edits)


@pytest.mark.parametrize("status,body", [(409, {}), (503, {}), (200, dict(success=True, document=dict(id="one", extraction_revision="old"))), (200, dict(success=True))])
def test_correction_failure_not_replayed(service, status, body):
    client, state = service
    state["status"], state["body"] = status, json.dumps(body).encode()
    with pytest.raises((ValueError, OffGridError)):
        client.kb.correct("one", expected_revision="old", edits=[dict(block_id="clause", expected_text_sha256="a" * 64, text="Correction")])
    assert len(state["requests"]) == 1


def test_navigation_pagination_requires_matching_receipt(service):
    client, state = service
    state["body"] = json.dumps(dict(document_id="d", revision="r", offset=0, blocks=[], navigation_offset=100, navigation=[])).encode()
    assert client.kb.structure("d", revision="r", navigation_offset=100)["navigation_offset"] == 100
    assert "navigation_offset=100" in state["requests"][0][1]
    with pytest.raises(ValueError):
        client.kb.structure("d", revision="r", navigation_offset=200)
