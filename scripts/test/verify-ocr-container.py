"""Exercise already-built parser/OCR images offline, without host document tools.

Uses only authored fixtures and an explicitly supplied, pinned language pack.
Never mounts a user workspace or changes an installed service.
"""
import argparse
import base64
import hashlib
import json
from pathlib import Path
import subprocess
import uuid
import xml.etree.ElementTree as ET


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--pdf-image", required=True)
    parser.add_argument("--ocr-image", required=True)
    parser.add_argument("--language", type=Path, required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    lock = json.loads((root / "runtimes/ocr/engine.lock.json").read_text())
    source = json.loads((root / "runtimes/ocr/source.lock.json").read_text())
    language = args.language.read_bytes()
    sha = lambda data: hashlib.sha256(data).hexdigest()
    assert sha(language) == source["evaluation_language"]["sha256"]
    images = {}
    for name in (args.pdf_image, args.ocr_image):
        image = subprocess.check_output(["docker", "image", "inspect", "--format", "{{.Id}}", name], text=True, timeout=30).strip()
        assert image.startswith("sha256:")
        images[name] = image

    def run(image, data, arguments=()):
        name = "offgrid-ocr-fixture-" + uuid.uuid4().hex
        try:
            result = subprocess.run(["docker", "run", "--pull=never", "--rm", "-i", "--name", name,
                                     "--network=none", "--read-only", "--user=65532:65532", "--cap-drop=ALL",
                                     "--security-opt=no-new-privileges", "--memory=1g", "--pids-limit=32",
                                     images[image], *arguments], input=data, capture_output=True, timeout=55)
            if result.returncode:
                raise RuntimeError(f"Fixture worker exited {result.returncode}")
            return result.stdout
        finally:
            subprocess.run(["docker", "rm", "-f", name], capture_output=True, timeout=15)

    assert sha(run(args.ocr_image, b"", ["--licenses"])) == lock["notices_sha256"]
    cases = [("corpus/scan.pdf", "Scanned receipt total 42"),
             ("ocr/inputs/visible-rotation-180.pdf", "Harvest total 37 bags")]
    for file, expected in cases:
        pdf = (root / "internal/rag/testdata/documents" / file).read_bytes()
        raster = json.loads(run(args.pdf_image, pdf, ["--raster-page", "1"]))
        assert not raster.get("error"), raster
        png = base64.b64decode(raster["png"])
        request = {"protocol": 1, "language": "eng", "language_sha256": sha(language),
                   "model": base64.b64encode(language).decode(), "png": raster["png"], "png_sha256": sha(png)}
        receipt = json.loads(run(args.ocr_image, json.dumps(request).encode()))
        assert receipt["protocol"] == 1 and receipt["revision"] == lock["revision"]
        assert receipt["png_sha256"] == sha(png) and receipt["language_sha256"] == sha(language)
        assert not receipt.get("error") and receipt["state"] == "complete", receipt
        words = ["".join(node.itertext()) for node in ET.fromstring(receipt["hocr"]).iter()
                 if "ocrx_word" in node.attrib.get("class", "").split()]
        assert " ".join(words) == expected, (file, words)
        print(f"PASS offline OCR {file}", flush=True)
    print("PASS offline raster/OCR containers and redistribution notices")


if __name__ == "__main__":
    main()
