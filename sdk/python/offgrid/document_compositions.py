"""Resumable local summary composition; source identity is not semantic approval."""
from hashlib import sha256
from urllib.parse import urlencode

_BASE = "/v1/document-analyses/synthesis/compositions"
_STATES = ("queued", "running", "interrupted", "cancelled", "failed", "awaiting_review")


def _integer(value, maximum=100000):
    return type(value) is int and 0 <= value <= maximum


def _status(result, composition_id=None):
    if not isinstance(result, dict) or any(not isinstance(result.get(k), str) or not result[k] for k in ("id", "analysis_id", "plan_id", "source_plan_id", "created_at", "updated_at")):
        raise ValueError("Invalid composition identity")
    if composition_id and result["id"] != composition_id:
        raise ValueError("Wrong composition identity")
    model = result.get("model")
    if not isinstance(model, dict) or not model.get("model_id") or not model.get("language") or model.get("protocol") != "document-synthesis-candidates-v1":
        raise ValueError("Invalid composition model")
    if result.get("state") not in _STATES or type(result.get("reviews_current")) is not bool or not _integer(result.get("attempt")) or not _integer(result.get("total_steps"), 65536) or not _integer(result.get("completed_steps"), result["total_steps"]):
        raise ValueError("Invalid composition progress")
    if result["state"] == "awaiting_review" and result["completed_steps"] != result["total_steps"]:
        raise ValueError("Unfinished composition")
    return result


class DocumentCompositions:
    def __init__(self, synthesis):
        self._synthesis = synthesis
        self._client = synthesis._client

    def create(self, request_key, analysis_id, model, language):
        if any(not isinstance(v, str) or not v for v in (request_key, analysis_id, model, language)) or len(request_key) > 128 or len(analysis_id) > 128:
            raise ValueError("Provide request_key, analysis_id, local model and language")
        result = _status(self._client._request("POST", _BASE, dict(request_key=request_key, analysis_id=analysis_id, model=model, language=language), retry=False))
        if result["analysis_id"] != analysis_id or result["model"]["model_id"] != model or result["model"]["language"] != language:
            raise ValueError("Unconfirmed composition; inspect history before retrying")
        return result

    def get(self, composition_id):
        if not isinstance(composition_id, str) or not 0 < len(composition_id) <= 128:
            raise ValueError("Invalid composition ID")
        return _status(self._client._request("GET", _BASE + "?" + urlencode({"id": composition_id})), composition_id)

    def history(self, analysis_id, offset=0):
        if not analysis_id or not _integer(offset, 256):
            raise ValueError("Invalid history request")
        result = self._client._request("GET", _BASE + "?" + urlencode({"analysis_id": analysis_id, "offset": offset}))
        if not isinstance(result, dict) or result.get("analysis_id") != analysis_id or not isinstance(result.get("items"), list) or len(result["items"]) > 20 or result.get("next_offset") not in (None, offset + 20):
            raise ValueError("Invalid composition history")
        for item in result["items"]:
            if _status(item)["analysis_id"] != analysis_id:
                raise ValueError("Wrong composition history parent")
        return result

    def control(self, composition_id, action):
        if not composition_id or action not in ("cancel", "retry"):
            raise ValueError("Invalid composition action")
        result = self._client._request("POST", _BASE + "/" + action, {"id": composition_id}, retry=False)
        if not isinstance(result, dict) or result.get("accepted") is not True:
            raise ValueError("Unconfirmed composition action")
        return result

    def report(self, composition_id):
        job = self.get(composition_id)
        parent = self._synthesis.get(job["analysis_id"])
        result = self._client._request("GET", _BASE + "/report?" + urlencode({"id": composition_id}))
        if not isinstance(result, dict):
            raise ValueError("Invalid composition report")
        status = _status(result.get("composition"), composition_id)
        if status["state"] != "awaiting_review" or status["plan_id"] != job["plan_id"] or status["analysis_id"] != parent["id"] or status["source_plan_id"] != parent["plan"]["id"] or result.get("semantic_review_required") is not True or result.get("source_units") != len(parent["plan"]["units"]) or not _integer(result.get("unrepresented_batches"), parent["completed_batches"]):
            raise ValueError("Invalid report coverage")
        sections = result.get("sections")
        if not isinstance(sections, list) or len(sections) > 4:
            raise ValueError("Invalid report sections")
        for section in sections:
            if not isinstance(section, dict) or not isinstance(section.get("text"), str) or not 0 < len(section["text"].encode("utf-8")) <= 2048 or not _integer(section.get("contributors"), 65536) or section["contributors"] == 0 or section.get("verification") != "source_identity_only":
                raise ValueError("Invalid composed section")
        for key in ("gaps", "extraction_gaps", "candidate_gaps"):
            gaps = result.get(key)
            if not isinstance(gaps, list) or len(gaps) > 256 or any(not isinstance(g, dict) or not g.get("code") or not _integer(g.get("count")) or g["count"] == 0 for g in gaps):
                raise ValueError("Invalid report gaps")
        return result

    def evidence(self, composition_id, section, offset=0):
        if not _integer(section, 3) or not _integer(offset, 65536):
            raise ValueError("Invalid evidence page")
        report = self.report(composition_id)
        if section >= len(report["sections"]):
            raise ValueError("Missing section")
        result = self._client._request("GET", _BASE + "/evidence?" + urlencode({"id": composition_id, "section": section, "offset": offset}))
        if not isinstance(result, dict) or result.get("composition_id") != composition_id or result.get("plan_id") != report["composition"]["plan_id"] or result.get("section") != section or result.get("offset") != offset or result.get("total") != report["sections"][section]["contributors"] or not isinstance(result.get("items"), list) or len(result["items"]) > 8:
            raise ValueError("Invalid evidence identity")
        end = offset + len(result["items"])
        if end > result["total"] or (end < result["total"] and (end == offset or result.get("next_offset") != end)) or (end == result["total"] and result.get("next_offset") is not None):
            raise ValueError("Invalid evidence pagination")
        # Read the pinned original receipt through the existing quote validator.
        for item in result["items"]:
            leaf = item.get("input", {})
            if not _integer(leaf.get("batch")) or not _integer(leaf.get("candidate_index"), 31) or not _integer(leaf.get("review_revision"), 256) or leaf.get("decision") not in ("keep", "unreviewed") or type(leaf.get("section_checked")) is not bool or (leaf["section_checked"] and leaf["decision"] == "unreviewed"):
                raise ValueError("Invalid evidence review")
            page = self._synthesis.results(report["composition"]["analysis_id"], leaf["batch"])
            receipts = page["receipts"]
            if not receipts or receipts[0]["batch_id"] != leaf.get("batch_id") or leaf["candidate_index"] >= len(receipts[0]["result"]["candidates"]) or item.get("candidate") != receipts[0]["result"]["candidates"][leaf["candidate_index"]]:
                raise ValueError("Composition evidence differs from its original receipt")
        return result

    def export(self, composition_id, format="json"):
        if not composition_id or format not in ("json", "csv", "markdown"):
            raise ValueError("Invalid composition export")
        result = self._client._request("GET", _BASE + "/export?" + urlencode({"id": composition_id, "format": format}), retry=False)
        if not isinstance(result, dict) or result.get("analysis_id") != composition_id or result.get("format") != format or not isinstance(result.get("content"), str):
            raise ValueError("Invalid composition export identity")
        raw = result["content"].encode("utf-8")
        if not 0 < len(raw) <= 8 * 1024 * 1024 or sha256(raw).hexdigest() != result.get("sha256"):
            raise ValueError("Invalid composition export digest")
        return result
