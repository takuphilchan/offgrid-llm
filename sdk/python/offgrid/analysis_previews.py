"""Validate additive bounded source previews without fetching or executing content."""
import hashlib


def validate_previews(page):
    if "previews" not in page:
        return
    previews, changes = page["previews"], page["result"]["changes"]
    if not isinstance(previews, list) or len(previews) != len(changes):
        raise ValueError("Invalid source preview count")
    for preview, change in zip(previews, changes):
        if not isinstance(preview, dict) or not isinstance(change, dict):
            raise ValueError("Invalid source preview")
        for side in ("before", "after"):
            value, citation = preview.get(side), change.get(side)
            if (value is None) != (citation is None):
                raise ValueError("Source preview side mismatch")
            if value is None:
                continue
            if not isinstance(value, dict) or not isinstance(citation, dict) or not isinstance(value.get("text"), str) or type(value.get("truncated")) is not bool:
                raise ValueError("Invalid source preview fields")
            span, original = value.get("span"), citation.get("span")
            if not isinstance(span, dict) or not isinstance(original, dict):
                raise ValueError("Invalid source preview span")
            start, end = span.get("start_byte"), span.get("end_byte")
            data = value["text"].encode("utf-8")
            if (type(start) is not int or type(end) is not int or type(original.get("end_byte")) is not int
                    or start != original.get("start_byte") or start < 0 or end < start or end > original["end_byte"]
                    or len(data) > 512 or len(data) != end - start or value["truncated"] != (end < original["end_byte"])
                    or hashlib.sha256(data).hexdigest() != value.get("text_sha256")
                    or not value["truncated"] and value["text_sha256"] != citation.get("text_sha256")):
                raise ValueError("Source preview does not match its citation")
