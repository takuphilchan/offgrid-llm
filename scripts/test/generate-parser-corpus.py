"""Author bounded, public document fixtures; no installed-user input or network.

Run after generate-document-fixtures.py in its pinned fixture environment.
These are document files, not mocked parser responses. Deliberately malformed
files are labelled explicitly. Regeneration is deterministic.
"""
from hashlib import sha256
from io import BytesIO
from pathlib import Path
import json
import struct
import zipfile
import zlib

import reportlab
from reportlab.pdfgen import canvas
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfbase.pdfdoc import PDFDictionary, PDFStream
from reportlab.lib.utils import ImageReader
from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[2] / "internal/rag/testdata/documents"
OUT = ROOT / "corpus"
OUT.mkdir(parents=True, exist_ok=True)
FONT = Path(reportlab.__file__).parent / "fonts"
pdfmetrics.registerFont(TTFont("FixtureVera", str(FONT / "Vera.ttf")))
(OUT / "bitstream-vera-license.txt").write_bytes((FONT / "bitstream-vera-license.txt").read_bytes())
CASES = []


def add(name, data, family, text=(), units=(), error="", notes=""):
    (OUT / name).write_bytes(data)
    CASES.append(dict(path=name, sha256=sha256(data).hexdigest(), bytes=len(data),
                      family=family, expected_text=list(text), source_units=list(units),
                      expected_error=error, notes=notes))


def pdf(name, pages, *, compressed=True, font="Helvetica", encrypt=None,
        rotation=0, image=False, mixed=False, columns=False):
    buffer = BytesIO()
    # invariant fixes timestamps/IDs and encryption randomness for authored files.
    doc = canvas.Canvas(buffer, invariant=1, pageCompression=int(compressed),
                        encrypt=encrypt, pagesize=(612, 792))
    doc.setTitle(name)
    for i, text in enumerate(pages):
        doc.setPageRotation(rotation)
        doc.setFont(font, 14)
        if image or (mixed and i == 1):
            bitmap = Image.new("RGB", (1200, 220), "white")
            ImageDraw.Draw(bitmap).text((30, 60), text, fill="black", font_size=32)
            doc.drawImage(ImageReader(bitmap), 30, 550, width=550, height=100)
        elif columns:
            doc.drawString(40, 700, text[0])
            doc.drawString(330, 700, text[1])
        else:
            for line, value in enumerate(text.split("\n")):
                doc.drawString(40, 700 - line * 24, value)
        doc.showPage()
    doc.save()
    return buffer.getvalue()


for name, text, options in [
    ("plain.pdf", "The clinic opens at 08:30.", {"compressed": False}),
    ("compressed.pdf", "The library closes at 17:00.", {}),
    ("mapped-latin.pdf", "Café déjà vu — naïve façade", {"font": "FixtureVera"}),
    ("escaped.pdf", "Budget (approved) and path C:\\reports", {"compressed": False}),
    ("multiline.pdf", "First paragraph.\nSecond paragraph.\nThird paragraph.", {}),
    ("rotated.pdf", "Rotated page: harvest total 42.", {"rotation": 90}),
    ("punctuation.pdf", "Email: help@example.test; code #A-17; 12.50%.", {}),
]:
    add(name, pdf(name, [text], **options), "pdf_text", text.split("\n"), ["page:1"])
pages = ["First page: red beans.", "Second page: green peas.", "Third page: blue sky."]
add("three-pages.pdf", pdf("three-pages", pages), "pdf_text", pages,
    ["page:1", "page:2", "page:3"])
add("columns.pdf", pdf("columns", [("Left column: maize", "Right column: rice")], columns=True),
    "pdf_text", ["Left column: maize", "Right column: rice"], ["page:1"],
    notes="Two independent columns; no claim of general table reconstruction.")
add("scan.pdf", pdf("scan", ["Scanned receipt total 42"], image=True), "pdf_scan",
    ["Scanned receipt total 42"], ["page:1"], "document_ocr_required")
add("scan-rotated.pdf", pdf("scan-rotated", ["Rotated scan total 37"], image=True, rotation=90),
    "pdf_scan", ["Rotated scan total 37"], ["page:1"], "document_ocr_required")
add("mixed.pdf", pdf("mixed", ["Native page total 12", "Scanned page total 34"], mixed=True),
    "pdf_mixed", ["Native page total 12", "Scanned page total 34"], ["page:1", "page:2"],
    "document_ocr_required", "Without OCR, partial native text must not be reported complete.")
add("encrypted.pdf", pdf("encrypted", ["Private fixture total 91"], encrypt="fixture-password"),
    "pdf_encrypted", error="document_encrypted", notes="Authored password, no real secrets.")
add("blank.pdf", pdf("blank", [""]), "pdf_empty", error="document_empty")
add("truncated.pdf", pdf("truncated", ["Incomplete fixture"] )[:160],
    "corrupt", error="document_invalid")
add("not-a-pdf.pdf", b"This is a text file, not a PDF.\n", "corrupt", error="document_invalid")


