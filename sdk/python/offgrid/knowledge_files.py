"""Bounded binary Knowledge transfers through the configured service only."""

import hashlib
import json
import mimetypes
import os
import stat
import uuid
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urlsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener

MAX_FILE_BYTES = 50 << 20


class _NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def _transfer(client, method, path, body=None, content_type=None):
    from .client import OffGridError
    origin = urlsplit(client.base_url)
    if (origin.scheme not in ("http", "https") or not origin.hostname
            or origin.username or origin.password or origin.query or origin.fragment
            or origin.path not in ("", "/")):
        raise ValueError("Knowledge transfers require an HTTP(S) service origin")
    headers = {}
    if client.api_key:
        headers["Authorization"] = "Bearer " + client.api_key
    if content_type:
        headers["Content-Type"] = content_type
    request = Request(client.base_url.rstrip("/") + path, data=body, headers=headers, method=method)
    try:
        # Never follow a redirect with document bytes or credentials, and never
        # replay an upload after a timeout or lost acknowledgment.
        return build_opener(_NoRedirect()).open(request, timeout=client.timeout)
    except HTTPError as error:
        with error:
            raw = error.read(65537)
        code = "document_request_failed"
        try:
            result = json.loads(raw) if len(raw) <= 65536 else {}
            nested = result.get("error") if isinstance(result, dict) else None
            candidate = nested.get("code") if isinstance(nested, dict) else result.get("error_code")
            if candidate in {
                "document_invalid", "document_limit", "document_empty", "document_encrypted",
                "document_unsupported_format", "document_parser_unavailable", "document_ocr_required",
                "document_storage_limit", "document_original_unavailable", "document_cancelled",
                "document_timeout", "document_index_failed",
                "document_ocr_language_missing", "document_ocr_language_invalid",
                "document_ocr_language_unavailable", "document_ocr_language_storage",
            }:
                code = candidate
        except (ValueError, AttributeError, TypeError):
            pass
        raise OffGridError("Knowledge request failed (HTTP %d, %s). Inspect Knowledge status before retrying." % (error.code, code), code=code) from None
    except (URLError, TimeoutError):
        raise OffGridError("Knowledge request was not acknowledged. Inspect its state before retrying.", code="document_outcome_unknown") from None


def upload(client, source, name=None):
    from .client import OffGridError
    info = os.lstat(source)
    if not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= MAX_FILE_BYTES:
        raise ValueError("Choose a regular document between 1 byte and 50 MiB")
    with open(source, "rb") as stream:
        opened = os.fstat(stream.fileno())
        if not os.path.samestat(info, opened):
            raise ValueError("Document changed before upload")
        data = stream.read(MAX_FILE_BYTES + 1)
    if not 0 < len(data) <= MAX_FILE_BYTES:
        raise ValueError("Document exceeds upload limits")
    filename = name or os.path.basename(source)
    if not filename or any(char in filename for char in '\r\n\x00/\\'):
        raise ValueError("Document name must be a plain filename")
    # Preserve the real extension: the server chooses a typed parser from it.
    if os.path.splitext(filename)[1].lower() != os.path.splitext(source)[1].lower():
        raise ValueError("Custom document name must preserve the file extension")
    boundary = "OffGrid" + uuid.uuid4().hex
    while boundary.encode() in data:
        boundary = "OffGrid" + uuid.uuid4().hex
    escaped = filename.replace('"', '\\"')
    media_type = mimetypes.guess_type(source)[0] or "application/octet-stream"
    header = ('--%s\r\nContent-Disposition: form-data; name="file"; filename="%s"\r\nContent-Type: %s\r\n\r\n' % (boundary, escaped, media_type)).encode("utf-8")
    body = header + data + ("\r\n--%s--\r\n" % boundary).encode()
    with _transfer(client, "POST", "/v1/documents/ingest", body, "multipart/form-data; boundary=" + boundary) as response:
        raw = response.read((1 << 20) + 1)
    try:
        result = json.loads(raw) if len(raw) <= (1 << 20) else None
    except ValueError:
        result = None
    if not isinstance(result, dict) or result.get("success") is not True or not isinstance(result.get("document"), dict) or not result["document"].get("id"):
        raise OffGridError("Import acknowledgment is incomplete. Inspect Knowledge before retrying.", code="document_outcome_unknown")
    return result


def import_ocr_language(client, language, source):
    from .client import OffGridError
    limit = 32 << 20
    info = os.lstat(source)
    if not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= limit:
        raise ValueError("Choose a regular OCR language file up to 32 MiB")
    with open(source, "rb") as stream:
        if not os.path.samestat(info, os.fstat(stream.fileno())):
            raise ValueError("OCR language file changed before upload")
        data = stream.read(limit + 1)
    if len(data) > limit:
        raise ValueError("OCR language file exceeds 32 MiB")
    with _transfer(client, "POST", "/v1/documents/ocr-languages?" + urlencode({"language": language}), data, "application/octet-stream") as response:
        raw = response.read((1 << 20) + 1)
    try:
        result = json.loads(raw) if len(raw) <= (1 << 20) else None
        valid = isinstance(result, dict) and isinstance(result.get("languages"), list) and any(
            isinstance(row, dict) and row.get("language") == language and row.get("state") == "installed_verified"
            for row in result["languages"])
    except ValueError:
        valid = False
    if not valid:
        raise OffGridError("OCR setup was not confirmed. Check language status before retrying.", code="document_invalid_response")
    return result


def original(client, document_id):
    """Return verified original bytes; never reconstruct a legacy source."""
    from .client import OffGridError
    path = "/v1/documents/source?" + urlencode({"id": document_id, "format": "original"})
    with _transfer(client, "GET", path) as response:
        data = response.read(MAX_FILE_BYTES + 1)
        valid = (0 < len(data) <= MAX_FILE_BYTES
                 and response.headers.get("Content-Type") == "application/octet-stream"
                 and response.headers.get("X-OffGrid-Source-SHA256") == hashlib.sha256(data).hexdigest())
    if not valid:
        raise OffGridError("Original response failed size, type or digest validation.", code="document_invalid_response")
    return data
