"""Author additional OCR fixtures without changing the frozen parser baseline.

Uses document-fixtures-requirements.txt. No network or personal documents.
Square pages keep all content visible at each PDF rotation; the earlier clipped
rotation case stays intact as negative evidence.
"""
from hashlib import sha256
from io import BytesIO
import json
from pathlib import Path

from PIL import Image, ImageDraw
from reportlab.lib.utils import ImageReader
from reportlab.pdfgen import canvas

OUT = Path(__file__).resolve().parents[2] / "internal/rag/testdata/documents/ocr/inputs"
OUT.mkdir(parents=True, exist_ok=True)
cases = []


def fixture(name, rotation=0, native=False, native_text="Native stock total 12 bags"):
    text = "Harvest total 37 bags"
    image = Image.new("RGB", (1200, 240), "white")
    ImageDraw.Draw(image).text((50, 70), text, fill="black", font_size=48)
    output = BytesIO()
    page = canvas.Canvas(output, pagesize=(400, 400), invariant=1, pageCompression=1)
    page.setPageRotation(rotation)
    page.drawImage(ImageReader(image), 40, 180, width=320, height=64)
    if native:
        page.setFont("Helvetica", 14)
        page.drawString(40, 300, native_text)
    page.showPage()
    page.save()
    data = output.getvalue()
    (OUT / name).write_bytes(data)
    cases.append({"path": name, "sha256": sha256(data).hexdigest(), "rotation": rotation,
                  "expected_text": ([native_text] if native else []) + [text],
                  "page": 1, "mixed_same_page": native})


for rotation in (0, 90, 180, 270):
    fixture(f"visible-rotation-{rotation}.pdf", rotation)
fixture("mixed-same-page.pdf", native=True)
fixture("mixed-repeated-text.pdf", native=True, native_text="Harvest total 37 bags")
(OUT / "manifest.json").write_text(json.dumps({
    "provenance": "Authored by OffGrid for testing; no personal or third-party document content.",
    "qualification": "Expected source content, not a claim of successful OCR.",
    "cases": cases,
}, indent=2) + "\n", encoding="utf-8")
print(f"Authored {len(cases)} OCR fixtures in {OUT}")
