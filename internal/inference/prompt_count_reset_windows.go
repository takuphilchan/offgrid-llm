//go:build windows && !llama

package inference

import (
	"errors"
	"syscall"
)

func promptCountConnectionReset(err error) bool {
	return errors.Is(err, syscall.WSAECONNRESET)
}
