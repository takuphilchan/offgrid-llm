import unittest
from offgrid.collections import Collections


class Fixture:
    def __init__(self, response):
        self.response = response
        self.calls = []

    def _request(self, *args, **kwargs):
        self.calls.append((args, kwargs))
        return self.response


class CollectionTests(unittest.TestCase):
    def test_stale_audience_receipt_is_not_replayed(self):
        source = dict(id="private", version=1, revision=1, name="Records", owner="owner", readers=[], editors=[])
        client = Fixture(source)
        with self.assertRaises(ValueError):
            Collections(client).update("private", expected_revision=1, name="Records", readers=[], editors=[])
        self.assertEqual(len(client.calls), 1)
        self.assertEqual(client.calls[0][1], dict(retry=False))

    def test_explicit_membership_is_required_before_request(self):
        client = Fixture({})
        with self.assertRaises(ValueError):
            Collections(client).update("private", expected_revision=1, name="Records", readers=None, editors=[])
        self.assertEqual(client.calls, [])

    def test_creation_cannot_claim_a_shared_result_is_private(self):
        client = Fixture(dict(id="workspace", version=1, revision=1, name="Records", readers=[], editors=[]))
        with self.assertRaises(ValueError):
            Collections(client).create("Records")
        self.assertEqual(len(client.calls), 1)


if __name__ == "__main__":
    unittest.main()
