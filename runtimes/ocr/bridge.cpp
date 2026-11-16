// OffGrid's bounded, memory-only OCR ABI. No file, URL, shell, arbitrary option
// or model-download interface is exposed to the host process.
#include <tesseract/baseapi.h>
#include <cstdint>
#include <cstring>
#include <new>
#include <string>
#include <vector>

namespace {
struct MemoryPack { std::string language; const char* data; uint32_t size; };
// Each request owns a fresh, single-threaded WASM module. No host file reader is
// available. Keep verified model buffers alive for the complete engine lifetime.
std::vector<MemoryPack> packs;
bool read_pack(const char* filename, std::vector<char>* data) {
  if (!filename || !data) return false;
  for (const auto& pack : packs) {
    if (std::string("/offgrid/") + pack.language + ".traineddata" == filename) {
      data->assign(pack.data, pack.data + pack.size);
      return true;
    }
  }
  return false;
}
uint32_t read32(const unsigned char* value) {
  return uint32_t(value[0]) | (uint32_t(value[1]) << 8) |
    (uint32_t(value[2]) << 16) | (uint32_t(value[3]) << 24);
}
bool unpack(const unsigned char* model, int length, const char* language) {
  packs.clear();
  if (length < 8 || std::memcmp(model, "OGM1", 4) != 0) return false;
  const auto count = read32(model + 4);
  if (count < 2 || count > 3) return false;
  size_t offset = 8;
  std::string selection;
  for (uint32_t i = 0; i < count; ++i) {
    if (offset + 8 > size_t(length)) return false;
    const auto namesize = read32(model + offset), size = read32(model + offset + 4);
    offset += 8;
    if (namesize < 2 || namesize > 12 || size == 0 ||
        uint64_t(offset) + namesize + size > uint64_t(length)) return false;
    const std::string name(reinterpret_cast<const char*>(model + offset), namesize);
    for (const char c : name) if (!((c >= 'a' && c <= 'z') || c == '_')) return false;
    for (const auto& pack : packs) if (pack.language == name) return false;
    offset += namesize;
    packs.push_back({name, reinterpret_cast<const char*>(model + offset), size});
    offset += size;
    if (!selection.empty()) selection += "+";
    selection += name;
  }
  return offset == size_t(length) && selection == language;
}
}

extern "C" {
const char* offgrid_ocr_version() { return tesseract::TessBaseAPI::Version(); }

void* offgrid_ocr_create(const unsigned char* model, int length, const char* language) {
  if (!model || length < 1 || length > 32 * 1024 * 1024 || !language) return nullptr;
  size_t language_length = 0;
  while (language_length <= 38 && language[language_length] != '\0') ++language_length;
  if (language_length < 2 || language_length > 38) return nullptr;
  const bool mixed = std::strchr(language, '+') != nullptr;
  for (size_t i = 0; i < language_length; ++i) {
    const char c = language[i];
    if (!((c >= 'a' && c <= 'z') || c == '_' || c == '+')) return nullptr;
  }
  if (mixed && !unpack(model, length, language)) return nullptr;
  if (!mixed && language_length > 12) return nullptr;
  auto* api = new (std::nothrow) tesseract::TessBaseAPI();
  if (!api) return nullptr;
  if (api->Init(mixed ? "/offgrid/" : reinterpret_cast<const char*>(model), mixed ? 0 : length, language,
                tesseract::OEM_LSTM_ONLY, nullptr, 0, nullptr, nullptr, false, mixed ? read_pack : nullptr) != 0) {
    delete api;
    return nullptr;
  }
  if (mixed) {
    std::vector<std::string> loaded;
    api->GetLoadedLanguagesAsVector(&loaded);
    bool complete = loaded.size() == packs.size();
    for (const auto& pack : packs) {
      bool found = false;
      for (const auto& id : loaded) if (id == pack.language) found = true;
      complete = complete && found;
    }
    // Tesseract can report success after loading only some requested languages.
    // That behavior is not accepted as an OffGrid mixed-language receipt.
    if (!complete) { delete api; return nullptr; }
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
