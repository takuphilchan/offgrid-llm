"""Candidate and export boundaries; execution requires the Python CI environment."""
import hashlib
import json
import pytest
from .test_knowledge_files import service


def test_synthesis_export_receipt_and_no_replay(service):
    client, state = service
    text = "Draft — 政策 17.25"
    receipt = dict(analysis_id="job", format="csv", content=text,
                   sha256=hashlib.sha256(text.encode()).hexdigest())
    state["body"] = json.dumps(receipt).encode()
    assert client.kb.synthesis.export("job", "csv")["content"] == text
    assert "/v1/document-analyses/synthesis/export?" in state["requests"][0][1]
    receipt["content"] = "forged"
    state["body"] = json.dumps(receipt).encode()
    with pytest.raises(ValueError):
        client.kb.synthesis.export("job", "csv")
    with pytest.raises(ValueError):
        client.kb.synthesis.export("job", "html")
    assert len(state["requests"]) == 2


def test_synthesis_rejects_false_complete_and_unsupported_action(service):
    client, state = service
    state["body"] = b'{"id":"job","state":"complete","plan":{},"model":{}}'
    with pytest.raises(ValueError):
        client.kb.synthesis.get("job")
    with pytest.raises(ValueError):
        client.kb.synthesis.control("job", "approve")
    assert len(state["requests"]) == 1
