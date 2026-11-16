package documentocr

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
)

const MaxSelectionLanguages = 3
const MaxSelectionBytes = 32 << 20

// Selection is ordered: Tesseract uses the first language as the primary model.
// This identity is separate from interface locale and never adds dependencies.
func SelectionPacks(selection string) ([]LanguagePack, bool) {
	if len(selection) > 38 {
		return nil, false
	}
	ids := strings.Split(selection, "+")
	if len(ids) > MaxSelectionLanguages {
		return nil, false
	}
	packs := make([]LanguagePack, 0, len(ids))
	seen, size := map[string]bool{}, 8
	for _, id := range ids {
		pack, ok := LookupLanguage(id)
		if !ok || seen[id] {
			return nil, false
		}
		seen[id] = true
		size += 8 + len(id) + pack.Bytes
		if size > MaxSelectionBytes {
			return nil, false
		}
		packs = append(packs, pack)
	}
	return packs, true
}

// LookupSelection preserves the historical single-pack digest. Mixed selections
// bind every pinned pack and its order, independent of transport serialization.
func LookupSelection(selection string) (LanguagePack, bool) {
	packs, ok := SelectionPacks(selection)
	if !ok {
		return LanguagePack{}, false
	}
	if len(packs) == 1 {
		return packs[0], true
	}
	var identity strings.Builder
	identity.WriteString("offgrid-ocr-selection-v1\x00")
	result := LanguagePack{Language: selection, Qualification: "unverified"}
	for _, pack := range packs {
		identity.WriteString(pack.Language + "\x00" + pack.SHA256 + "\x00")
		result.Bytes += pack.Bytes
	}
	sum := sha256.Sum256([]byte(identity.String()))
	result.SHA256 = hex.EncodeToString(sum[:])
	return result, true
}

var ErrSelection = errors.New("invalid OCR language selection or pack data")

// Models encodes a bounded memory-only bundle; single-language calls retain the
// original raw traineddata wire format. It performs no filesystem/network reads.
func Models(selection string, data [][]byte) ([]byte, error) {
	packs, ok := SelectionPacks(selection)
	if !ok || len(data) != len(packs) {
		return nil, ErrSelection
	}
	for i, pack := range packs {
		if !validModel(pack, data[i]) {
			return nil, ErrSelection
		}
	}
	if len(packs) == 1 {
		return bytes.Clone(data[0]), nil
	}
	var out bytes.Buffer
	out.WriteString("OGM1")
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(packs)))
	for i, pack := range packs {
		_ = binary.Write(&out, binary.LittleEndian, uint32(len(pack.Language)))
		_ = binary.Write(&out, binary.LittleEndian, uint32(len(data[i])))
		out.WriteString(pack.Language)
		out.Write(data[i])
	}
	if out.Len() > MaxSelectionBytes {
		return nil, ErrSelection
	}
	return out.Bytes(), nil
}

func validModel(pack LanguagePack, data []byte) bool {
	if len(data) != pack.Bytes {
		return false
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == pack.SHA256
}

// ValidateModels is also run inside the isolated worker. Both boundaries verify
// the individual digests, not just the selection identifier supplied by a caller.
func ValidateModels(selection string, data []byte) bool {
	packs, ok := SelectionPacks(selection)
	if !ok || len(data) > MaxSelectionBytes {
		return false
	}
	if len(packs) == 1 {
		return validModel(packs[0], data)
	}
	if len(data) < 8 || string(data[:4]) != "OGM1" || binary.LittleEndian.Uint32(data[4:8]) != uint32(len(packs)) {
		return false
	}
	data = data[8:]
	for _, pack := range packs {
		if len(data) < 8 {
			return false
		}
		nameSize, modelSize := uint64(binary.LittleEndian.Uint32(data[:4])), uint64(binary.LittleEndian.Uint32(data[4:8]))
		data = data[8:]
		if nameSize != uint64(len(pack.Language)) || modelSize != uint64(pack.Bytes) || nameSize+modelSize > uint64(len(data)) {
			return false
		}
		if string(data[:nameSize]) != pack.Language || !validModel(pack, data[nameSize:nameSize+modelSize]) {
			return false
		}
		data = data[nameSize+modelSize:]
	}
	return len(data) == 0
}
