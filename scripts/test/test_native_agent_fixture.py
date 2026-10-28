import contextlib
import ast
import hashlib
import copy
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import io
import os
from pathlib import Path
import subprocess
import tempfile
import threading
import unittest
from unittest.mock import patch

from native_agent_fixture import configure, verified_input_write, valid_conversation_workspace_transition


class FixtureBoundaryTests(unittest.TestCase):
    def test_conversation_workspace_transition_requires_same_scope_and_upgrade(self):
        old = {'execution': {'host': 'local', 'session': 'one'}, 'execution_workspace': {'root': '/fixture', 'backend': 'local', 'directory': '.', 'permissions': {'read': True}}}
        self.assertTrue(valid_conversation_workspace_transition(old, copy.deepcopy(old)))
        new = copy.deepcopy(old)
        new['execution']['session'] = 'two'
        self.assertFalse(valid_conversation_workspace_transition(old, new))
        new['execution_workspace']['permissions']['write'] = True
        self.assertFalse(valid_conversation_workspace_transition(old, new))
        new['steps'] = [{'type': 'input_resolved', 'tool_name': 'request_workspace_access'}]
        self.assertTrue(valid_conversation_workspace_transition(old, new))
        for section, key, value in [('execution', 'host', 'foreign'), ('execution_workspace', 'root', '/other')]:
            bad = copy.deepcopy(new); bad[section][key] = value
            self.assertFalse(valid_conversation_workspace_transition(old, bad))
        new['execution_workspace']['permissions']['read'] = False
        self.assertFalse(valid_conversation_workspace_transition(old, new))

    def test_native_readback_requires_matching_provenance_not_text_claims(self):
        expected = 'caf\u00e9 \u65e5\u672c\u8a9e'
        prompt = '<a>' + expected + '</a>'
        digest = hashlib.sha256(expected.encode()).hexdigest()
        reference = {'host': 'host', 'session': 'session'}
        view = {'host': 'host', 'session': 'session', 'operation': 'file_write',
                'receipt': {'state': 'complete', 'result': {'host': 'host', 'path': '/fixture/result.txt', 'verified': True, 'bytes': len(expected.encode()), 'revision': digest}},
                'source_input': {'version': 1, 'input': 'request', 'input_digest': hashlib.sha256(prompt.encode()).hexdigest(), 'content_digest': digest, 'start_byte': 3, 'end_byte': 3 + len(expected.encode())}}
        def accepts(value):
            return verified_input_write({'tool_name': 'workspace_write_from_input', 'tool_result': json.dumps(value)}, prompt, expected, reference, '/fixture/result.txt')
        self.assertTrue(accepts(view))
        for section, key, wrong in [('receipt', 'state', 'uncertain'), ('source_input', 'version', True), ('source_input', 'content_digest', '0'*64), ('source_input', 'input_digest', '0'*64), ('source_input', 'start_byte', 0), ('source_input', 'input', 'foreign')]:
            bad = copy.deepcopy(view); bad[section][key] = wrong
            self.assertFalse(accepts(bad))
        for key, wrong in [('host', 'foreign'), ('path', '/other/result.txt'), ('bytes', True), ('verified', False), ('revision', '0'*64)]:
            bad = copy.deepcopy(view); bad['receipt']['result'][key] = wrong
            self.assertFalse(accepts(bad))
        bad = copy.deepcopy(view); bad['session'] = 'foreign'
        self.assertFalse(accepts(bad))
        self.assertFalse(verified_input_write({'tool_name': 'workspace_write_from_input', 'tool_result': 'I verified it.'}, prompt, expected, reference, '/fixture/result.txt'))

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.cli = Path(self.directory.name) / 'trusted-cli'
        self.cli.write_text('fixture only')
        self.cli.chmod(0o700)

    def configured(self, extra):
        argv = ['fixture', '--cli', str(self.cli), '--model', 'explicit-model', *extra]
        probe = subprocess.CompletedProcess([], 0, 'go version go1.26.1 linux/amd64', '')
        with patch('sys.argv', argv), patch('sys.platform', 'linux'), patch('shutil.which', return_value='/trusted/go'), patch('subprocess.run', return_value=probe):
            return configure('isolated qualification fixture')

    def test_opt_in_required_before_any_host_process(self):
        argv = ['fixture', '--cli', str(self.cli), '--model', 'explicit-model']
        with patch('sys.argv', argv), patch('subprocess.run') as run, contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit):
                configure('isolated qualification fixture')
            run.assert_not_called()

    def test_unicode_fixture_bytes_are_not_a_mojibake_self_comparison(self):
        source = Path(__file__).with_name('test-native-agent-conversation.py').read_text(encoding='utf-8')
        tree = ast.parse(source)
        wanted = next(node.value.value for node in ast.walk(tree) if isinstance(node, ast.Assign) and any(isinstance(target, ast.Name) and target.id == 'wanted' for target in node.targets))
        self.assertEqual(hashlib.sha256(wanted.encode('utf-8')).hexdigest(), '99d687fc0fe231322ce25cbf2df188802be89c0b7ccc58655ede23f278bafbe1')
        self.assertIn('notes-\u65e5\u672c\u8a9e.md', [node.value for node in ast.walk(tree) if isinstance(node, ast.Constant) and isinstance(node.value, str)])

    def test_remote_credentialed_or_non_origin_endpoint_rejected(self):
        for endpoint in ['https://example.com', 'http://example.com', 'http://user:secret@localhost', 'http://localhost/path', 'http://localhost?key=private']:
            with self.subTest(endpoint=endpoint), contextlib.redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit):
                    self.configured(['--allow-disposable-workspace', '--endpoint', endpoint])

    def test_authentication_stays_on_selected_origin_and_redirect_never_follows(self):
        requests = []
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args): pass
            def do_GET(self):
                requests.append((self.path, self.headers.get('Authorization')))
                if self.path == '/redirect':
                    self.send_response(307)
                    self.send_header('Location', '/not-authorized')
                    self.end_headers()
                else:
                    self.send_response(200)
                    self.end_headers()
                    self.wfile.write(b'{"ready":true}')
        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            with patch.dict(os.environ, {'OFFGRID_API_KEY': 'private-fixture-key'}):
                _, model, api = self.configured(['--allow-disposable-workspace', '--endpoint', f'http://127.0.0.1:{server.server_port}'])
                self.assertEqual(model, 'explicit-model')
                self.assertEqual(api('/ready'), {'ready': True})
                with self.assertRaisesRegex(RuntimeError, 'HTTP 307'):
                    api('/redirect')
            self.assertEqual(requests, [('/ready', 'Bearer private-fixture-key'), ('/redirect', 'Bearer private-fixture-key')])
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


if __name__ == '__main__':
    unittest.main()
