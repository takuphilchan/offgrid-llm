"""Explicit, revision-checked Knowledge collection management. No mutation retries."""
from urllib.parse import urlencode


def policy(value, identity=None):
    if (not isinstance(value, dict) or not value.get("id") or
        (identity and value["id"] != identity) or value.get("version") != 1 or
        type(value.get("revision")) is not int or value["revision"] < 1 or
        not value.get("name") or (value["id"] != "workspace" and not value.get("owner")) or
        any(not isinstance(value.get(key), list) or len(value[key]) > 128 or
            any(not isinstance(member, str) or not member for member in value[key])
            for key in ("readers", "editors"))):
        raise ValueError("Collection state was not confirmed. Inspect the audience before retrying.")
    return value


class Collections:
    def __init__(self, client):
        self._client = client

    def list(self):
        result = self._client._request("GET", "/v1/collections")
        values = result.get("collections") if isinstance(result, dict) else None
        if not isinstance(values, list) or len(values) > 1024:
            raise ValueError("Collection list is incomplete")
        values = [policy(item) for item in values]
        if len({item["id"] for item in values}) != len(values):
            raise ValueError("Duplicate collection identity")
        return values

    def get(self, identity):
        if not identity:
            raise ValueError("Collection identity is required")
        return policy(self._client._request("GET", "/v1/collections?" + urlencode(dict(id=identity))), identity)

    def audit(self, identity, *, after=0):
        if not identity or type(after) is not int or after < 0:
            raise ValueError("Provide a collection identity and nonnegative audit cursor")
        result = self._client._request("GET", "/v1/collections?" + urlencode(dict(id=identity, view="audit", after=after)))
        events = result.get("events") if isinstance(result, dict) else None
        if not isinstance(events, list) or len(events) > 100:
            raise ValueError("Collection audit page is incomplete")
        for event in events:
            if not isinstance(event, dict) or type(event.get("sequence")) is not int or event["sequence"] <= after or any(not event.get(key) for key in ("actor", "action", "subject", "created_at")):
                raise ValueError("Collection audit sequence is invalid")
            after = event["sequence"]
        return events

    def create(self, name):
        result = policy(self._client._request("POST", "/v1/collections", dict(name=name), retry=False))
        if result["id"] == "workspace" or result["revision"] != 1 or result["name"] != name or result["readers"] or result["editors"]:
            raise ValueError("Private collection creation was not confirmed")
        return result

    def update(self, identity, *, expected_revision, name, readers, editors):
        if (not identity or type(expected_revision) is not int or expected_revision < 1 or
            any(not isinstance(items, list) or len(items) > 128 or
                any(not isinstance(item, str) or not item for item in items) or len(set(items)) != len(items)
                for items in (readers, editors))):
            raise ValueError("Provide explicit membership and the current revision")
        body = dict(expected_revision=expected_revision, name=name, readers=readers, editors=editors)
        result = policy(self._client._request("PUT", "/v1/collections?" + urlencode(dict(id=identity)), body, retry=False), identity)
        if result["revision"] != expected_revision + 1 or result["name"] != name or sorted(result["readers"]) != sorted(readers) or sorted(result["editors"]) != sorted(editors):
            raise ValueError("Sharing change was not confirmed. Inspect the current audience before retrying.")
        return result

    def document(self, document_id):
        result = self._client._request("GET", "/v1/documents/collection?" + urlencode(dict(id=document_id)))
        if not isinstance(result, dict) or result.get("document_id") != document_id or type(result.get("revision")) is not int or result["revision"] < 1:
            raise ValueError("Document collection was not confirmed")
        policy(result.get("collection"))
        return result

    def import_file(self, identity, expected_revision, path, *, ocr_language=None):
        """Retain an original privately; process its returned import ID explicitly."""
        from .knowledge_files import upload
        return upload(self._client, path, collection=identity, revision=expected_revision, ocr_language=ocr_language)

    def move(self, document_id, *, expected_revision, expected_source_revision, destination_id, expected_destination_revision):
        if not document_id or not destination_id or any(type(n) is not int or n < 1 for n in (expected_revision, expected_source_revision, expected_destination_revision)):
            raise ValueError("Review both collection audiences before moving a document")
        body = dict(document_id=document_id, expected_revision=expected_revision, expected_source_revision=expected_source_revision,
                    destination_id=destination_id, expected_destination_revision=expected_destination_revision)
        result = self._client._request("PUT", "/v1/documents/collection", body, retry=False)
        if not isinstance(result, dict) or result.get("document_id") != document_id or type(result.get("revision")) is not int or result.get("revision") not in (expected_revision, expected_revision + 1):
            raise ValueError("Document movement was not confirmed")
        saved = policy(result.get("collection"), destination_id)
        if saved["revision"] != expected_destination_revision:
            raise ValueError("Destination audience does not match the reviewed revision")
        return result
