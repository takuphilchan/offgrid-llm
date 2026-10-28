"""Explicit opt-in helpers for real native-agent qualification, never default CI."""
import argparse


def valid_conversation_workspace_transition(previous, current):
    """A host session can change only for a recorded, strictly broader grant."""
    old = previous.get('execution_workspace') or {}
    new = current.get('execution_workspace') or {}
    old_ref, new_ref = previous.get('execution') or {}, current.get('execution') or {}
    if not old_ref or not new_ref:
        return False
    if any(old.get(key) != new.get(key) for key in ('root', 'directory', 'backend')) or old_ref.get('host') != new_ref.get('host'):
        return False
    old_permissions, new_permissions = old.get('permissions', {}), new.get('permissions', {})
    if old_ref == new_ref:
        return old_permissions == new_permissions
    keys = ('read', 'write', 'processes')
    broader = all(not old_permissions.get(k) or new_permissions.get(k) for k in keys) and any(new_permissions.get(k) and not old_permissions.get(k) for k in keys)
    resolved = any(step.get('type') == 'input_resolved' and step.get('tool_name') == 'request_workspace_access' for step in current.get('steps', []))
    return broader and resolved
import hashlib
import ipaddress
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request


def verified_input_write(step, prompt, wanted, reference, path):
    """Accept native re-open/read-back evidence, never an assistant claim."""
    if step.get('tool_name') != 'workspace_write_from_input':
        return False
    try:
        view = json.loads(step['tool_result'])
        proof = view['source_input']
        receipt = view['receipt']
        observed = receipt['result']
        start, end = proof['start_byte'], proof['end_byte']
        original, expected = prompt.encode('utf-8'), wanted.encode('utf-8')
        return (
            view['host'] == reference['host'] == observed['host']
            and view['session'] == reference['session']
            and view['operation'] == 'file_write' and receipt['state'] == 'complete'
            and observed['path'] == str(path) and observed['verified'] is True
            and type(observed['bytes']) is int and observed['bytes'] == len(expected)
            and observed['revision'] == hashlib.sha256(expected).hexdigest()
            and type(proof['version']) is int and proof['version'] == 1 and proof['input'] == 'request'
            and proof['content_digest'] == hashlib.sha256(expected).hexdigest()
            and proof['input_digest'] == hashlib.sha256(original).hexdigest()
            and type(start) is int and type(end) is int
            and 0 <= start <= end <= len(original) and original[start:end] == expected
        )
    except (KeyError, TypeError, ValueError):
        return False


def configure(description, *, conversation=False):
    parser = argparse.ArgumentParser(description=description)
    parser.add_argument('--cli', required=True, help='Absolute trusted installed OffGrid executable')
    parser.add_argument('--model', required=True, help='Explicit already installed local model')
    parser.add_argument('--endpoint', default='http://127.0.0.1:11611')
    parser.add_argument('--allow-disposable-workspace', action='store_true', help='Approve only this fixture\'s temporary files and trusted Go test command')
    if conversation:
        parser.add_argument('--require-input-source', action='store_true', help='Require typed source provenance in addition to exact bytes and independent read-back')
        parser.add_argument('--check-abstention', action='store_true', help='Prepend greeting and explanation turns that must not inspect or mutate the workspace')
    args = parser.parse_args()
    if not args.allow_disposable_workspace:
        parser.error('No effects authorized: pass --allow-disposable-workspace explicitly')
    if not sys.platform.startswith('linux'):
        parser.error('This PTY/proc fixture is Linux/WSL only; other platforms remain unverified')
    cli = Path(args.cli)
    if not cli.is_absolute() or cli.is_symlink() or not cli.is_file() or not os.access(cli, os.X_OK):
        parser.error('--cli must be an absolute trusted regular executable')
    origin = urllib.parse.urlsplit(args.endpoint)
    host = origin.hostname or ''
    try:
        local = host == 'localhost' or ipaddress.ip_address(host).is_loopback
    except ValueError:
        local = False
    if not local or origin.scheme != 'http' or origin.username or origin.password or origin.path not in ['', '/'] or origin.query or origin.fragment:
        parser.error('--endpoint must be a loopback HTTP origin without credentials, path or query')
    go = shutil.which('go')
    if not go:
        parser.error('Go 1.24+ must already be installed on this host PATH')
    probe = subprocess.run([go, 'version'], cwd=tempfile.gettempdir(), env={**os.environ, 'GOTOOLCHAIN': 'local'}, capture_output=True, text=True, timeout=5, check=True)
    version = re.search(r'go(\d+)\.(\d+)', probe.stdout)
    if not version or tuple(map(int, version.groups())) < (1, 24):
        parser.error('Fixture requires installed Go 1.24+; no toolchain is downloaded')
    endpoint = args.endpoint.rstrip('/')
    os.environ['OFFGRID_SERVER_URL'] = endpoint
    headers = {'Content-Type': 'application/json'}
    key = os.environ.get('OFFGRID_API_KEY', '').strip()
    if key:
        headers['Authorization'] = 'Bearer ' + key

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, request, file, code, message, headers, new_url):
            return None
    opener = urllib.request.build_opener(NoRedirect())

    def api(path, body=None):
        request = urllib.request.Request(endpoint + path, data=None if body is None else json.dumps(body).encode(), headers=headers)
        try:
            with opener.open(request, timeout=8) as response:
                return json.load(response)
        except urllib.error.HTTPError as error:
            code = error.code
            error.close()
            raise RuntimeError(f'Fixture API rejected the request (HTTP {code}); no request replay') from None
    result = str(cli), args.model, api
    return (*result, args.require_input_source, args.check_abstention) if conversation else result
