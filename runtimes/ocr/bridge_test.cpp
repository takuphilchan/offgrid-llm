// Isolated native ABI smoke test. This does not qualify the packaged worker.
// Arguments are authored fixture paths, dimensions and pinned language bytes.
#include <cassert>
#include <fstream>
#include <iostream>
#include <string>
#include <vector>

extern "C" {
const char* offgrid_ocr_version();
void* offgrid_ocr_create(const unsigned char*, int, const char*);
char* offgrid_ocr_recognize(void*, const unsigned char*, int, int, int);
int offgrid_ocr_result_size(const char*);
void offgrid_ocr_free_result(char*);
void offgrid_ocr_destroy(void*);
}

static std::vector<unsigned char> read(const char* path) {
  std::ifstream file(path, std::ios::binary);
  assert(file.good());
  return std::vector<unsigned char>(std::istreambuf_iterator<char>(file), {});
}

int main(int argc, char** argv) {
  assert(argc == 5);
  const int width = std::stoi(argv[3]), height = std::stoi(argv[4]);
  assert(width > 0 && width <= 8192 && height > 0 && height <= 8192);
  assert(std::string(offgrid_ocr_version()) == "5.5.4");
  assert(!offgrid_ocr_create(nullptr, 0, "eng"));
  auto model = read(argv[1]), rgba = read(argv[2]);
  assert(model.size() <= 32 * 1024 * 1024);
  assert(rgba.size() == static_cast<size_t>(width) * height * 4);
  assert(!offgrid_ocr_create(model.data(), model.size(), "../eng"));
  assert(!offgrid_ocr_create(model.data(), model.size(), "eng+osd"));
  auto handle = offgrid_ocr_create(model.data(), model.size(), "eng");
  assert(handle);
  assert(!offgrid_ocr_recognize(handle, rgba.data(), rgba.size(), 8193, 1));
  assert(!offgrid_ocr_recognize(handle, rgba.data(), rgba.size(), -1, height));
  assert(!offgrid_ocr_recognize(handle, rgba.data(), rgba.size() - 1, width, height));
  assert(!offgrid_ocr_recognize(handle, nullptr, rgba.size(), width, height));
  assert(offgrid_ocr_result_size(nullptr) == -1);
  char* result = offgrid_ocr_recognize(handle, rgba.data(), rgba.size(), width, height);
  assert(result);
  const int size = offgrid_ocr_result_size(result);
  assert(size > 0);
  std::cout.write(result, size);
  offgrid_ocr_free_result(result);
  offgrid_ocr_destroy(handle);
}
