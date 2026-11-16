"""Durable, revision-pinned comparisons. No automatic mutation replay."""
import hashlib
from urllib.parse import urlencode
from .analysis_previews import validate_previews


def analysis_record(value, identity=None, sources=None):
    if not isinstance(value, dict) or not value.get("id") or identity and value["id"] != identity:
        raise ValueError("Analysis identity was not confirmed")
    plan = value.get("plan")
    if (not isinstance(plan, dict) or plan.get("version") != "document-analysis-v1" or
            plan.get("workflow") != "comparison" or not plan.get("id") or
            not isinstance(plan.get("sources"), list) or len(plan["sources"]) != 2 or
            sources is not None and plan["sources"] != sources or
            not isinstance(plan.get("units"), list) or len(plan["units"]) < 2 or not isinstance(plan.get("gaps"), list) or
            not isinstance(value.get("pairs"), list) or not value["pairs"]):
        raise ValueError("Analysis sources or traversal were not confirmed")
    for key, total in (("completed_steps", len(value["pairs"])), ("completed_units", len(plan["units"]))):
        if type(value.get(key)) is not int or not 0 <= value[key] <= total:
            raise ValueError("Invalid analysis progress")
    state = value.get("state")
    if state not in ("queued", "running", "interrupted", "cancelled", "failed", "complete", "incomplete"):
        raise ValueError("Unknown analysis state")
    if state in ("complete", "incomplete") and (value["completed_steps"] != len(value["pairs"]) or value["completed_units"] != len(plan["units"])):
        raise ValueError("Unaccounted analysis source units")
    if state == "complete" and (plan["gaps"] or value.get("review_required") is not False):
        raise ValueError("Incomplete evidence was labelled complete")
    return value


class DocumentAnalyses:
    def __init__(self, client):
        self._client = client

    def export(self, identity, format="json"):
        """Return a verified UTF-8 artifact; the caller controls saving/retention.

        Formats: json, csv, markdown. Only finished comparisons are exportable;
        incomplete extraction remains disclosed in the artifact. No retry.
        """
        if not isinstance(identity, str) or not identity or len(identity.encode()) > 128 or format not in ("json", "csv", "markdown"):
            raise ValueError("Provide an analysis ID and json, csv or markdown format")
        value = self._client._request("GET", "/v1/document-analyses/export?" + urlencode(dict(id=identity, format=format)), retry=False)
        if (not isinstance(value, dict) or value.get("analysis_id") != identity or value.get("format") != format
                or not isinstance(value.get("content"), str) or not value["content"]):
            raise ValueError("Export identity was not confirmed")
        content = value["content"].encode("utf-8")
        if len(content) > 8 << 20 or hashlib.sha256(content).hexdigest() != value.get("sha256"):
            raise ValueError("Export size or digest was not confirmed")
        return value

    def compare(self, request_key, sources):
        """Compare two ordered source references; each pins document_id/revision.

        Include original_sha256 for original-file extractions. This computes
        text and cell changes, not a model-written policy interpretation.
        """
        if not isinstance(request_key, str) or not request_key or len(request_key.encode()) > 128 or not isinstance(sources, list) or len(sources) != 2:
            raise ValueError("Provide a request key and two source references")
        body = dict(request_key=request_key, workflow="comparison", sources=sources)
        return analysis_record(self._client._request("POST", "/v1/document-analyses", body, retry=False), sources=sources)

    def get(self, identity):
        if not identity:
            raise ValueError("Analysis identity is required")
        return analysis_record(self._client._request("GET", "/v1/document-analyses?" + urlencode(dict(id=identity))), identity)

    def list(self, offset=0, limit=20):
        """Bounded owner-private history under current source permissions."""
        if type(offset) is not int or not 0 <= offset <= 256 or type(limit) is not int or not 1 <= limit <= 20:
            raise ValueError("Invalid analysis history pagination")
        page = self._client._request("GET", "/v1/document-analyses?" + urlencode(dict(offset=offset, limit=limit)))
        if (not isinstance(page, dict) or not isinstance(page.get("analyses"), list) or len(page["analyses"]) > limit
                or "next_offset" not in page or page["next_offset"] is not None and (type(page["next_offset"]) is not int or page["next_offset"] != offset + limit or page["next_offset"] > 256)):
            raise ValueError("Invalid analysis history response")
        seen = set()
        for item in page["analyses"]:
            if (not isinstance(item, dict) or not isinstance(item.get("id"), str) or not item["id"] or item["id"] in seen
                    or item.get("state") not in ("queued", "running", "interrupted", "cancelled", "failed", "complete", "incomplete")
                    or not isinstance(item.get("documents"), list) or len(item["documents"]) != 2
                    or any(not isinstance(d, str) or not d for d in item["documents"])):
                raise ValueError("Invalid analysis history item")
            seen.add(item["id"])
        return page

    def _control(self, identity, action):
        if not identity:
            raise ValueError("Analysis identity is required")
        result = self._client._request("POST", "/v1/document-analyses/" + action, dict(id=identity), retry=False)
        if not isinstance(result, dict) or result.get("accepted") is not True:
            raise ValueError("Analysis action unconfirmed; inspect status before retrying")
        return result

    def cancel(self, identity):
        return self._control(identity, "cancel")

    def retry(self, identity):
        return self._control(identity, "retry")

    def results(self, identity, step, *, offset=0, limit=100):
        if (not identity or any(type(n) is not int for n in (step, offset, limit)) or
                step < 0 or not 0 <= offset <= 400000 or not 1 <= limit <= 100):
            raise ValueError("Invalid analysis pagination")
        query = urlencode(dict(id=identity, step=step, offset=offset, limit=limit))
        page = self._client._request("GET", "/v1/document-analyses/results?" + query)
        if (not isinstance(page, dict) or page.get("analysis_id") != identity or
                page.get("step") != step or page.get("offset") != offset or
                type(page.get("total_changes")) is not int or not isinstance(page.get("result"), dict)):
            raise ValueError("Invalid analysis result identity")
        changes = page["result"].get("changes")
        if not isinstance(changes, list) or len(changes) > limit or offset + len(changes) > page["total_changes"]:
            raise ValueError("Invalid analysis result bounds")
        end = offset + len(changes)
        if page.get("next_offset") != (end if end < page["total_changes"] else None):
            raise ValueError("Invalid analysis result cursor")
        validate_previews(page)
        return page

    def source(self, identity, step, change, side):
        if (not identity or any(type(n) is not int or n < 0 for n in (step, change)) or side not in ("before", "after")):
            raise ValueError("Invalid analysis source reference")
        query = urlencode(dict(id=identity, step=step, change=change, side=side))
        result = self._client._request("GET", "/v1/document-analyses/source?" + query)
        citation = result.get("citation") if isinstance(result, dict) else None
        if (not isinstance(citation, dict) or result.get("analysis_id") != identity or
                not result.get("document_id") or citation.get("source") != (0 if side == "before" else 1) or
                not citation.get("revision") or not isinstance(result.get("text"), str) or
                type(result.get("truncated")) is not bool):
            raise ValueError("Invalid analysis source receipt")
        data = result["text"].encode("utf-8")
        span = citation.get("span", {})
        start, end = span.get("start_byte"), span.get("end_byte")
        if (type(start) is not int or type(end) is not int or start < 0 or end < start or
                len(data) > min(end - start, 256 * 1024) or
                result["truncated"] != (len(data) < end - start)):
            raise ValueError("Invalid analysis source span")
        if not result["truncated"] and hashlib.sha256(data).hexdigest() != citation.get("text_sha256"):
            raise ValueError("Analysis source digest mismatch")
        return result
