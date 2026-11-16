package documentocr

import (
	"encoding/hex"
	"errors"
)

const MaxCheckpointBytes = 3 << 20
const MaxCheckpointSourceBytes = 32 << 20
const MaxCheckpointPages = 1000

var ErrCheckpointInvalid = errors.New("invalid OCR page checkpoint")

// Checkpoints is supplied by the source owner, never by document content. Stores
// recheck current source authority for every operation. A nil read is a miss;
// storage/authority errors must stop processing rather than bypass persistence.
type Checkpoints struct {
	Read  func(page int, key string) ([]byte, error)
	Write func(page int, key string, receipt []byte) error
}

func ValidCheckpointKey(page int, key string) bool {
	if page < 1 || page > MaxCheckpointPages || len(key) != 64 {
		return false
	}
	_, err := hex.DecodeString(key)
	return err == nil
}
