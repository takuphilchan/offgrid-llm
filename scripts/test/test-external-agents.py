"""Opt-in real-runtime checks in disposable homes; never changes installed config.

Requires installed Hermes/OpenClaw and an explicitly selected local service/model.
Only the file toolsets are enabled for the optional disposable-file workflow.
This is not an OS sandbox or general model qualification.
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

from external_agent_oracle import changed_paths, verify_copy, workspace_snapshot


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("--server", default="http://127.0.0.1:11611")
    parser.add_argument("--model", required=True)
    parser.add_argument("--agent", choices=["hermes", "openclaw"], required=True)
    parser.add_argument("--workflow", action="store_true")
    parser.add_argument("--greeting", action="store_true", help="Also exercise hello with file tools available; review the private answer log")
    parser.add_argument("--workflow-budget", type=int, default=180, help="Explicit isolated workflow budget in seconds (10–900)")
    parser.add_argument("--direct-tools", action="store_true", help="OpenClaw only: offer the isolated allowed tools directly instead of Tool Search")
    parser.add_argument("--fixture", action="store_true", help="Use deterministic local responses; NOT real-model qualification")
    args = parser.parse_args()
    if not 10 <= args.workflow_budget <= 900:
        parser.error("--workflow-budget must be 10–900 seconds")
    if args.direct_tools and args.agent != "openclaw":
        parser.error("--direct-tools is supported only for the isolated OpenClaw fixture")
    if os.name != "posix":
        raise SystemExit("This isolation harness currently requires Linux/WSL or macOS")
    executable = shutil.which(args.agent)
    if not executable:
        raise SystemExit(f"Install {args.agent} explicitly before this test")
    root = Path(tempfile.mkdtemp(prefix=f"offgrid-{args.agent}-qualification-"))
    print(f"Private evidence: {root}", flush=True)
    workspace = root / "workspace"
    workspace.mkdir()
    fixture = None
    if args.fixture:
        from external_agent_fixture import start_fixture
        fixture, generation = start_fixture(root, args.model)
        args.server = f"http://127.0.0.1:{fixture.server_port}"
        print("DETERMINISTIC FIXTURE: installed-runtime compatibility, not model quality", flush=True)
    env = {key: os.environ[key] for key in ["PATH", "LANG", "LC_ALL", "TMPDIR", "SYSTEMROOT"] if key in os.environ}
    env.update(HOME=str(root), USER="offgrid-test", HERMES_HOME=str(root / ".hermes"),
               OPENCLAW_STATE_DIR=str(root / ".openclaw"), OPENCLAW_CONFIG_PATH=str(root / ".openclaw/openclaw.json"),
               OFFGRID_INTEGRATIONS_DIR=str(root / "managed"), OFFGRID_SERVER_URL=args.server,
               OFFGRID_CONFIG=str(root / "offgrid.yaml"), NO_COLOR="1", TERM="dumb")
    env["OFFGRID_HERMES_BIN" if args.agent == "hermes" else "OFFGRID_OPENCLAW_BIN"] = executable
    if not args.fixture and os.environ.get("OFFGRID_API_KEY"):
        env["OFFGRID_API_KEY"] = os.environ["OFFGRID_API_KEY"]
    (root / "offgrid.yaml").write_text("{}\n")
    if args.agent == "openclaw":
        (root / ".openclaw").mkdir()
        test_tools = {"allow": ["read", "write"], "fs": {"workspaceOnly": True}}
        if args.direct_tools:
            test_tools["toolSearch"] = False
        Path(env["OPENCLAW_CONFIG_PATH"]).write_text(json.dumps({
            "agents": {"defaults": {"workspace": str(workspace)}},
            "tools": test_tools,
            "gateway": {"mode": "local"},
        }))
    else:
        (root / ".hermes").mkdir()
        # Keep isolated local inference evidence independent of GitHub update
        # checks performed even by `hermes --version`. User settings are untouched.
        (root / ".hermes/config.yaml").write_text("agent:\n  max_turns: 6\nupdates:\n  check: false\n")
    binary = str(args.binary.resolve())
    results = []

    def run(label, command, timeout=240, check=True):
        started = time.monotonic()
        with (root / f"{label}.log").open("w") as log:
            process = subprocess.Popen([binary, *command], cwd=workspace, env=env,
                                       stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
            try:
                code = process.wait(timeout=timeout)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()
                code = 124
        result = {"stage": label, "exit_code": code, "seconds": round(time.monotonic() - started, 2)}
        results.append(result)
        (root / "results.json").write_text(json.dumps(results, indent=2))
        print(json.dumps(result), flush=True)
        if code and check:
            raise SystemExit(f"{label} failed; inspect {root / (label + '.log')}")
        return code

    for label in ["configure-first", "configure-repeat"]:
        run(label, ["integrations", "configure", args.agent, "--model", args.model, "--yes"])
    run("status", ["integrations", "status", args.agent, "--json"])
    run("smoke", ["integrations", "test", args.agent])
    run("status-after-smoke", ["integrations", "status", args.agent, "--json"])
    if args.greeting:
        before_greeting = workspace_snapshot(workspace)
        command = ["integrations", "run", args.agent, "hello"]
        if args.agent == "hermes":
            command.extend(["--toolsets", "file", "--ignore-rules", "--max-turns", "6", "--run-budget", str(args.workflow_budget), "--oneshot"])
        else:
            command.extend(["--timeout", str(args.workflow_budget)])
        code = run("greeting", command, timeout=args.workflow_budget + 30, check=False)
        changed = changed_paths(before_greeting, workspace_snapshot(workspace))
        results.append({"stage": "greeting-workspace-preserved", "passed": not changed, "unexpected_changes": changed})
        # Exit status is transport evidence, not proof of a sensible greeting.
        results.append({"stage": "greeting-quality", "status": "requires_log_review"})
        (root / "results.json").write_text(json.dumps(results, indent=2))
        if code or changed:
            raise SystemExit("Greeting failed or changed disposable workspace files; inspect private evidence")
    if args.workflow:
        source = workspace / "source.txt"
        expected = "OffGrid test — café 日本語 12345".encode("utf-8")
        source.write_bytes(expected)
        before_workflow = workspace_snapshot(workspace)
        target = workspace / "copy.txt"
        task = (f"Use file tools to read {source} and write its exact contents to {target}. "
                "Read the new file back and confirm an exact match. Only access these two files. Do not use shell, network or other tools.")
        command = ["integrations", "run", args.agent, task]
        if args.agent == "hermes":
            command.extend(["--toolsets", "file", "--ignore-rules", "--max-turns", "6", "--run-budget", str(args.workflow_budget), "--oneshot"])
        else:
            command.extend(["--timeout", str(args.workflow_budget)])
        code = run("file-workflow", command, timeout=args.workflow_budget + 30, check=False)
        verification = verify_copy(workspace, expected, before_workflow)
        results.append(verification)
        (root / "results.json").write_text(json.dumps(results, indent=2))
        if code or not verification["passed"]:
            raise SystemExit("Agent failed independent file verification; inspect results.json")
    if fixture:
        events = json.loads((root / "fixture-events.json").read_text())
        tool_requests = [event for event in events if event["tools"]]
        sequential = bool(tool_requests) and all(event["parallel_tool_calls_present"] and
                                               event["parallel_tool_calls"] is False for event in tool_requests)
        results.append({"stage": "native-sequential-default", "passed": sequential,
                        "tool_requests": len(tool_requests)})
        (root / "results.json").write_text(json.dumps(results, indent=2))
        if (args.greeting or args.workflow) and not sequential:
            raise SystemExit("Installed runtime did not apply the first-party sequential default; inspect fixture-events.json")
        generation.clear()
        with (root / "cancellation.log").open("w") as log:
            command = [binary, "integrations", "run", args.agent, "FIXTURE_CANCEL_WAIT"]
            if args.agent == "hermes":
                command.extend(["--toolsets", "none", "--ignore-rules", "--oneshot"])
            process = subprocess.Popen(command, cwd=workspace, env=env, stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
            try:
                if not generation.wait(45):
                    raise RuntimeError("Cancellation fixture never reached generation")
                started = time.monotonic()
                process.send_signal(signal.SIGINT)
                code = process.wait(timeout=5)
                results.append({"stage": "cancel-active-generation", "exit_code": code, "seconds": round(time.monotonic() - started, 2), "passed": code != 0})
                if code == 0:
                    raise RuntimeError("Cancellation reported success")
                disconnected = fixture.cancelled.wait(5)
                results.append({"stage": "cancel-inference-connection", "passed": disconnected})
                (root / "results.json").write_text(json.dumps(results, indent=2))
                if not disconnected:
                    raise RuntimeError("CLI exited but its inference request remained connected")
            finally:
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()
        fixture.shutdown()
    (root / "results.json").write_text(json.dumps(results, indent=2))
    print("Completed only the checks listed in results.json; evidence retained privately.")


if __name__ == "__main__":
    main()
