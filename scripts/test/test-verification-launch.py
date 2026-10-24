"""Opt-in installed agent launch tests in disposable homes and workspaces.

Default inference is deterministic (not model quality). --live requires an
explicit local service and installed model; personal configuration is untouched.
Evidence stays in a disposable private directory, never an OS sandbox.
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import tempfile
import time
from urllib.parse import urlparse
from urllib.request import urlopen

from external_agent_fixture import start_fixture
from external_agent_oracle import workspace_snapshot, changed_paths, verify_copy


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("--agent", choices=("hermes", "openclaw"), default="openclaw")
    parser.add_argument("--live", action="store_true", help="Use the explicitly selected local service/model instead of fixtures")
    parser.add_argument("--server")
    parser.add_argument("--model")
    args = parser.parse_args()
    if os.name != "posix": parser.error("installed fixture currently requires Linux/WSL")
    if args.live:
        if not args.model or not args.server: parser.error("--live requires --server and --model")
        target_url = urlparse(args.server)
        if target_url.scheme != "http" or target_url.hostname not in ("127.0.0.1", "localhost") or target_url.username or target_url.path not in ("", "/"):
            parser.error("live qualification requires a local loopback HTTP service")
        with urlopen(args.server.rstrip("/") + "/v1/stats", timeout=5) as response:
            lifecycle = json.load(response)["inference"]["lifecycle"]
        if lifecycle.get("active") != 0 or lifecycle.get("queued") != 0 or lifecycle.get("switching") is not False:
            raise SystemExit("Service is busy; no test started")
    elif args.server or args.model: parser.error("--server/--model require explicit --live")
    binary = str(args.binary.resolve())
    executable = shutil.which(args.agent)
    if not executable: raise SystemExit("explicitly installed runtime is required")
    root = Path(tempfile.mkdtemp(prefix="offgrid-verification-launch-"))
    print(f"Private {'real-model' if args.live else 'deterministic'} evidence: {root}", flush=True)
    workspace = root / "workspace"
    workspace.mkdir()
    state = root / ".openclaw"
    state.mkdir()
    fixture, generation = (None, None) if args.live else start_fixture(root, "fixture-model")
    config = state / "openclaw.json"
    tool_config = state / "tools.json"
    tool_config.write_text(json.dumps({"allow": ["read", "write"], "fs": {"workspaceOnly": True}, "toolSearch": False}))
    config.write_text(json.dumps({
        "agents": {"defaults": {"workspace": str(root / "saved-workspace"), "systemAgent": {"agentId": "fixture-owner"}, "timeoutSeconds": 180},
                   "entries": {"fixture-owner": {"workspace": str(root / "saved-agent-workspace"), "agentDir": str(root / "saved-agent")}}},
        "session": {"store": str(root / "saved-sessions" / "sessions.json")},
        "tools": {"$include": "tools.json"},
        "gateway": {"mode": "local"},
        "plugins": {"entries": {"offgrid": {"enabled": True, "hooks": {"allowConversationAccess": True}}}},
    }))
    env = {key: os.environ[key] for key in ("PATH", "LANG", "TMPDIR") if key in os.environ}
    env.update(HOME=str(root), OPENCLAW_STATE_DIR=str(state), OPENCLAW_CONFIG_PATH=str(config),
               OFFGRID_OPENCLAW_BIN=executable, OFFGRID_INTEGRATIONS_DIR=str(root / "managed"),
               OFFGRID_SERVER_URL=args.server if args.live else f"http://127.0.0.1:{fixture.server_port}",
               OFFGRID_CONFIG=str(root / "offgrid.yaml"), NO_COLOR="1", TERM="dumb")
    (root / "offgrid.yaml").write_text("{}\n")
    if args.agent == "hermes":
        home = root / ".hermes"
        home.mkdir()
        (home / "config.yaml").write_text("updates:\n  check: false\nplatform_toolsets:\n  cli: [file]\nagent:\n  max_turns: 6\n  run_budget_seconds: 180\nplugins:\n  enabled: [offgrid-verifier]\n")
        env.update(HERMES_HOME=str(home), OFFGRID_HERMES_BIN=executable, TERMINAL_ENV="local", TERMINAL_CWD=str(workspace))
        env.pop("OFFGRID_OPENCLAW_BIN", None)

    def run(label, command, expected_success=True):
        with (root / (label + ".log")).open("w") as log:
            process = subprocess.Popen([binary, *command], env=env, cwd=workspace,
                                       stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
            try:
                code = process.wait(timeout=240 if args.live else 120)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGTERM)
                try: process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()
                code = 124
        print(f"{label}: exit {code}", flush=True)
        if (code == 0) != expected_success or code == 124: raise SystemExit(f"Inspect {root / (label + '.log')}")
        if label in ("greeting", "copy"):
            assert "File contract: passed (contracted_files_verified)" in (root / (label + ".log")).read_text(), "missing passing receipt"

    try:
        run("configure", ["integrations", "configure", args.agent, "--yes", "--model", args.model if args.live else "fixture-model"])
        if args.agent == "hermes": run("install-hooks", ["integrations", "install", "hermes-verifier"])
        saved_config = {path: path.read_bytes() for path in (config, tool_config)}
        if args.agent == "hermes":
            saved_config[home / "config.yaml"] = (home / "config.yaml").read_bytes()
        contract = root / "unchanged.json"
        contract.write_text(json.dumps(dict(version=1, kind="no_changes", workspace=str(workspace))))
        before = workspace_snapshot(workspace)
        run("greeting", [args.agent, "run", "hello", "--verify-contract", str(contract)])
        assert not changed_paths(before, workspace_snapshot(workspace))
        expected = "OffGrid exact — café 日本語 12345".encode()
        source, target = workspace / "source.txt", workspace / "copy.txt"
        source.write_bytes(expected)
        before = workspace_snapshot(workspace)
        contract = root / "copy.json"
        contract.write_text(json.dumps(dict(version=1, kind="exact_copy", workspace=str(workspace), source="source.txt", destination="copy.txt")))
        run("copy", [args.agent, "run", f"Read {source}, copy its exact contents to {target}, then read back {target}. Use only file tools.", "--verify-contract", str(contract)])
        result = verify_copy(workspace, expected, before)
        (root / "oracle.json").write_text(json.dumps(result))
        assert result["passed"], "independent file oracle failed"
        def check_preserved_config():
            for path, content in saved_config.items(): assert path.read_bytes() == content, f"changed saved config: {path.name}"
            for name in ("saved-workspace", "saved-agent-workspace", "saved-agent", "saved-sessions"):
                assert not (root / name).exists(), f"touched original runtime state: {name}"
        check_preserved_config()
        if args.live:
            print("One real-model contracted greeting/copy and independent oracle passed. Review greeting text; this is not general model qualification.")
            return
        # Even matching pre-existing bytes cannot replace this invocation's
        # required read/write/read-back evidence. A lying zero-exit must fail.
        before = workspace_snapshot(workspace)
        run("false-completion", [args.agent, "run", "FIXTURE_FALSE_COMPLETION", "--verify-contract", str(contract)], expected_success=False)
        assert "dependent_read_write_readback_required" in (root / "false-completion.log").read_text()
        assert not changed_paths(before, workspace_snapshot(workspace))
        generation.clear()
        contract = root / "unchanged.json"
        with (root / "cancellation.log").open("w") as log:
            process = subprocess.Popen([binary, args.agent, "run", "FIXTURE_CANCEL_WAIT", "--verify-contract", str(contract)],
                                       env=env, cwd=workspace, stdin=subprocess.DEVNULL,
                                       stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
            try:
                assert generation.wait(90), "cancellation never reached generation"
                started = time.monotonic()
                process.send_signal(signal.SIGINT)
                assert process.wait(timeout=8) != 0, "cancellation claimed success"
                assert fixture.cancelled.wait(5), "inference remained connected after cancellation"
                print(f"cancellation: stopped and disconnected in {time.monotonic() - started:.2f}s", flush=True)
            finally:
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()
        assert "File contract: cancelled (invocation_cancelled)" in (root / "cancellation.log").read_text()
        assert not changed_paths(before, workspace_snapshot(workspace))
        check_preserved_config()
        events = json.loads((root / "fixture-events.json").read_text())
        expected_tools = {"read", "write"} if args.agent == "openclaw" else {"read_file", "write_file"}
        assert any(expected_tools.issubset(set(event["tools"])) for event in events), "configured file tools missing"
        print("Installed launch, hooks, independent oracle, false-completion rejection, cancellation and configuration preservation passed; no real model qualification.")
    finally:
        if fixture: fixture.shutdown()


if __name__ == "__main__": main()
