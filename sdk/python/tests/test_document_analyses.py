"""Analysis API transport and source validation; no model-quality assertions."""
import hashlib
import json
import pytest

def test_analysis_export_receipt(service):
    client, state = service
    text = "政策 17.25"
    receipt = dict(analysis_id="job", format="markdown", content=text, sha256=hashlib.sha256(text.encode()).hexdigest())
    state["body"] = json.dumps(receipt).encode()
    assert client.kb.analyses.export("job", "markdown")["content"] == text
    receipt["content"] = "forged"
    state["body"] = json.dumps(receipt).encode()
    with pytest.raises(ValueError):
        client.kb.analyses.export("job", "markdown")
    with pytest.raises(ValueError):
        client.kb.analyses.export("job", "html")
    assert len(state["requests"]) == 2

def test_analysis_history_bounds_and_receipts(service):
    client, state = service
    state["body"] = b'{"analyses":[],"next_offset":null}'
    assert client.kb.analyses.list(20, 10)["analyses"] == []
    assert "offset=20&limit=10" in state["requests"][0][1]
    with pytest.raises(ValueError):
        client.kb.analyses.list(-1)
    state["body"] = b'{"analyses":[],"next_offset":0}'
    with pytest.raises(ValueError):
        client.kb.analyses.list()

def test_analysis_preview_integrity(service):
    client, state = service
    text = "政策 17.25"
    digest = hashlib.sha256(text.encode()).hexdigest()
    span = dict(start_byte=0, end_byte=len(text.encode()))
    citation = dict(source=0, revision="old", unit_id="page-1", span=span, text_sha256=digest)
    preview = dict(text=text, span=span, text_sha256=digest, truncated=False)
    page = dict(analysis_id="job", step=0, offset=0, next_offset=None, total_changes=1,
                result=dict(changes=[dict(kind="text_removed", before=citation)]), previews=[dict(before=preview)])
    state["body"] = json.dumps(page).encode()
    assert client.kb.analyses.results("job", 0)["previews"][0]["before"]["text"] == text
    preview["text"] = "altered"
    state["body"] = json.dumps(page).encode()
    with pytest.raises(ValueError):
        client.kb.analyses.results("job", 0)
from offgrid import OffGridError
from .test_knowledge_files import service


def test_analysis_source_digest(service):
    client, state = service
    text = "政策 17.25"
    result = dict(analysis_id="private&id", document_id="before", text=text, truncated=False,
                  citation=dict(source=0, revision="old", unit_id="u", span=dict(start_byte=0, end_byte=len(text.encode())), text_sha256=hashlib.sha256(text.encode()).hexdigest()))
    state["body"] = json.dumps(result).encode()
    assert client.kb.analyses.source("private&id", 0, 0, "before")["text"] == text
    assert "id=private%26id" in state["requests"][0][1]
    result["citation"]["text_sha256"] = "0" * 64
    state["body"] = json.dumps(result).encode()
    with pytest.raises(ValueError):
        client.kb.analyses.source("private&id", 0, 0, "before")


@pytest.mark.parametrize("status", [409, 503])
def test_analysis_creation_is_not_replayed(service, status):
    client, state = service
    state["status"], state["body"] = status, b"{}"
    with pytest.raises(OffGridError):
        client.kb.analyses.compare("request", [dict(document_id="before", revision="old"), dict(document_id="after", revision="new")])
    assert len(state["requests"]) == 1


def test_analysis_result_cursor(service):
    client, state = service
    state["body"] = json.dumps(dict(analysis_id="job", step=0, offset=0, next_offset=9, total_changes=5, result=dict(changes=[]))).encode()
    with pytest.raises(ValueError):
        client.kb.analyses.results("job", 0)
