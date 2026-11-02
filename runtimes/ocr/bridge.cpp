// OffGrid's bounded, memory-only OCR ABI. No file, URL, shell, arbitrary option
// or model-download interface is exposed to the host process.
#include <tesseract/baseapi.h>
#include <cstdint>
#include <cstring>
#include <new>

extern "C" {
const char* offgrid_ocr_version() { return tesseract::TessBaseAPI::Version(); }

void* offgrid_ocr_create(const unsigned char* model, int length, const char* language) {
  if (!model || length < 1 || length > 32 * 1024 * 1024 || !language) return nullptr;
  size_t language_length = 0;
  while (language_length <= 12 && language[language_length] != '\0') ++language_length;
  if (language_length < 2 || language_length > 12) return nullptr;
  for (size_t i = 0; i < language_length; ++i) {
    const char c = language[i];
    if (!((c >= 'a' && c <= 'z') || c == '_')) return nullptr;
  }
  auto* api = new (std::nothrow) tesseract::TessBaseAPI();
  if (!api) return nullptr;
  if (api->Init(reinterpret_cast<const char*>(model), length, language,
                tesseract::OEM_LSTM_ONLY, nullptr, 0, nullptr, nullptr, false, nullptr) != 0) {
    delete api;
    return nullptr;
  }
  api->SetPageSegMode(tesseract::PSM_AUTO);
  return api;
}

// Pixels have already been decoded and checked by Go. Passing raw RGBA avoids
// enabling Leptonica's file/image-codec readers in this worker.
char* offgrid_ocr_recognize(void* handle, const unsigned char* rgba, int length, int width, int height) {
  if (!handle || !rgba || width < 1 || height < 1 || width > 8192 || height > 8192 ||
      static_cast<int64_t>(width) * height > 16000000 ||
      static_cast<int64_t>(width) * height * 4 != length) return nullptr;
  auto* api = static_cast<tesseract::TessBaseAPI*>(handle);
  api->SetImage(rgba, width, height, 4, width * 4);
  api->SetSourceResolution(200);
  if (api->Recognize(nullptr) != 0) return nullptr;
  return api->GetHOCRText(0);
}

int offgrid_ocr_result_size(const char* text) {
  if (!text) return -1;
  // Do not expose unbounded string scans/copies to the Go receipt builder.
  for (int i = 0; i <= 2 * 1024 * 1024; ++i) if (text[i] == '\0') return i;
  return -1;
}
void offgrid_ocr_free_result(char* text) { delete[] text; }
void offgrid_ocr_destroy(void* handle) { delete static_cast<tesseract::TessBaseAPI*>(handle); }
}
