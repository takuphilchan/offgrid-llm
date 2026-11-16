import pytest
from offgrid.session_documents import SessionDocuments


class Client:
    def __init__(self, mode="ok"):
        self.mode = mode
        self.writes = []

    def _request(self, method, path, body=None, **kwargs):
        if path == "/api/v2/system":
            return dict(product="offgrid", capabilities=[] if self.mode == "legacy" else ["session-document-corrections-v1"])
        if method == "POST":
            self.writes.append((path, body, kwargs))
            assert kwargs["retry"] is False
            if self.mode == "uncertain":
                raise RuntimeError("Unconfirmed response")
            return dict(attachment=dict(id="file-" + "a" * 32, status="ready", revision="c" * 64,
                                        parent_revision="d" * 64 if self.mode == "wrong" else "b" * 64,
                                        extraction_revision="extraction-next"))
        return dict(id="file-" + "a" * 32, current_revision="b" * 64,
                    revisions=[dict(revision="b" * 64, extraction_revision="extraction-old")])


@pytest.mark.parametrize("mode", ["ok", "legacy", "wrong", "uncertain"])
def test_correction_is_pinned_capability_gated_and_not_replayed(mode):
    service = SessionDocuments()
    service._client = Client(mode)
    def save():
        return service.correct_attachment("chat & policy", "file-" + "a" * 32,
                                          expected_revision="b" * 64, extraction_revision="extraction-old",
                                          edits=[dict(block_id="note", expected_text_sha256="d" * 64, text="corrected")])
    if mode == "ok":
        assert save()["revision"] == "c" * 64
    else:
        with pytest.raises((ValueError, RuntimeError)):
            save()
    assert len(service._client.writes) == (0 if mode == "legacy" else 1)
    if service._client.writes:
        path, body, _ = service._client.writes[0]
        assert "chat%20%26%20policy" in path
        assert body["expected_revision"] == "b" * 64


def test_revision_history_remains_bounded_and_pinned():
    service = SessionDocuments()
    service._client = Client()
    result = service.attachment_revisions("chat", "file-" + "a" * 32)
    assert result["revisions"][0]["revision"] == result["current_revision"]
