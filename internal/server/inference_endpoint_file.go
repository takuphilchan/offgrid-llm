package server

import (
	"errors"
	"os"
)

func openObservedInferenceEndpointFile(path string, before os.FileInfo) (*os.File, error) {
	file, err := openInferenceEndpointConfigFile(path)
	if err != nil {
		return nil, errors.New("inference endpoint configuration is unavailable")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<10 || !os.SameFile(before, info) {
		file.Close()
		return nil, errors.New("inference endpoint configuration must be the observed bounded regular file")
	}
	return file, nil
}
