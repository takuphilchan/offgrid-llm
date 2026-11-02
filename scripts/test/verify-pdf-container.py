"""Run authored PDF fixtures in a shell-free, offline parser container.

Build docker/Dockerfile's document-worker-check target first. Never targets a
running service, mounts user data, pulls an image or uses private documents.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import uuid


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--image", required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    corpus = root / "internal/rag/testdata/documents/corpus"
    manifest = json.loads((corpus / "manifest.json").read_text(encoding="utf-8"))
    lock = json.loads((root / "runtimes/pdfium/engine.lock.json").read_text())
    image = subprocess.check_output(
        ["docker", "image", "inspect", "--format", "{{.Id}}", args.image],
        text=True, timeout=30,
    ).strip()
    if not image.startswith("sha256:"):
        raise RuntimeError("Expected an already-built immutable image")

    def run(data, arguments=()):
        name = "offgrid-pdf-fixture-" + uuid.uuid4().hex
        command = ["docker", "run", "--pull=never", "--rm", "-i", "--name", name,
                   "--network=none", "--read-only", "--user=65532:65532",
                   "--cap-drop=ALL", "--security-opt=no-new-privileges",
                   "--memory=768m", "--pids-limit=32", image, *arguments]
        try:
            result = subprocess.run(command, input=data, capture_output=True, timeout=45)
            if result.returncode:
                raise RuntimeError(f"Parser container exited {result.returncode}")
            return result.stdout
        finally:
            # Only this fixture-owned identity, including a client timeout.
            subprocess.run(["docker", "rm", "-f", name], capture_output=True, timeout=15)

    notices = run(b"", ["--licenses"])
    assert hashlib.sha256(notices).hexdigest() == lock["notices_sha256"]
    count = 0
    for fixture in manifest["files"]:
        if not fixture["path"].endswith(".pdf"):
            continue
        data = (corpus / fixture["path"]).read_bytes()
        assert hashlib.sha256(data).hexdigest() == fixture["sha256"]
        result = json.loads(run(data))
        assert result["protocol"] == 1 and result["sha256"] == fixture["sha256"]
        assert result["revision"] == "pdfium-8057-0001cb839e62425b-offgrid1"
        pages = result.get("pages", [])
        text = " ".join(" ".join(page["text"] for page in pages).split())
        error = result.get("error", "")
        if not error:
            error = ("document_ocr_required" if any(page["state"] == "ocr_required" for page in pages)
                     else "" if text else "document_empty")
        assert error == fixture["expected_error"], fixture["path"]
        if not error:
            assert [f'page:{page["page"]}' for page in pages] == fixture["source_units"]
            for expected in fixture["expected_text"]:
                expected = " ".join(expected.split())
                offset = text.find(expected)
                assert offset >= 0, fixture["path"]
                text = text[offset + len(expected):]
        print(f"PASS {fixture['path']}", flush=True)
        count += 1
    assert count >= 18, "PDF corpus unexpectedly narrowed"
    print(f"PASS {count} PDF cases and embedded redistribution notices")


if __name__ == "__main__":
    main()
