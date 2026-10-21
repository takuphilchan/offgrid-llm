package models

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	ggufMetadataScanLimit = int64(64 << 20)
	ggufStringLimit       = uint64(16 << 20)
)

// GGUFContextProfile contains only bounded, architecture-level metadata needed
// for context admission. It deliberately does not load tensors or tokenizer data.
type GGUFContextProfile struct {
	Architecture       string
	ContextLength      int
	KVElementsPerToken int64
}

// ReadGGUFContextProfile reads the small scalar portion of a GGUF header. Unknown
// or malformed metadata is an error so callers can keep the model unmeasured
// instead of inventing a trained context limit.
func ReadGGUFContextProfile(path string) (GGUFContextProfile, error) {
	f, err := os.Open(path)
	if err != nil {
		return GGUFContextProfile{}, err
	}
	defer f.Close()
	return readGGUFContextProfile(f)
}

func readGGUFContextProfile(r io.ReadSeeker) (GGUFContextProfile, error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil || string(magic[:]) != "GGUF" {
		return GGUFContextProfile{}, fmt.Errorf("invalid GGUF header")
	}
	version, err := readU32(r)
	if err != nil || version < 2 || version > 3 {
		return GGUFContextProfile{}, fmt.Errorf("unsupported GGUF version")
	}
	if _, err = readU64(r); err != nil { // tensor count
		return GGUFContextProfile{}, err
	}
	metadataCount, err := readU64(r)
	if err != nil || metadataCount > 1_000_000 {
		return GGUFContextProfile{}, fmt.Errorf("invalid GGUF metadata count")
	}

	profile := GGUFContextProfile{}
	values := map[string]uint64{}
	for i := uint64(0); i < metadataCount; i++ {
		position, _ := r.Seek(0, io.SeekCurrent)
		if position > ggufMetadataScanLimit {
			return GGUFContextProfile{}, fmt.Errorf("GGUF context metadata exceeds scan limit")
		}
		key, err := readGGUFString(r)
		if err != nil {
			return GGUFContextProfile{}, err
		}
		valueType, err := readU32(r)
		if err != nil {
			return GGUFContextProfile{}, err
		}
		wantedString := key == "general.architecture"
		wantedNumber := contextMetadataKey(key)
		value, err := readGGUFValue(r, valueType, wantedString || wantedNumber, 0)
		if err != nil {
			return GGUFContextProfile{}, fmt.Errorf("read GGUF metadata %q: %w", key, err)
		}
		if wantedString {
			profile.Architecture, _ = value.(string)
		}
		if wantedNumber && ggufIntegerType(valueType) {
			if number, ok := value.(uint64); ok {
				values[key] = number
			}
		}
		if contextProfileComplete(profile.Architecture, values) {
			break
		}
	}

	arch := strings.TrimSpace(profile.Architecture)
	context := values[arch+".context_length"]
	blocks := values[arch+".block_count"]
	embedding := values[arch+".embedding_length"]
	heads := values[arch+".attention.head_count"]
	kvHeads := values[arch+".attention.head_count_kv"]
	if context > uint64(^uint(0)>>1) || context > 16_777_216 {
		return GGUFContextProfile{}, fmt.Errorf("GGUF context length overflows int")
	}
	profile.ContextLength = int(context)
	if kvHeads == 0 {
		kvHeads = heads
	}
	if blocks > 0 && blocks <= 1_000_000 && embedding > 0 && embedding <= 1_000_000 && heads > 0 && heads <= 1_000_000 && kvHeads > 0 && kvHeads <= heads {
		headWidth := embedding / heads
		keyWidth := values[arch+".attention.key_length"]
		valueWidth := values[arch+".attention.value_length"]
		if keyWidth == 0 {
			keyWidth = headWidth
		}
		if valueWidth == 0 {
			valueWidth = headWidth
		}
		if keyWidth > 0 && keyWidth <= 1_000_000 && valueWidth > 0 && valueWidth <= 1_000_000 {
			elements := blocks * kvHeads * (keyWidth + valueWidth)
			if elements <= uint64(^uint64(0)>>1) {
				profile.KVElementsPerToken = int64(elements)
			}
		}
	}
	if profile.ContextLength < 1 {
		return GGUFContextProfile{}, fmt.Errorf("GGUF trained context is unavailable")
	}
	return profile, nil
}

func ggufIntegerType(valueType uint32) bool {
	return valueType <= 5 || valueType == 10 || valueType == 11
}

func contextMetadataKey(key string) bool {
	for _, suffix := range []string{
		".context_length", ".block_count", ".embedding_length",
		".attention.head_count", ".attention.head_count_kv",
		".attention.key_length", ".attention.value_length",
	} {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}
	return false
}

func contextProfileComplete(arch string, values map[string]uint64) bool {
	if arch == "" {
		return false
	}
	for _, suffix := range []string{".context_length", ".block_count", ".embedding_length", ".attention.head_count"} {
		if values[arch+suffix] == 0 {
			return false
		}
	}
	return values[arch+".attention.head_count_kv"] > 0
}

func readGGUFValue(r io.ReadSeeker, valueType uint32, capture bool, depth int) (any, error) {
	if depth > 2 {
		return nil, fmt.Errorf("nested GGUF array is too deep")
	}
	sizes := map[uint32]int64{0: 1, 1: 1, 2: 2, 3: 2, 4: 4, 5: 4, 6: 4, 7: 1, 10: 8, 11: 8, 12: 8}
	if size, ok := sizes[valueType]; ok {
		var raw [8]byte
		if _, err := io.ReadFull(r, raw[:size]); err != nil {
			return nil, err
		}
		if !capture {
			return nil, nil
		}
		switch size {
		case 1:
			return uint64(raw[0]), nil
		case 2:
			return uint64(binary.LittleEndian.Uint16(raw[:2])), nil
		case 4:
			return uint64(binary.LittleEndian.Uint32(raw[:4])), nil
		case 8:
			return binary.LittleEndian.Uint64(raw[:8]), nil
		}
	}
	switch valueType {
	case 8: // string
		value, err := readGGUFString(r)
		if !capture {
			return nil, err
		}
		return value, err
	case 9: // array
		elementType, err := readU32(r)
		if err != nil {
			return nil, err
		}
		count, err := readU64(r)
		if err != nil || count > 10_000_000 {
			return nil, fmt.Errorf("invalid GGUF array length")
		}
		if size, ok := sizes[elementType]; ok {
			bytes := uint64(size) * count
			if count != 0 && bytes/count != uint64(size) || bytes > uint64(^uint64(0)>>1) {
				return nil, fmt.Errorf("GGUF array size overflow")
			}
			_, err = r.Seek(int64(bytes), io.SeekCurrent)
			return nil, err
		}
		for i := uint64(0); i < count; i++ {
			if _, err = readGGUFValue(r, elementType, false, depth+1); err != nil {
				return nil, err
			}
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown GGUF value type %d", valueType)
	}
}

func readGGUFString(r io.Reader) (string, error) {
	length, err := readU64(r)
	if err != nil {
		return "", err
	}
	if length > ggufStringLimit {
		return "", fmt.Errorf("GGUF string exceeds limit")
	}
	value := make([]byte, int(length))
	if _, err = io.ReadFull(r, value); err != nil {
		return "", err
	}
	return string(value), nil
}

func readU32(r io.Reader) (uint32, error) {
	var value uint32
	err := binary.Read(r, binary.LittleEndian, &value)
	return value, err
}

func readU64(r io.Reader) (uint64, error) {
	var value uint64
	err := binary.Read(r, binary.LittleEndian, &value)
	return value, err
}
