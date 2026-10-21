"""
Tests for OffGrid Python Client

Run with: pytest tests/
"""

import pytest
from unittest.mock import Mock, patch, MagicMock
import json
import socket
import urllib.request

# Import the library
import sys
sys.path.insert(0, '..')
from offgrid import Client, OffGridError
from offgrid.client import Client as ClientClass
import offgrid.client as client_module


@pytest.fixture(autouse=True)
def isolated_client_transport(monkeypatch):
    """Keep unit tests off installed services, including the pooled HTTP path."""
    def reject_network(*args, **kwargs):
        raise AssertionError("Unit tests must not access a real network service")

    monkeypatch.setattr(socket.socket, "connect", reject_network)
    monkeypatch.setattr(socket, "create_connection", reject_network)
    opener = Mock()
    # Resolve the decorated mock at request time, not fixture construction time.
    opener.open.side_effect = lambda *args, **kwargs: client_module.urlopen(*args, **kwargs)
    monkeypatch.setattr(urllib.request, "build_opener", lambda *args, **kwargs: opener)


class TestClient:
    """Test Client class."""

    @patch('offgrid.client.urlopen')
    def test_agent_preserves_additive_verification(self, send):
        client = Client(api_key="fixture-key")
        for verification in ({"version": 1, "state": "partial", "checks": []},
                             {"version": 2, "state": "pending", "checks": [{
                                 "expectation": {"id": "edit", "kind": "document_edit_matches", "document": "tab", "snapshot": "before", "edit_mode": "insert_text", "text": "café 日本語"},
                                 "binding": {"session": "host", "document": {"id": "tab", "generation": "g", "container": "c", "editor": "e", "revision": "r", "snapshot": "before", "action_id": "one-input"}},
                                 "state": "pending"}]},
                             {"version": 99, "future_field": "preserve"}):
            snapshot = {"run_id": "run-fixture", "status": "completed",
                        "output": "answer", "verification": verification}
            response = MagicMock()
            response.read.return_value = json.dumps(snapshot).encode()
            send.return_value = response
            assert client.agent.run("fixture", model="fixture") == snapshot
            request = send.call_args[0][0]
            assert request.get_header("Authorization") == "Bearer fixture-key"
            assert request.full_url.endswith("/v1/agents/run")
    
    def test_client_init(self):
        """Test client initialization with defaults."""
        client = Client()
        assert client.host == "http://localhost:11611"
        assert client.base_url == "http://localhost:11611"
    
    def test_client_custom_config(self):
        """Test client with custom configuration."""
        client = Client(host="http://192.168.1.100:8080", timeout=60)
        assert client.host == "http://192.168.1.100:8080"
        assert client.timeout == 60
        assert client.base_url == "http://192.168.1.100:8080"
    
    @patch('offgrid.client.urlopen')
    def test_list_models(self, mock_urlopen):
        """Test listing models."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "data": [
                {"id": "model1", "size": 1000000},
                {"id": "model2", "size": 2000000}
            ]
        }).encode()
        mock_urlopen.return_value = mock_response
        
        client = Client()
        models = client.list_models()
        
        assert len(models) == 2
        assert models[0]["id"] == "model1"
        assert models[1]["id"] == "model2"
    
    @patch('offgrid.client.urlopen')
    def test_chat(self, mock_urlopen):
        """Test chat completion."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "choices": [{
                "message": {
                    "role": "assistant",
                    "content": "Hello! How can I help you?"
                }
            }]
        }).encode()
        mock_urlopen.return_value = mock_response
        
        client = Client()
        client._default_model = "test-model"  # Skip model lookup
        
        response = client.chat("Hello!")
        assert response == "Hello! How can I help you?"
    
    @patch('offgrid.client.urlopen')
    def test_complete(self, mock_urlopen):
        """Test text completion."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "choices": [{
                "text": "is a programming language."
            }]
        }).encode()
        mock_urlopen.return_value = mock_response
        
        client = Client()
        client._default_model = "test-model"
        
        response = client.complete("Python")
        assert response == "is a programming language."
    
    @patch('offgrid.client.urlopen')
    def test_embed(self, mock_urlopen):
        """Test embeddings."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "data": [
                {"embedding": [0.1, 0.2, 0.3]}
            ]
        }).encode()
        mock_urlopen.return_value = mock_response
        
        client = Client()
        client._default_model = "test-model"
        
        embedding = client.embed("Hello")
        assert embedding == [0.1, 0.2, 0.3]
    
    @patch('offgrid.client.urlopen')
    def test_embed_multiple(self, mock_urlopen):
        """Test multiple embeddings."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "data": [
                {"embedding": [0.1, 0.2]},
                {"embedding": [0.3, 0.4]}
            ]
        }).encode()
        mock_urlopen.return_value = mock_response
        
        client = Client()
        client._default_model = "test-model"
        
        embeddings = client.embed(["Hello", "World"])
        assert len(embeddings) == 2
        assert embeddings[0] == [0.1, 0.2]
        assert embeddings[1] == [0.3, 0.4]
    
    @patch('offgrid.client.urlopen')
    def test_health_check(self, mock_urlopen):
        """Test health check."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "status": "healthy"
        }).encode()
        mock_urlopen.return_value = mock_response
        
        client = Client()
        assert client.health() == True
    
    @patch('offgrid.client.urlopen')
    def test_info(self, mock_urlopen):
        """Test system info."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "status": "healthy",
            "uptime": "1h 30m",
            "system": {
                "cpu_percent": 25.5,
                "memory_percent": 60.0
            }
        }).encode()
        mock_urlopen.return_value = mock_response
        
        client = Client()
        info = client.info()
        
        assert info["status"] == "healthy"
        assert info["uptime"] == "1h 30m"
        assert info["system"]["cpu_percent"] == 25.5


class TestModelManager:
    """Test ModelManager class."""

    @patch('offgrid.client.urlopen')
    def test_package_operations_use_authenticated_existing_transport(self, send):
        from urllib.parse import parse_qs, urlsplit
        response = MagicMock()
        response.read.return_value = b'{"models": [], "id": "op", "state": "complete"}'
        send.return_value = response
        client = Client(api_key="fixture-key")
        assert client.models.inventory("speech_recognition") == []
        req = send.call_args[0][0]
        assert req.get_header("Authorization") == "Bearer fixture-key"
        assert parse_qs(urlsplit(req.full_url).query)["category"] == ["speech_recognition"]
        client.models.install_package("preview-1", request_id="stable-request")
        first = send.call_args[0][0].data
        client.models.install_package("preview-1", request_id="stable-request")
        assert send.call_args[0][0].data == first
        client.models.control_model_operation("op", "cancel")
        assert send.call_args[0][0].full_url.endswith("/op/cancel")
        client.models.remove_package("fixture", "rev-1")
        assert send.call_args[0][0].full_url.endswith("/fixture/rev-1/remove")

    @patch('offgrid.client.urlopen')
    def test_native_provider_inventory_is_metadata_only_and_authenticated(self, send):
        response = MagicMock()
        response.read.return_value = b'{"models":[{"id":"offline-mlx","engine":"mlx","format":"mlx_safetensors","backend":"metal","readiness":{"installed":true,"runtime_ready":false,"qualified":false}}]}'
        send.return_value = response
        client = Client(api_key="fixture-key")
        model = client.models.inventory("language")[0]
        assert (model["engine"], model["format"], model["backend"]) == ("mlx", "mlx_safetensors", "metal")
        assert model["readiness"]["qualified"] is False
        request = send.call_args[0][0]
        assert request.get_header("Authorization") == "Bearer fixture-key"
        assert request.full_url.endswith("/api/v2/models?category=language")

    def test_package_validation_and_polling_never_resubmit(self):
        client = Client()
        client._request = Mock(side_effect=[{"state": "downloading"}, {"state": "complete"}])
        with patch("offgrid.models.time.sleep"):
            assert client.models.wait_for_model_operation("op")["state"] == "complete"
        assert all(call.args[0] == "GET" for call in client._request.call_args_list)
        with pytest.raises(ValueError):
            client.models.control_model_operation("op", "execute")
        with pytest.raises(ValueError):
            client.models.preview_package(repository="org/repo")
        with pytest.raises(ValueError):
            client.models.preview_package("id", repository="org/repo")
        client._request = Mock(return_value={"state": "interrupted", "message": "Resume explicitly", "error_code": "recovery_required"})
        with pytest.raises(OffGridError) as error:
            client.models.wait_for_model_operation("op")
        assert error.value.code == "recovery_required"
        client._request = Mock(side_effect=[KeyboardInterrupt(), {"state": "cancelling"}])
        with pytest.raises(KeyboardInterrupt):
            client.models.wait_for_model_operation("op")
        assert client._request.call_args.args[1].endswith("/cancel")

    @patch('offgrid.client.urlopen')
    def test_package_http_conflicts_are_not_retried(self, send):
        from urllib.error import HTTPError
        from io import BytesIO
        send.side_effect = HTTPError("https://fixture.invalid", 409, "conflict", {}, BytesIO(b'{"error":{"message":"Changed source", "code":"request_conflict"}}'))
        with pytest.raises(OffGridError) as error:
            Client(api_key="fixture").models.install_package("preview", request_id="request-id")
        assert error.value.code == "request_conflict"
        assert send.call_count == 1
    
    @patch('offgrid.client.urlopen')
    def test_search(self, mock_urlopen):
        """Test model search."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "results": [
                {"id": "org/llama-7b", "size_gb": "4.0"},
                {"id": "org/llama-13b", "size_gb": "8.0"}
            ]
        }).encode()
        mock_urlopen.return_value = mock_response
        
        client = Client()
        results = client.models.search("llama")
        
        assert len(results) == 2
        assert results[0]["id"] == "org/llama-7b"
    
    @patch('offgrid.client.urlopen')
    def test_search_with_ram_filter(self, mock_urlopen):
        """Test model search with RAM filter."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "results": [
                {"id": "small-model", "size_gb": "2.0"},
                {"id": "large-model", "size_gb": "10.0"}
            ]
        }).encode()
        mock_urlopen.return_value = mock_response
        
        client = Client()
        results = client.models.search("llama", ram=4)
        
        # Only small-model should pass (2.0 * 1.2 = 2.4 < 4)
        assert len(results) == 1
        assert results[0]["id"] == "small-model"


class TestKnowledgeBase:
    """Test KnowledgeBase class."""
    
    @patch('offgrid.client.urlopen')
    def test_status(self, mock_urlopen):
        """Test KB status."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "enabled": True,
            "documents": 5
        }).encode()
        mock_urlopen.return_value = mock_response
        
        client = Client()
        status = client.kb.status()
        
        assert status["enabled"] == True
        assert status["documents"] == 5
    
    @patch('offgrid.client.urlopen')
    def test_search(self, mock_urlopen):
        """Test KB search."""
        mock_response = MagicMock()
        mock_response.read.return_value = json.dumps({
            "results": [
                {
                    "score": 0.95,
                    "document_name": "notes.md",
                    "chunk": {"content": "Some relevant content"}
                }
            ]
        }).encode()
        mock_urlopen.return_value = mock_response
        
        client = Client()
        results = client.kb.search("query")
        
        assert len(results) == 1
        assert results[0]["score"] == 0.95
        assert results[0]["content"] == "Some relevant content"


