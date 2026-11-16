"""Local document candidate jobs. Citation identity is not semantic approval."""
from hashlib import sha256
import json
from urllib.parse import urlencode


class DocumentSynthesis:
    def __init__(self, client):
        self._client = client

    @property
    def compositions(self):
        from .document_compositions import DocumentCompositions
        return DocumentCompositions(self)

    def review(self, analysis_id, batch, revision=0):
        from .synthesis_review import read
        return read(self, analysis_id, batch, revision)

    def save_review(self, request):
        from .synthesis_review import save
        return save(self, request)

    @staticmethod
    def _job(value, expected_id=None):
        if not isinstance(value, dict) or not value.get("id") or (expected_id and value["id"] != expected_id):
            raise ValueError("Invalid synthesis identity")
        plan, model = value.get("plan", {}), value.get("model", {})
        if plan.get("workflow") not in ("summary", "requirements") or plan.get("version") != "document-analysis-v1" or not isinstance(plan.get("sources"), list) or not 1 <= len(plan["sources"]) <= 16 or not isinstance(plan.get("units"), list) or not plan["units"] or model.get("protocol") != "document-synthesis-candidates-v1":
            raise ValueError("Invalid synthesis plan")
        if value.get("state") not in ("queued", "running", "interrupted", "cancelled", "failed", "awaiting_review"):
            raise ValueError("Invalid synthesis state")
        for key in ("completed_batches", "completed_units", "unit_cursor", "next_byte", "attempt"):
            if type(value.get(key)) is not int or value[key] < 0:
                raise ValueError("Invalid synthesis progress")
        if value["completed_units"] != value["unit_cursor"] or value["unit_cursor"] > len(plan["units"]) or (value["state"] == "awaiting_review") != (value["unit_cursor"] == len(plan["units"])):
            raise ValueError("Inconsistent synthesis coverage")
        return value

    def create(self, *, request_key, workflow, sources, model, language):
        if not request_key or workflow not in ("summary", "requirements") or not isinstance(sources, list) or not 1 <= len(sources) <= 16 or not model or not language:
            raise ValueError("Provide an analysis workflow, sources, local model and language")
        result = self._job(self._client._request("POST", "/v1/document-analyses/synthesis", dict(request_key=request_key, workflow=workflow, sources=sources, model=model, language=language), retry=False))
        if result["plan"]["sources"] != sources or result["plan"]["workflow"] != workflow or result["model"]["model_id"] != model or result["model"]["language"] != language:
            raise ValueError("Unconfirmed synthesis; inspect history before retrying")
        return result

    def get(self, analysis_id):
        if not analysis_id:
            raise ValueError("Analysis ID is required")
        return self._job(self._client._request("GET", "/v1/document-analyses/synthesis?" + urlencode({"id": analysis_id})), analysis_id)

    def history(self, offset=0):
        if type(offset) is not int or not 0 <= offset <= 256:
            raise ValueError("Invalid history offset")
        result = self._client._request("GET", "/v1/document-analyses/synthesis?" + urlencode({"offset": offset}))
        if not isinstance(result, dict) or not isinstance(result.get("analyses"), list) or len(result["analyses"]) > 20 or result.get("next_offset") not in (None, offset + 20):
            raise ValueError("Invalid synthesis history")
        return result

    def control(self, analysis_id, action):
        if not analysis_id or action not in ("cancel", "retry"):
            raise ValueError("Invalid synthesis action")
        path = "/v1/document-analyses/cancel" if action == "cancel" else "/v1/document-analyses/synthesis/retry"
        result = self._client._request("POST", path, {"id": analysis_id}, retry=False)
        if not isinstance(result, dict) or result.get("accepted") is not True:
            raise ValueError("Unconfirmed synthesis action")
        return result

    def export(self, analysis_id, format="json"):
        """Export draft candidates and coverage, not a linguistically approved answer."""
        if not analysis_id or len(analysis_id) > 128 or format not in ("json", "csv", "markdown"):
            raise ValueError("Provide an analysis ID and json, csv or markdown format")
        query = urlencode({"id": analysis_id, "format": format})
        result = self._client._request("GET", "/v1/document-analyses/synthesis/export?" + query, retry=False)
        if not isinstance(result, dict) or result.get("analysis_id") != analysis_id or result.get("format") != format or not isinstance(result.get("content"), str):
            raise ValueError("Invalid synthesis export identity")
        raw = result["content"].encode("utf-8")
        if not 0 < len(raw) <= 8 * 1024 * 1024 or sha256(raw).hexdigest() != result.get("sha256"):
            raise ValueError("Invalid synthesis export size or digest")
        return result

    def results(self, analysis_id, offset=0):
        if type(offset) is not int or not 0 <= offset <= 100000:
            raise ValueError("Invalid result offset")
        job = self.get(analysis_id)
        result = self._client._request("GET", "/v1/document-analyses/synthesis/results?" + urlencode({"id": analysis_id, "offset": offset}))
        if not isinstance(result, dict) or result.get("analysis_id") != analysis_id or result.get("offset") != offset or not isinstance(result.get("receipts"), list) or len(result["receipts"]) > 2 or type(result.get("total_batches")) is not int:
            raise ValueError("Invalid synthesis result page")
        end = offset + len(result["receipts"])
        if not end <= result["total_batches"] <= 100000 or result.get("next_offset") != (end if end < result["total_batches"] else None) or end == offset < result["total_batches"]:
            raise ValueError("Invalid synthesis pagination")
        for receipt in result["receipts"]:
            if receipt.get("validation", "source-values-v2") != "source-values-v2":
                raise ValueError("Unsupported synthesis validation revision")
            data = receipt.get("result", {})
            if data.get("batch_id") != receipt.get("batch_id") or not isinstance(data.get("candidates"), list) or len(data["candidates"]) > 32 or receipt.get("method") == "model" and data.get("semantic_review_required") is not True:
                raise ValueError("Invalid synthesis receipt")
            model = job["model"]
            runtime = {"name": model["runtime"]["name"], "revision": model["runtime"]["revision"]}
            if model["runtime"].get("sha256"):
                runtime["sha256"] = model["runtime"]["sha256"]
            identity = {"model_id": model["model_id"], "model_sha256": model["model_sha256"], "runtime": runtime, "language": model["language"], "context_tokens": model["context_tokens"], "response_tokens": model["response_tokens"], "protocol": model["protocol"]}
            encoded = json.dumps(identity, ensure_ascii=False, separators=(",", ":"))
            for char in "<>&\u2028\u2029":
                encoded = encoded.replace(char, "\\u%04x" % ord(char))
            if receipt.get("model_identity") != "synthesis-model-" + sha256(encoded.encode("utf-8")).hexdigest():
                raise ValueError("Invalid synthesis model identity")
            ordinal, primary = receipt.get("unit_ordinal"), receipt.get("primary", {})
            if type(ordinal) is not int or not 0 <= ordinal < len(job["plan"]["units"]):
                raise ValueError("Invalid synthesis source unit")
            unit = job["plan"]["units"][ordinal]["span"]
            start, end = primary.get("start_byte"), primary.get("end_byte")
            if type(start) is not int or type(end) is not int or not unit["start_byte"] <= start <= end <= unit["end_byte"] or receipt.get("end_of_unit") != (end == unit["end_byte"]):
                raise ValueError("Invalid synthesis source range")
            method, tokens = receipt.get("method"), receipt.get("prompt_tokens")
            if type(tokens) is not int or method not in ("model", "empty_source") or (method == "model" and not 1 <= tokens <= model["context_tokens"]-model["response_tokens"]-256) or (method == "empty_source" and (tokens != 0 or data["candidates"] or data.get("no_findings_reported"))):
                raise ValueError("Invalid synthesis execution receipt")
            for candidate in data["candidates"]:
                if candidate.get("verification") != "source_identity_only" or not candidate.get("evidence"):
                    raise ValueError("Invalid synthesis candidate")
                for field in ("evidence", "conditions", "exceptions"):
                    quotes = candidate.get(field)
                    if not isinstance(quotes, list) or len(quotes) > 8:
                        raise ValueError("Invalid synthesis evidence")
                    for quote in quotes:
                        self._quote(quote, job)
        return result

    @staticmethod
    def _quote(quote, job):
        c = quote.get("citation", {})
        source, span, text = c.get("source"), c.get("span", {}), quote.get("text")
        if type(source) is not int or not 0 <= source < len(job["plan"]["sources"]) or c.get("revision") != job["plan"]["sources"][source]["revision"] or not isinstance(text, str) or not text:
            raise ValueError("Invalid source quote identity")
        raw = text.encode("utf-8")
        start, end = span.get("start_byte"), span.get("end_byte")
        if type(start) is not int or type(end) is not int or len(raw) != end-start or len(raw) > 8192 or sha256(raw).hexdigest() != c.get("text_sha256"):
            raise ValueError("Invalid source quote digest")
        if not any(u["source"] == source and u["unit_id"] == c.get("unit_id") and u["span"]["start_byte"] <= start < end <= u["span"]["end_byte"] for u in job["plan"]["units"]):
            raise ValueError("Invalid source quote range")
