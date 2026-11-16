"""Private structured inspection never becomes a Knowledge write."""
import json
import pytest
from .test_knowledge_files import service


def test_private_structure_uses_source_revision_and_authentication(service):
    client, state = service
    attachment = "file-" + "a" * 32
    state["body"] = json.dumps(dict(document_id=attachment, revision="extraction-one",
                                    offset=100, relationship_offset=2000,
                                    can_correct=False, blocks=[])).encode()
    result = client.sessions.attachment_structure("chat & policy", attachment,
                                                 revision="source-one", offset=100,
                                                 relationship_offset=2000)
    assert result["revision"] == "extraction-one"
    method, path, headers, _ = state["requests"][0]
    assert method == "GET" and "chat%20%26%20policy" in path and "revision=source-one" in path
    assert headers["Authorization"] == "Bearer isolated-key"


@pytest.mark.parametrize("change", [dict(can_correct=True), dict(document_id="other"),
                                  dict(offset=1), dict(blocks=[dict(editable=True)])])
def test_private_structure_rejects_mismatched_receipts(service, change):
    client, state = service
    attachment = "file-" + "a" * 32
    result = dict(document_id=attachment, revision="extraction-one", offset=0,
                  relationship_offset=0, can_correct=False, blocks=[])
    result.update(change)
    state["body"] = json.dumps(result).encode()
    with pytest.raises(ValueError):
        client.sessions.attachment_structure("chat", attachment, revision="source-one")


def test_private_navigation_requires_matching_cursor(service):
    client, state = service
    attachment = "file-" + "a" * 32
    state["body"] = json.dumps(dict(document_id=attachment, revision="extraction-one", offset=0, relationship_offset=0, navigation_offset=100, navigation=[], can_correct=False, blocks=[])).encode()
    assert client.sessions.attachment_structure("chat", attachment, revision="source-one", navigation_offset=100)["navigation_offset"] == 100
    with pytest.raises(ValueError):
        client.sessions.attachment_structure("chat", attachment, revision="source-one", navigation_offset=200)