def object_stream_pdf():
    # ISO 32000 object and cross-reference streams, not a hand-waved header.
    objects = [(1, b"<< /Type /Catalog /Pages 2 0 R >>"),
               (2, b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
               (3, b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>"),
               (5, b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")]
    header, bodies = b"", b""
    for number, value in objects:
        header += f"{number} {len(bodies)} ".encode()
        bodies += value + b"\n"
    packed = zlib.compress(header + bodies)
    text = b"BT /F1 14 Tf 40 700 Td (Object stream page: total 23.) Tj ET"
    data = bytearray(b"%PDF-1.5\n%\xe2\xe3\xcf\xd3\n")
    offsets = {}

    def stream(number, dictionary, content):
        offsets[number] = len(data)
        data.extend(f"{number} 0 obj\n<< {dictionary} /Length {len(content)} >>\nstream\n".encode())
        data.extend(content + b"\nendstream\nendobj\n")

    stream(4, "", text)
    stream(6, f"/Type /ObjStm /N 4 /First {len(header)} /Filter /FlateDecode", packed)
    offsets[7] = len(data)
    indices = {number: index for index, (number, _) in enumerate(objects)}
    xref = bytearray(struct.pack(">BIH", 0, 0, 65535))
    for number in range(1, 8):
        xref.extend(struct.pack(">BIH", 2, 6, indices[number]) if number in indices
                    else struct.pack(">BIH", 1, offsets[number], 0))
    stream(7, "/Type /XRef /Size 8 /Root 1 0 R /W [1 4 2]", bytes(xref))
    data.extend(f"startxref\n{offsets[7]}\n%%EOF\n".encode())
    return bytes(data)


add("object-stream.pdf", object_stream_pdf(), "pdf_text", ["Object stream page: total 23."], ["page:1"],
    notes="Page tree and fonts are compressed into an object stream; real xref stream, PDF 1.5.")

# A real PDF with an intentionally excessive Flate content stream (17 MiB).
# Compressed fixture is tiny; authoring/testing expansion is explicitly bounded.
buffer = BytesIO()
doc = canvas.Canvas(buffer, invariant=1)
stream = PDFStream(PDFDictionary({"Filter": "/FlateDecode"}),
                   zlib.compress(b" " * (17 * 1024 * 1024)))
# Use a normal saved document below and add an unreferenced stream: parsers must
# not decompress arbitrary non-page objects just because a regex finds them.
doc.drawString(40, 700, "Small visible page")
doc._doc.Reference(stream, "FixtureLargeUnusedStream")
doc.save()
add("unused-large-stream.pdf", buffer.getvalue(), "pdf_resource",
    ["Small visible page"], ["page:1"], notes="17 MiB unreferenced stream; reading page text does not require it.")

texts = {
    "latin.txt": "Café — résumé: 12345.\n",
    "japanese.txt": "日本語 図書館は午前九時に開きます。\n",
    "greek.txt": "Ελληνικά βιβλιοθήκη\n",
    "hindi.txt": "हिन्दी पुस्तकालय\n",
    "arabic.txt": "المكتبة مفتوحة اليوم\n",
    "notes.md": "# Demo\n\nBring clean water and a notebook.\n",
}
for name, text in texts.items():
    add(name, text.encode(), "text", [text.strip()], ["section:1"])
add("table.csv", b"item,quantity\nmaize,12\nbeans,7\n", "text", ["maize", "12", "beans", "7"], ["section:1"])
add("record.json", b'{"clinic":"North","opening":"08:30"}', "text", ["North", "08:30"], ["section:1"])
add("page.html", b"<!doctype html><html><body><h1>Clinic</h1><p>Open at 08:30.</p></body></html>",
    "text", ["Clinic", "Open at 08:30."], ["section:1"])
add("record.xml", b"<?xml version='1.0'?><clinic><name>North</name><open>08:30</open></clinic>",
    "text", ["North", "08:30"], ["section:1"])
add("basic.rtf", br"{\rtf1\ansi Clinic opens at 08:30.\par Bring a notebook.}",
    "text", ["Clinic opens at 08:30.", "Bring a notebook."], ["section:1"])
add("invalid-utf8.txt", b"bad\xfftext", "corrupt", error="document_invalid")
add("malformed.json", b'{"name":', "corrupt", error="document_invalid")

for name, text, units in [
    ("ordered-workbook.xlsx", ["This sheet is first in workbook order", "Quarterly budget", "日本語 Ελληνικά हिन्दी"],
     ["sheet:First sheet!A1", "sheet:Budget café!A1", "sheet:Budget café!C3"]),
    ("reordered-presentation.pptx", ["Displayed first: café 日本語", "Created first, displayed second", "Displayed third"],
     ["slide:1", "slide:2", "slide:3"]),
]:
    add(name, (ROOT / name).read_bytes(), "office", text, units)


def package(entries):
    output = BytesIO()
    with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, data in sorted(entries.items()):
            info = zipfile.ZipInfo(name, (2000, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(info, data)
    return output.getvalue()


word = '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Clinic café 日本語</w:t></w:r></w:p><w:p><w:r><w:t>Open at 08:30.</w:t></w:r></w:p></w:body></w:document>'
entries = {
    "[Content_Types].xml": '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>',
    "_rels/.rels": '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>',
    "word/document.xml": word,
}
add("paragraphs.docx", package(entries), "office", ["Clinic café 日本語", "Open at 08:30."], ["paragraph:1", "paragraph:2"])
add("corrupt.docx", b"PK\x03\x04broken", "corrupt", error="document_invalid")
add("expanding.docx", package({"word/document.xml": b" " * (65 * 1024 * 1024)}),
    "resource", error="document_limit", notes="65 MiB expanded ZIP exceeds archive ceiling.")
add("empty-package.xlsx", package({"unrelated.xml": "<empty/>"}), "corrupt", error="document_invalid")

manifest = dict(schema_version=1, provenance="Authored OffGrid public fixtures; no user documents. scripts/test/generate-parser-corpus.py",
                license="MIT; embedded Bitstream Vera subset in mapped-latin.pdf is covered by bitstream-vera-license.txt",
                generators={"reportlab": "4.4.4", "pillow": "13.0.0"},
                scoring="Ordered exact snippets after whitespace normalization; errors must match exactly. Source-unit readiness scored separately. OCR expectations describe unavailable-without-OCR, not OCR quality qualification.",
                files=CASES)
(OUT / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
print(f"Authored {len(CASES)} fixtures in {OUT}")