class TestAgentConversations:
    @patch('offgrid.client.urlopen')
    @pytest.mark.parametrize('status', ['running', 'waiting_for_input', 'waiting_for_approval', 'interrupted', 'uncertain', 'failed', 'completed'])
    def test_submit_preserves_canonical_state_without_model_lookup(self, send, status):
        client = Client(api_key='fixture-key')
        saved = {'turn': {'id': 'turn-request-1', 'job_id': 'run-one'},
                 'job': {'run_id': 'run-one', 'status': status, 'future_evidence': {'value': 1}}}
        response = MagicMock()
        response.read.return_value = json.dumps(saved).encode()
        identity = MagicMock()
        identity.read.return_value = b'{"product":"offgrid","capabilities":["agent-conversations-v1"]}'
        send.side_effect = [identity, response]
        selection = {'tools': [], 'knowledge': [], 'computer': False}
        result = client.sessions.submit_agent_turn('café ? notes', 'Hello', request_id='turn-request-1',
                                                  profile_id='profile-one', profile_revision=3,
                                                  restrictions=selection)
        assert result == saved
        assert send.call_count == 2
        request = send.call_args[0][0]
        assert request.full_url.endswith('/v1/sessions/caf%C3%A9%20%3F%20notes/generate')
        assert request.get_header('Authorization') == 'Bearer fixture-key'
        assert json.loads(request.data) == {'content': 'Hello', 'request_id': 'turn-request-1',
                                           'agent': {'id': 'profile-one', 'revision': 3},
                                           'restrictions': selection}

    @patch('offgrid.client.urlopen')
    def test_profiles_revision_requests_and_read_only_inspection(self, send):
        client = Client(api_key='fixture-key')
        response = MagicMock()
        response.read.return_value = b'{"profiles":[],"revision":3}'
        send.return_value = response
        settings = {'name': 'Research', 'model': 'installed', 'max_iterations': 8}
        operations = [
            (lambda: client.agent.profiles.list(), 'GET', '/api/v2/agent-profiles', None),
            (lambda: client.agent.profiles.get('profile-one', revision=2), 'GET', '/api/v2/agent-profiles/profile-one?revision=2', None),
            (lambda: client.agent.profiles.effective('profile-one', revision=2), 'GET', '/api/v2/agent-profiles/profile-one/effective?revision=2', None),
            (lambda: client.agent.profiles.resources(), 'GET', '/api/v2/agent-profiles/resources', None),
            (lambda: client.agent.profiles.create(settings), 'POST', '/api/v2/agent-profiles', {'settings': settings}),
            (lambda: client.agent.profiles.update('profile-one', settings, expected_revision=2), 'PUT', '/api/v2/agent-profiles/profile-one', {'settings': settings, 'expected_revision': 2}),
            (lambda: client.agent.profiles.archive('profile-one', expected_revision=3), 'POST', '/api/v2/agent-profiles/profile-one/archive', {'expected_revision': 3}),
            (lambda: client.agent.job('run-one'), 'GET', '/api/v2/jobs/run-one', None),
            (lambda: client.sessions.turn('chat'), 'GET', '/v1/sessions/chat/turn', None),
            (lambda: client.sessions.recover_agent_turn('chat', 'turn-request-1'), 'POST', '/v1/sessions/chat/turn/recover', {'id': 'turn-request-1'}),
            (lambda: client.agent.steer('run-one', 'Only summarize', request_id='correction-1'), 'POST', '/api/v2/jobs/run-one/steer', {'request_id': 'correction-1', 'instruction': 'Only summarize'}),
            (lambda: client.agent.stop('run-one'), 'POST', '/api/v2/jobs/run-one/cancel', {}),
            (lambda: client.sessions.cancel_turn('chat', 'turn-request-1'), 'POST', '/v1/sessions/chat/turn/cancel', {'id': 'turn-request-1'}),
            (lambda: client.sessions.delete('chat', work='retain'), 'DELETE', '/v1/sessions/chat?work=retain', None),
        ]
        for call, method, path, body in operations:
            send.reset_mock()
            call()
            assert send.call_count == 1
            request = send.call_args[0][0]
            assert request.method == method
            assert request.full_url.endswith(path)
            assert (json.loads(request.data) if request.data is not None else None) == body
        with pytest.raises(ValueError):
            client.sessions.delete('chat', work='discard-uncertain')

    @patch('offgrid.client.urlopen')
    @pytest.mark.parametrize('operation', ['submit', 'job', 'profile', 'recover', 'steer', 'stop'])
    def test_lost_acknowledgment_does_not_retry_mutations(self, send, operation):
        client = Client(max_retries=4, retry_delay=0)
        send.side_effect = client_module.URLError('lost acknowledgment')
        if operation in ('submit', 'job'):
            identity = MagicMock()
            identity.read.return_value = b'{"product":"offgrid","capabilities":["agent-conversations-v1","agent-profiles-v1"]}'
            send.side_effect = [identity, client_module.URLError('lost acknowledgment')]
        calls = {
            'submit': lambda: client.sessions.submit_agent_turn('chat', 'Hello', request_id='turn-request-1', profile_id='profile-one', profile_revision=1),
            'job': lambda: client.agent.submit('Hello', request_id='job-request-1', profile_id='profile-one', profile_revision=1),
            'profile': lambda: client.agent.profiles.create({'name': 'A', 'model': 'installed', 'max_iterations': 5}),
            'recover': lambda: client.sessions.recover_agent_turn('chat', 'turn-request-1'),
            'steer': lambda: client.agent.steer('run-one', 'Stop editing', request_id='correction-1'),
            'stop': lambda: client.agent.stop('run-one'),
        }
        with pytest.raises(OffGridError):
            calls[operation]()
        assert send.call_count == (2 if operation in ('submit', 'job') else 1)

    @patch('offgrid.client.urlopen')
    def test_old_service_cannot_silently_run_agent_turn_as_plain_chat(self, send):
        response = MagicMock()
        response.read.return_value = b'{"product":"offgrid","capabilities":[]}'
        send.return_value = response
        client = Client()
        with pytest.raises(OffGridError) as error:
            client.sessions.submit_agent_turn('chat', 'Hello', request_id='same-turn', profile_id='writer', profile_revision=1)
        assert error.value.code == 'upgrade_required'
        assert send.call_count == 1
        assert send.call_args[0][0].method == 'GET'

    @patch('offgrid.client.urlopen')
    def test_old_service_cannot_ignore_pinned_profile_for_job(self, send):
        response = MagicMock()
        response.read.return_value = b'{"product":"offgrid","capabilities":[]}'
        send.return_value = response
        with pytest.raises(OffGridError) as error:
            Client().agent.submit('Hello', request_id='same-job', profile_id='writer', profile_revision=1)
        assert error.value.code == 'upgrade_required'
        assert send.call_count == 1
        assert send.call_args[0][0].method == 'GET'


class TestOffGridError:
    """Test OffGridError exception."""
    
    def test_error_basic(self):
        """Test basic error."""
        error = OffGridError("Something went wrong")
        assert str(error) == "Something went wrong"
        assert error.message == "Something went wrong"
    
    def test_error_with_code(self):
        """Test error with code."""
        error = OffGridError("Model not found", code="model_not_found")
        assert error.code == "model_not_found"
    
    def test_error_with_details(self):
        """Test error with details."""
        error = OffGridError("Error", details="Additional info")
        assert error.details == "Additional info"


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
