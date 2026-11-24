package inference

import (
	"errors"
	"os"
	"path/filepath"
)

func LoadONNXEmbeddingRuntimeConfig(filename string) (ONNXEmbeddingRuntimeConfig, error) {
	var config ONNXEmbeddingRuntimeConfig
	if !filepath.IsAbs(filename) || filepath.Clean(filename) != filename {
		return config, errors.New("ONNX runtime enrollment requires a canonical absolute file path")
	}
	before, err := os.Lstat(filename)
	if err != nil {
		return config, err
	}
	if !before.Mode().IsRegular() || before.Size() > 4<<20 {
		return config, errors.New("ONNX runtime enrollment must be a bounded regular file")
	}
	file, err := openONNXRuntimeConfigFile(filename)
	if err != nil {
		return config, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || opened.Size() > 4<<20 {
		return config, errors.New("ONNX runtime enrollment file changed")
	}
	return DecodeONNXEmbeddingRuntimeConfig(file)
}
