"""Binary document transfer contracts with disposable loopback servers only."""
import hashlib
import json
import threading
from email.parser import BytesParser
from email.policy import default
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlsplit

import pytest
from offgrid import Client, OffGridError
from offgrid import knowledge_files


def test_explicit_attachment_import_does_not_upload_or_retry(service):
    client, state = service
    state["status"] = 202
    state["body"] = json.dumps({"id": "import-one", "state": "accepted"}).encode()
    result = client.kb.import_attachment("private", "file-" + "a" * 32, "b" * 64)
    assert result["state"] == "accepted"
    method, path, _, body = state["requests"][0]
    assert (method, path) == ("POST", "/v1/documents/imports")
    assert json.loads(body) == {"attachment": {"session": "private", "id": "file-" + "a" * 32, "sha256": "b" * 64}}
    state["status"] = 503
    with pytest.raises(OffGridError):
        client.kb.import_attachment("private", "file-" + "a" * 32, "b" * 64)
    assert len(state["requests"]) == 2
    with pytest.raises(ValueError):
        client.kb.import_attachment("private", "../wrong", "b" * 64)
    assert len(state["requests"]) == 2


@pytest.fixture
def service():
    state = {"requests": [], "status": 200, "body": b"", "headers": {}}

    class Handler(BaseHTTPRequestHandler):
        def handle_request(self):
            body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
            state["requests"].append((self.command, self.path, self.headers, body))
            self.send_response(state["status"])
            for key, value in state["headers"].items():
                self.send_header(key, value)
            self.end_headers()
            self.wfile.write(state["body"])

        do_POST = handle_request
        do_GET = handle_request
        do_DELETE = handle_request
        do_PUT = handle_request

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    client = Client(host="http://127.0.0.1:%d" % server.server_port, api_key="isolated-key")
    try:
        yield client, state
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


def test_offline_ocr_language_transfer_and_acknowledgment(service, tmp_path):
    client, state = service
    path = tmp_path / "eng.traineddata"
    path.write_bytes(b"opaque language fixture")
    state["body"] = json.dumps({"languages": [{"language": "eng", "state": "installed_verified"}]}).encode()
    assert client.kb.import_ocr_language("eng", path)["languages"][0]["state"] == "installed_verified"
    method, endpoint, headers, body = state["requests"][0]
    assert (method, endpoint, body) == ("POST", "/v1/documents/ocr-languages?language=eng", path.read_bytes())
    assert headers["Content-Type"] == "application/octet-stream"
    assert headers["Authorization"] == "Bearer isolated-key"
    state["body"] = b'{"languages": [{"language":"eng", "state":"missing"}]}'
    with pytest.raises(OffGridError):
        client.kb.import_ocr_language("eng", path)
    assert len(state["requests"]) == 2  # no silent retry, setup or network acquisition


@pytest.mark.parametrize("suffix", [".pdf", ".docx", ".xlsx", ".pptx", ".txt"])
def test_binary_upload_preserves_bytes_and_name(service, tmp_path, suffix):
    client, state = service
    path = tmp_path / ("source 日本語" + suffix)
    data = b"%PDF-binary\x00\xff\xfe\r\n" + "café".encode()
    path.write_bytes(data)
    state["body"] = json.dumps({"success": True, "document": {"id": "retained", "index_status": "ready"}}).encode()
    assert client.kb.add(path)["document"]["id"] == "retained"
    assert len(state["requests"]) == 1  # no implicit enable, model selection or retries
    method, endpoint, headers, body = state["requests"][0]
    assert (method, endpoint) == ("POST", "/v1/documents/ingest")
    assert headers["Authorization"] == "Bearer isolated-key"
    message = BytesParser(policy=default).parsebytes(("Content-Type: " + headers["Content-Type"] + "\r\n\r\n").encode() + body)
    parts = list(message.iter_parts())
    assert len(parts) == 1
    assert parts[0].get_filename() == path.name
    assert parts[0].get_payload(decode=True) == data


@pytest.mark.parametrize("problem", ["missing", "directory", "oversize", "empty", "header", "extension"])
def test_invalid_local_file_never_transfers(service, tmp_path, monkeypatch, problem):
    client, state = service
    path = tmp_path / "source.pdf"
    path.write_bytes(b"source")
    name = None
    if problem == "missing":
        path.unlink()
    elif problem == "directory":
        path = tmp_path
    elif problem == "empty":
        path.write_bytes(b"")
    elif problem == "oversize":
        monkeypatch.setattr(knowledge_files, "MAX_FILE_BYTES", 3)
    elif problem == "header":
        name = "evil\r\nInjected: value.pdf"
    else:
        name = "source.txt"
    with pytest.raises((ValueError, OSError)):
        client.kb.add(path, name=name)
    assert not state["requests"]


