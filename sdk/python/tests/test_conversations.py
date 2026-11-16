"""Canonical commands preserve identity; inspection has no execution side effects."""
from unittest.mock import Mock
import pytest
from offgrid.conversations import Conversations
from urllib.parse import urlsplit, parse_qs


def test_result_paging_is_read_only_and_preserves_reference():
    transport = Mock()
    Conversations(transport).result("result:run:abc", offset=4096, limit=8192)
    transport._request.assert_called_once_with(
        "GET", "/api/v2/results/result%3Arun%3Aabc?offset=4096&limit=8192"
    )


def test_work_change_inspection_retains_paging_and_never_dispatches():
    transport = Mock()
    conversations = Conversations(transport)
    conversations.work_changes("work/one", after="change + one", limit=2)
    conversations.work_change("work/one", "change/one", view="content", offset=65536, limit=8, digest="abc")
    calls = transport._request.call_args_list
    assert all(call.args[0] == "GET" for call in calls)
    first, second = (urlsplit(call.args[1]) for call in calls)
    assert first.path == "/api/v2/work-items/work%2Fone/changes"
    assert parse_qs(first.query) == {"after": ["change + one"], "limit": ["2"]}
    assert second.path == "/api/v2/work-items/work%2Fone/changes/change%2Fone"
    assert parse_qs(second.query) == {"view": ["content"], "offset": ["65536"], "limit": ["8"], "hash": ["abc"]}
    conversations.work_change("work/one", "change/one", view="preserved", digest="abc", artifact="3:displaced")
    call = transport._request.call_args
    assert call.args[0] == "GET"
    assert parse_qs(urlsplit(call.args[1]).query)["artifact"] == ["3:displaced"]
    assert parse_qs(urlsplit(call.args[1]).query)["hash"] == ["abc"]


def test_input_commands_escape_paths_preserve_identity_and_disable_retry():
    transport = Mock()
    conversations = Conversations(transport)
    payload = {"content": "Keep this constraint", "origin": "dictation"}
    conversations.submit_input("stable-input", "conv/one", 9, payload)
    conversations.input_command("stable-stop", "conv/one", "input/two", 10, "stop", {})
    calls = transport._request.call_args_list
    assert calls[0].args == ("POST", "/api/v2/conversations/conv%2Fone/inputs", {
        "request_id": "stable-input", "target_id": "conv/one", "expected_revision": 9,
        "kind": "submit_input", "payload": payload,
    })
    assert calls[1].args == ("POST", "/api/v2/conversations/conv%2Fone/inputs/input%2Ftwo/commands", {
        "request_id": "stable-stop", "target_id": "input/two", "expected_revision": 10,
        "kind": "stop", "payload": {},
    })
    assert all(call.kwargs == {"retry": False} for call in calls)


def test_conversation_command_preserves_revision_and_does_not_retry():
    transport = Mock()
    transport._request.side_effect = RuntimeError("conflict")
    conversations = Conversations(transport)
    payload = {"input": {"content": "Keep this request"}}
    with pytest.raises(RuntimeError):
        conversations.command("stable", "conversation-1", 7, "create_work", payload)
    transport._request.assert_called_once_with("POST", "/api/v2/conversations/conversation-1/commands", {
        "request_id": "stable", "target_id": "conversation-1", "expected_revision": 7,
        "kind": "create_work", "payload": payload,
    }, retry=False)


def test_folder_commands_preserve_identity_and_never_retry_or_enroll():
    transport = Mock()
    conversations = Conversations(transport)
    reference = {"host": "host", "session": "session", "grant": "grant", "revision": 3}
    conversations.command("select-folder", "conversation", 8, "use_project_folder", {"folder": reference})
    conversations.command("clear-folder", "conversation", 9, "clear_project_folder", {})
    calls = transport._request.call_args_list
    assert calls[0].args[2] == {
        "request_id": "select-folder", "target_id": "conversation", "expected_revision": 8,
        "kind": "use_project_folder", "payload": {"folder": reference},
    }
    assert calls[1].args[2]["kind"] == "clear_project_folder"
    assert all(call.kwargs == {"retry": False} for call in calls)


