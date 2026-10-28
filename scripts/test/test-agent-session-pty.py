"""Isolated terminal input checks; requires a compiled cmd/offgrid test binary.

No service, installed model, consent, or user files are involved. The 'approval'
is a synthetic input-ownership marker, not an authorization to execute anything.
"""
import argparse
import fcntl
import json
import os
import pty
import select
import struct
import subprocess
import termios
import time


def run(binary, label, width, keys, expected, plain=False):
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 24, width, 0, 0))
    env = dict(os.environ, TERM="xterm-256color", OFFGRID_TEST_SESSION_PTY="1")
    env["OFFGRID_PLAIN"] = "1" if plain else "0"
    process = subprocess.Popen([binary, "-test.run=^TestAgentSessionPTYInput$", "-test.v"], stdin=slave, stdout=slave, stderr=slave, env=env)
    data = bytearray()

    def until(marker):
        deadline = time.monotonic() + 10
        while marker not in data:
            if time.monotonic() > deadline:
                raise AssertionError(f"{label}: missing {marker!r}; captured {bytes(data)!r}")
            ready, _, _ = select.select([master], [], [], .1)
            if ready:
                data.extend(os.read(master, 65536))

    try:
        until(b"> ")
        os.write(master, keys)
        until(b"APPROVAL_READY")
        # Raw terminal settings must already be restored before consent owns input.
        attrs = termios.tcgetattr(slave)
        assert attrs[3] & termios.ICANON and attrs[3] & termios.ECHO, label
        os.write(master, b"y\n")
        until(b"APPROVAL_RECEIVED")
        process.wait(timeout=10)
        assert process.returncode == 0, bytes(data)
        result = bytes(data).split(b"INPUT_RESULT:", 1)[1].splitlines()[0]
        assert json.loads(result) == expected, (label, result)
        if plain or width < 32:
            assert b"\x1b" not in data, (label, "plain output used cursor control")
        else:
            assert b"\x1b[?2004h" in data and b"\x1b[?2004l" in data, label
        print(f"PASS {label}: exact input and terminal/approval handoff")
    finally:
        if process.poll() is None:
            process.terminate()
            process.wait(timeout=5)
        os.close(master)
        os.close(slave)


def conversation(binary):
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))
    env = dict(os.environ, TERM="xterm-256color", OFFGRID_TEST_SESSION_PTY="1", OFFGRID_PLAIN="0", OFFGRID_TUI="1", OFFGRID_ANIMATIONS="1")
    process = subprocess.Popen([binary, "-test.run=^TestAgentSessionPTYConversation$", "-test.v"], stdin=slave, stdout=slave, stderr=slave, env=env)
    data = bytearray()

    def until(marker):
        deadline = time.monotonic() + 10
        while marker not in data:
            if time.monotonic() > deadline:
                raise AssertionError(f"conversation: missing {marker!r}; {bytes(data)!r}")
            ready, _, _ = select.select([master], [], [], .1)
            if ready:
                data.extend(os.read(master, 65536))

    try:
        until(b"> ")
        os.write(master, b"/skills\r")
        until(b"No skills attached")
        os.write(master, b"hello\r")
        until(b"Hello from the fixture")
        assert b"Completed in" not in data, "draft was buffered until completion"
        assert b"\r\x1b[2K| " in data, "wrapped session output disabled spinner"
        until(b"Completed in")
        os.write(master, b"/exit\r")
        until(b"PASS")
        process.wait(timeout=10)
        assert process.returncode == 0, bytes(data)
        assert bytes(data).count(b"OffGrid  /  Agent chat") == 1
        assert bytes(data).count(b"Hello from the fixture") == 1
        assert b"Inspect: offgrid" not in data and b"Turn: " not in data
        assert b"--resume fixture --profile profile-fixture --profile-revision 7" in data
        print("PASS conversation: one welcome, no command submission, animated progress, incremental answer, one result, resume receipt")
    finally:
        if process.poll() is None:
            process.terminate()
            process.wait(timeout=5)
        os.close(master)
        os.close(slave)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary")
    args = parser.parse_args()
    run(args.binary, "picker", 80, b"/\x1b[B\t\r", "/skills")
    run(args.binary, "unicode paste", 80, "\x1b[200~café 日本語\nsecond line\x1b[201~\r".encode(), "café 日本語\nsecond line")
    run(args.binary, "dismiss unknown", 40, b"/not-a-command\x1b\r", "/not-a-command")
    run(args.binary, "narrow fallback", 28, b"hello\n", "hello\n")
    run(args.binary, "plain", 80, b"/help\n", "/help\n", plain=True)
    conversation(args.binary)
