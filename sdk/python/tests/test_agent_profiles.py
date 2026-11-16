import unittest
from offgrid.agent import AgentProfiles


class Fixture:
    def __init__(self):
        self.calls = []
        self.capabilities = []
        self.fail = False

    def _require_capability(self, capability):
        self.capabilities.append(capability)

    def _request(self, *args, **kwargs):
        self.calls.append((args, kwargs))
        if self.fail:
            raise RuntimeError("conflict")
        return {"revision": 3, "jobs": 2, "memory": 1, "token": "a" * 64}


class ProfileDeletionTests(unittest.TestCase):
    def test_review_and_delete_bind_explicit_snapshot_without_retry(self):
        client = Fixture()
        profiles = AgentProfiles(client)
        review = profiles.deletion_review("my profile")
        self.assertEqual(client.calls[0][0], ("GET", "/api/v2/agent-profiles/my%20profile/deletion"))
        client.fail = True
        with self.assertRaises(RuntimeError):
            profiles.delete("my profile", expected_revision=review["revision"], review_token=review["token"])
        self.assertEqual(len(client.calls), 2)
        self.assertEqual(client.calls[-1], (("DELETE", "/api/v2/agent-profiles/my%20profile", {"expected_revision": 3, "review_token": "a" * 64}), {"retry": False}))
        self.assertEqual(client.capabilities, ["agent-profile-deletion-v1"] * 2)

    def test_invalid_review_is_rejected_before_network(self):
        client = Fixture()
        for revision, token in [(0, "a" * 64), (1, "bad"), (1, "g" * 64), (True, "a" * 64), (1.5, "a" * 64)]:
            with self.assertRaises(ValueError):
                AgentProfiles(client).delete("writer", expected_revision=revision, review_token=token)
        self.assertEqual(client.calls, [])


if __name__ == "__main__":
    unittest.main()
