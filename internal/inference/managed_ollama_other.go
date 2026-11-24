//go:build !windows && !linux

package inference

import "errors"

func prepareManagedOllamaEndpoint(HTTPEndpoint) (HTTPEndpoint, error) {
	return HTTPEndpoint{}, errors.New("owned Ollama process and listener containment is unavailable on this platform")
}
