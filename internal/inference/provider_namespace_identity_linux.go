package inference

import (
	"errors"
	"io"
	"os"
)

func providerProcRead(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (128<<10)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 128<<10 {
		return nil, errors.New("provider proc metadata exceeds bounds")
	}
	return raw, nil
}