def test_conversation_observation_only_uses_get():
    transport = Mock()
    conversations = Conversations(transport)
    conversations.get("private/id")
    conversations.configuration("private/id")
    conversations.events("private/id", 25, 10)
    conversations.inputs("private/id", 12, 5)
    conversations.input("private/id", "input/one")
    assert all(call.args[0] == "GET" for call in transport._request.call_args_list)
    assert transport._request.call_args_list[0].args[1] == "/api/v2/conversations/private%2Fid"
    assert transport._request.call_args_list[2].args[1].endswith("after=25&limit=10")
    assert transport._request.call_args_list[3].args[1] == "/api/v2/conversations/private%2Fid/inputs?after=12&limit=5"
    assert transport._request.call_args_list[4].args[1] == "/api/v2/conversations/private%2Fid/inputs/input%2Fone"


def test_project_and_work_commands_preserve_control_envelopes():
    transport = Mock()
    conversations = Conversations(transport)
    conversations.project("project/one", 2)
    conversations.project_command("project-change", "project/one", 2, "configure_project", {"name": "Project"})
    conversations.work_command("stop-work", "work/one", 5, "stop", {})
    calls = transport._request.call_args_list
    assert calls[0].args == ("GET", "/api/v2/projects/project%2Fone?revision=2")
    assert calls[1].args[1] == "/api/v2/projects/project%2Fone/commands"
    assert calls[2].args[1] == "/api/v2/work-items/work%2Fone/commands"
    assert calls[2].args[2]["expected_revision"] == 5
    assert all(call.kwargs == {"retry": False} for call in calls[1:])


def test_user_defaults_are_observed_then_changed_without_transport_retry():
    transport = Mock()
    conversations = Conversations(transport)
    conversations.user_preferences()
    conversations.configure_user_preferences("defaults", 3, {"delegation": False})
    calls = transport._request.call_args_list
    assert calls[0].args == ("GET", "/api/v2/user-preferences")
    assert calls[1].args == ("POST", "/api/v2/user-preferences/commands", {
        "request_id": "defaults", "target_id": "user-defaults",
        "expected_revision": 3, "kind": "configure_user_preferences",
        "payload": {"delegation": False},
    })
    assert calls[1].kwargs == {"retry": False}


def test_worker_answer_preserves_question_identity_without_retry():
    transport = Mock()
    payload = {"question_id": "question-one", "content": "Include offline runtimes"}
    Conversations(transport).worker_command("answer-once", "worker/one", 4, "answer", payload)
    transport._request.assert_called_once_with("POST", "/api/v2/workers/worker%2Fone/commands", {
        "request_id": "answer-once", "target_id": "worker/one", "expected_revision": 4,
        "kind": "answer", "payload": payload,
    }, retry=False)


def test_conversation_answer_preserves_displayed_question_without_model_turn():
    transport = Mock()
    payload = {"intent": "answer", "question_id": "question-one", "focused_work_id": "work-one", "content": "Yes", "origin": "dictation"}
    Conversations(transport).submit_input("answer-once", "conversation/one", 7, payload)
    transport._request.assert_called_once_with("POST", "/api/v2/conversations/conversation%2Fone/inputs", {
        "request_id": "answer-once", "target_id": "conversation/one", "expected_revision": 7,
        "kind": "submit_input", "payload": payload,
    }, retry=False)


def test_source_input_preserves_revision_page_and_selection_without_retry():
    transport = Mock()
    payload = {"content": "Explain the selected page", "source_session": "Policy",
               "attachments": [{"id": "file-" + "a" * 32, "revision": "b" * 64, "page": 8}],
               "knowledge_documents": ["policy"], "use_knowledge": True,
               "images": [{"name": "figure.png", "data_url": "data:image/png;base64,fixture"}]}
    Conversations(transport).submit_input("source-once", "conversation", 12, payload)
    call = transport._request.call_args
    assert call.args[2]["payload"] == payload
    assert call.args[2]["request_id"] == "source-once"
    assert call.kwargs == {"retry": False}


def test_library_block_scope_is_sent_atomically_without_retry():
    transport = Mock()
    payload = {"content": "Explain this exception", "knowledge_selections": [
        {"document_id": "policy", "revision": "extraction-original", "block_ids": ["clause", "exception"]}]}
    Conversations(transport).submit_input("scope-once", "conversation", 12, payload)
    transport._request.assert_called_once_with("POST", "/api/v2/conversations/conversation/inputs", {
        "request_id": "scope-once", "target_id": "conversation", "expected_revision": 12,
        "kind": "submit_input", "payload": payload}, retry=False)