@pytest.mark.parametrize("status", [302, 307, 503])
def test_upload_never_redirects_or_retries(service, tmp_path, status):
    client, state = service
    path = tmp_path / "source.pdf"
    path.write_bytes(b"private bytes")
    state.update(status=status, headers={"Location": client.base_url + "/unapproved"})
    state["body"] = b'{"error":"PRIVATE diagnostic", "error_code":"document_parser_unavailable"}'
    with pytest.raises(OffGridError) as failure:
        client.kb.add(path)
    assert "PRIVATE" not in str(failure.value)
    assert failure.value.code == "document_parser_unavailable"
    assert len(state["requests"]) == 1


def test_incomplete_receipt_is_not_success(service, tmp_path):
    client, state = service
    path = tmp_path / "source.txt"
    path.write_text("hello world")
    state["body"] = b'{"success":true}'
    with pytest.raises(OffGridError) as failure:
        client.kb.add(path)
    assert failure.value.code == "document_outcome_unknown"
    assert len(state["requests"]) == 1


def test_original_verifies_digest_and_query_identity(service):
    client, state = service
    data = b"original\x00\xff"
    state.update(body=data, headers={"Content-Type": "application/octet-stream", "X-OffGrid-Source-SHA256": hashlib.sha256(data).hexdigest()})
    assert client.kb.original("id&another=value") == data
    query = parse_qs(urlsplit(state["requests"][0][1]).query)
    assert query == {"id": ["id&another=value"], "format": ["original"]}
    state["headers"]["X-OffGrid-Source-SHA256"] = "incorrect"
    with pytest.raises(OffGridError) as failure:
        client.kb.original("id")
    assert failure.value.code == "document_invalid_response"


def test_text_and_url_import_use_explicit_service_contract(monkeypatch):
    client = Client()
    requests = []
    monkeypatch.setattr(client, "_request", lambda *args, **kwargs: requests.append((args, kwargs)))
    client.kb.add("note", content="")
    client.kb.add("https://example.test/document")
    assert requests == [
        (("POST", "/v1/documents/ingest", {"name": "note", "content": ""}), {"retry": False}),
        (("POST", "/v1/documents/ingest-url", {"url": "https://example.test/document"}), {"retry": False}),
    ]


def test_explicit_search_setup_and_source_locators(monkeypatch):
    client = Client()
    requests = []
    def respond(*args, **kwargs):
        requests.append((args, kwargs))
        return {"success": True}
    monkeypatch.setattr(client, "_request", respond)
    assert client.kb.enable(embedding_model="installed-search") is True
    assert requests == [(("POST", "/v1/rag/enable", {"embedding_model": "installed-search"}), {"retry": False})]
    source = {"kind": "page", "page": 2, "source_sha256": "retained-revision"}
    monkeypatch.setattr(client, "_request", lambda *args, **kwargs: {"results": [{"chunk": {"content": "evidence", "source": source, "page": 2, "content_hash": "exact-text"}}]})
    result = client.kb.search("question")[0]
    assert result["source"] == source
    assert result["page"] == 2
    assert result["content_hash"] == "exact-text"


@pytest.mark.parametrize("method,verb,action", [
    ("import_status", "GET", None),
    ("retry_import", "POST", "retry"),
    ("cancel_import", "POST", "cancel"),
    ("remove_import", "DELETE", None),
])
def test_import_recovery_uses_authenticated_identity(service, method, verb, action):
    client, state = service
    state["body"] = b'{"id":"saved", "state":"needs_input"}'
    assert getattr(client.kb, method)("id&other=value")["id"] == "saved"
    assert len(state["requests"]) == 1
    actual, path, headers, body = state["requests"][0]
    assert actual == verb
    assert headers["Authorization"] == "Bearer isolated-key"
    query = {"id": ["id&other=value"]}
    if action:
        query["action"] = [action]
    assert parse_qs(urlsplit(path).query) == query
    assert urlsplit(path).path == "/v1/documents/imports"
    assert not body  # no re-upload or source data in control requests


@pytest.mark.parametrize("method", ["retry_import", "cancel_import", "remove_import"])
def test_import_mutations_never_retry_ambiguous_failures(service, method):
    client, state = service
    state.update(status=503, body=b'{"error":"unavailable"}')
    with pytest.raises(OffGridError):
        getattr(client.kb, method)("saved")
    assert len(state["requests"]) == 1


def test_import_list_preserves_durable_failure(service):
    client, state = service
    item = {"id": "saved", "state": "needs_input", "error_code": "document_ocr_required"}
    state["body"] = json.dumps({"imports": [item]}).encode()
    assert client.kb.imports() == [item]


def test_partial_review_is_scoped_read_only(service):
    client, state = service
    state["body"] = b'{"version":1,"state":"partial","source_units":[]}'
    assert client.kb.review_import("saved&other=value")["state"] == "partial"
    verb, path, headers, body = state["requests"][0]
    assert verb == "GET"
    assert headers["Authorization"] == "Bearer isolated-key"
    assert parse_qs(urlsplit(path).query) == {"id": ["saved&other=value"], "view": ["extraction"]}
    assert not body
