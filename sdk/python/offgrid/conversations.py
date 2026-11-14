"""Canonical v2 conversation commands. Observation never resumes execution."""

from urllib.parse import quote, urlencode


class Conversations:
    def __init__(self, client):
        self._client = client

    def list(self, after=""):
        return self._client._request(
            "GET", "/api/v2/conversations?" + urlencode({"after": after})
        )

    def get(self, conversation_id):
        return self._client._request("GET", self._path(conversation_id))

    def inputs(self, conversation_id, after=0, limit=100):
        return self._client._request("GET", self._path(conversation_id) + "/inputs?"
            + urlencode({"after": after, "limit": limit}))

    def input(self, conversation_id, input_id):
        return self._client._request("GET", self._path(conversation_id) + "/inputs/"
            + quote(input_id, safe=""))

    def submit_input(self, request_id, conversation_id, expected_revision, payload):
        return self._client._request("POST", self._path(conversation_id) + "/inputs", {
            "request_id": request_id, "target_id": conversation_id,
            "expected_revision": expected_revision, "kind": "submit_input", "payload": payload,
        }, retry=False)

    def input_command(self, request_id, conversation_id, input_id, expected_revision, kind, payload):
        return self._client._request("POST", self._path(conversation_id) + "/inputs/"
            + quote(input_id, safe="") + "/commands", {
                "request_id": request_id, "target_id": input_id,
                "expected_revision": expected_revision, "kind": kind, "payload": payload,
            }, retry=False)

    def user_preferences(self):
        return self._client._request("GET", "/api/v2/user-preferences")

    def configure_user_preferences(self, request_id, expected_revision, preferences):
        return self._client._request("POST", "/api/v2/user-preferences/commands", {
            "request_id": request_id, "target_id": "user-defaults",
            "expected_revision": expected_revision, "kind": "configure_user_preferences",
            "payload": preferences,
        }, retry=False)

    def worker(self, worker_id):
        return self._client._request("GET", "/api/v2/workers/" + quote(worker_id, safe=""))

    def result(self, reference, offset=0, limit=4096):
        return self._client._request("GET", "/api/v2/results/"
            + quote(reference, safe="") + "?" + urlencode({"offset": offset, "limit": limit}))

    def work_changes(self, work_id, after="", limit=32):
        return self._client._request("GET", "/api/v2/work-items/"
            + quote(work_id, safe="") + "/changes?"
            + urlencode({"after": after, "limit": limit}))

    def work_change(self, work_id, change_id, view="files", offset=0, limit=32, digest="", artifact=""):
        query = {"view": view, "offset": offset, "limit": limit, "hash": digest}
        if artifact:
            query["artifact"] = artifact
        return self._client._request("GET", "/api/v2/work-items/"
            + quote(work_id, safe="") + "/changes/" + quote(change_id, safe="")
            + "?" + urlencode(query))

    def worker_messages(self, worker_id, after=0, limit=100):
        return self._client._request("GET", "/api/v2/workers/"
            + quote(worker_id, safe="") + "/messages?"
            + urlencode({"after": after, "limit": limit}))

    def worker_command(self, request_id, target_id, expected_revision, kind, payload):
        return self._client._request("POST", "/api/v2/workers/"
            + quote(target_id, safe="") + "/commands", {
                "request_id": request_id, "target_id": target_id,
                "expected_revision": expected_revision, "kind": kind, "payload": payload,
            }, retry=False)

    def project(self, project_id, revision=None):
        path = "/api/v2/projects/" + quote(project_id, safe="")
        if revision is not None:
            path += "?" + urlencode({"revision": revision})
        return self._client._request("GET", path)

    def work_command(self, request_id, target_id, expected_revision, kind, payload):
        return self._client._request("POST", "/api/v2/work-items/"
            + quote(target_id, safe="") + "/commands", {
                "request_id": request_id, "target_id": target_id,
                "expected_revision": expected_revision, "kind": kind, "payload": payload,
            }, retry=False)

    def project_command(self, request_id, target_id, expected_revision, kind, payload):
        path = "/api/v2/projects"
        if target_id:
            path += "/" + quote(target_id, safe="") + "/commands"
        return self._client._request("POST", path, {
            "request_id": request_id, "target_id": target_id,
            "expected_revision": expected_revision, "kind": kind, "payload": payload,
        }, retry=False)

    def configuration(self, conversation_id):
        return self._client._request(
            "GET", self._path(conversation_id) + "/configuration"
        )

    def events(self, conversation_id, after=0, limit=100):
        return self._client._request(
            "GET", self._path(conversation_id) + "/events?"
            + urlencode({"after": after, "limit": limit})
        )

    def command(self, request_id, target_id, expected_revision, kind, payload):
        """Keep the same request ID and payload when retrying an accepted command.

        A conflict requires a fresh snapshot and a deliberate new command.
        Creating a conversation uses target_id="" and expected_revision=0.
        """
        path = self._path(target_id) + "/commands" if target_id else "/api/v2/conversations"
        return self._client._request("POST", path, {
            "request_id": request_id,
            "target_id": target_id,
            "expected_revision": expected_revision,
            "kind": kind,
            "payload": payload,
        }, retry=False)

    @staticmethod
    def _path(conversation_id):
        return "/api/v2/conversations/" + quote(conversation_id, safe="")
