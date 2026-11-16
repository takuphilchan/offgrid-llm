"""Pinned human annotations; never change the original model receipt."""
from datetime import datetime
from urllib.parse import urlencode


def validate(value, analysis_id, batch, receipt):
    if not isinstance(value, dict) or value.get("analysis_id") != analysis_id or value.get("batch") != batch or value.get("batch_id") != receipt["batch_id"] or value.get("response_sha256") != receipt["response_sha256"]:
        raise ValueError("Invalid review identity")
    revision = value.get("revision")
    decisions = value.get("decisions")
    if type(revision) is not int or not 0 <= revision <= 256 or type(value.get("section_checked")) is not bool or not isinstance(decisions, list) or len(decisions) != len(receipt["result"]["candidates"]):
        raise ValueError("Invalid review revision or coverage")
    for item in decisions:
        if not isinstance(item, dict) or item.get("decision") not in ("unreviewed", "keep", "exclude") or not isinstance(item.get("note"), str) or len(item["note"].encode("utf-8")) > 2048 or (value["section_checked"] and item["decision"] == "unreviewed"):
            raise ValueError("Invalid review decision")
    if revision == 0:
        if value.get("author") != "" or value.get("updated_at") != "" or value["section_checked"] or any(d["decision"] != "unreviewed" or d["note"] for d in decisions):
            raise ValueError("Invalid unsaved review")
    else:
        if not isinstance(value.get("author"), str) or not value["author"] or not isinstance(value.get("updated_at"), str):
            raise ValueError("Missing review attribution")
        datetime.fromisoformat(value["updated_at"].replace("Z", "+00:00"))
    return value


def read(synthesis, analysis_id, batch, revision=0):
    if type(revision) is not int or not 0 <= revision <= 256:
        raise ValueError("Invalid review revision")
    page = synthesis.results(analysis_id, batch)
    if not page["receipts"]:
        raise ValueError("Missing synthesis section")
    value = synthesis._client._request("GET", "/v1/document-analyses/synthesis/review?" + urlencode({"id": analysis_id, "batch": batch, "revision": revision}))
    if revision and (not isinstance(value, dict) or value.get("revision") != revision):
        raise ValueError("Incorrect review revision")
    return validate(value, analysis_id, batch, page["receipts"][0])


def save(synthesis, request):
    if not isinstance(request, dict) or type(request.get("expected_revision")) is not int or not 0 <= request["expected_revision"] < 256:
        raise ValueError("Expected review revision is required")
    page = synthesis.results(request.get("id"), request.get("batch"))
    if not page["receipts"]:
        raise ValueError("Missing synthesis section")
    receipt = page["receipts"][0]
    if request.get("batch_id") != receipt["batch_id"] or request.get("response_sha256") != receipt["response_sha256"]:
        raise ValueError("Review does not match section")
    result = synthesis._client._request("POST", "/v1/document-analyses/synthesis/review", request, retry=False)
    validate(result, request["id"], request["batch"], receipt)
    if result["revision"] != request["expected_revision"] + 1 or result["decisions"] != request.get("decisions") or result["section_checked"] != request.get("section_checked"):
        raise ValueError("Unconfirmed review; read the saved review before retrying")
    return result
