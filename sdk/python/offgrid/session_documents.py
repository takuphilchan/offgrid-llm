"""Read-only, revision-pinned private attachment inspection."""
import re
from typing import Dict
from urllib.parse import quote, urlencode


class SessionDocuments:
    def attachment_structure(self, name: str, attachment_id: str, *, revision: str,
                             unit: str = "", offset: int = 0, limit: int = 100,
                             relationship_offset: int = 0, navigation_offset: int = 0) -> Dict:
        """Inspect retained structure; revision is the attachment's revision.

        Returns the separate immutable extraction revision. Does not reparse,
        publish to Knowledge, or grant permission to correct private sources.
        """
        if (not name or not re.fullmatch(r"file-[a-f0-9]{32}", attachment_id)
                or not revision or len(revision) > 128 or len(unit) > 128
                or not 0 <= offset <= 200000 or not 1 <= limit <= 200
                or not 0 <= relationship_offset <= 400000 or type(navigation_offset) is not int or not 0 <= navigation_offset <= 200000):
            raise ValueError("Invalid attachment structure request")
        query = urlencode(dict(revision=revision, unit=unit, offset=offset,
                               limit=limit, relationship_offset=relationship_offset, navigation_offset=navigation_offset))
        result = self._client._request("GET", f"/v1/sessions/{quote(name, safe='')}/attachments/{attachment_id}/structure?{query}")
        if ((result.get("navigation_offset", 0) != navigation_offset or navigation_offset > 0 and not isinstance(result.get("navigation"), list)) or result.get("document_id") != attachment_id or not result.get("revision")
                or result.get("offset") != offset
                or result.get("relationship_offset") != relationship_offset
                or result.get("can_correct") is not False
                or not isinstance(result.get("blocks"), list)
                or len(result["blocks"]) > limit
                or any(block.get("editable") is not False for block in result["blocks"])):
            raise ValueError("Invalid private structure receipt")
        return result

    def attachment_revisions(self, name: str, attachment_id: str) -> Dict:
        """Inspect bounded revision metadata under current attachment access."""
        if not name or not re.fullmatch(r"file-[a-f0-9]{32}", attachment_id):
            raise ValueError("Invalid attachment identity")
        result = self._client._request("GET", f"/v1/sessions/{quote(name, safe='')}/attachments/{attachment_id}/revisions")
        items = result.get("revisions")
        if (result.get("id") != attachment_id or not isinstance(items, list) or not 1 <= len(items) <= 65
                or result.get("current_revision") != items[0].get("revision")
                or any(not re.fullmatch(r"[a-f0-9]{64}", r.get("revision", "")) or not r.get("extraction_revision") for r in items)
                or len({r["revision"] for r in items}) != len(items)):
            raise ValueError("Invalid attachment revision history")
        return result

    def correct_attachment(self, name: str, attachment_id: str, *, expected_revision: str,
                           extraction_revision: str, edits: list) -> Dict:
        """Publish a derived revision. An uncertain write is never retried."""
        if (not name or not re.fullmatch(r"file-[a-f0-9]{32}", attachment_id)
                or not re.fullmatch(r"[a-f0-9]{64}", expected_revision) or not extraction_revision
                or not isinstance(edits, list) or not 1 <= len(edits) <= 256):
            raise ValueError("Invalid attachment correction")
        identity = self._client._request("GET", "/api/v2/system")
        if identity.get("product") != "offgrid" or "session-document-corrections-v1" not in identity.get("capabilities", []):
            raise ValueError("Update the service before correcting private documents")
        result = self._client._request("POST", f"/v1/sessions/{quote(name, safe='')}/attachments/{attachment_id}/corrections",
                                      dict(expected_revision=expected_revision, extraction_revision=extraction_revision, edits=edits), retry=False)
        item = result.get("attachment", {})
        if (item.get("id") != attachment_id or item.get("status") != "ready" or item.get("parent_revision") != expected_revision
                or not re.fullmatch(r"[a-f0-9]{64}", item.get("revision", "")) or item["revision"] == expected_revision
                or not item.get("extraction_revision") or item["extraction_revision"] == extraction_revision):
            raise ValueError("Correction was not confirmed. Inspect revisions before retrying.")
        return item
