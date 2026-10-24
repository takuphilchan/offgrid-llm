package models

import (
	"fmt"
	"io"
	"os"
)

// GGUFTemplateIdentity is data used to select reviewed compatibility adapters,
// never a model-provided executable adapter. Tensor data is not read.
type GGUFTemplateIdentity struct {
	Architecture string
	Template     string
}

func ReadGGUFTemplateIdentity(path string) (GGUFTemplateIdentity, error) {
	f, err := os.Open(path)
	if err != nil {
		return GGUFTemplateIdentity{}, err
	}
	defer f.Close()
	return readGGUFTemplateIdentity(io.NewSectionReader(f, 0, ggufMetadataScanLimit))
}

func readGGUFTemplateIdentity(r io.ReadSeeker) (GGUFTemplateIdentity, error) {
	var result GGUFTemplateIdentity
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil || string(magic[:]) != "GGUF" {
		return result, fmt.Errorf("invalid GGUF header")
	}
	version, err := readU32(r)
	if err != nil || version < 2 || version > 3 {
		return result, fmt.Errorf("unsupported GGUF version")
	}
	if _, err = readU64(r); err != nil {
		return result, err
	}
	count, err := readU64(r)
	if err != nil || count > 1_000_000 {
		return result, fmt.Errorf("invalid GGUF metadata count")
	}
	seen := map[string]bool{}
	for i := uint64(0); i < count; i++ {
		key, err := readGGUFString(r)
		if err != nil {
			return GGUFTemplateIdentity{}, err
		}
		kind, err := readU32(r)
		if err != nil {
			return GGUFTemplateIdentity{}, err
		}
		wanted := key == "general.architecture" || key == "tokenizer.chat_template"
		if wanted && (seen[key] || kind != 8) {
			return GGUFTemplateIdentity{}, fmt.Errorf("ambiguous GGUF template identity")
		}
		value, err := readGGUFValue(r, kind, wanted, 0)
		position, seekErr := r.Seek(0, io.SeekCurrent)
		if err != nil || seekErr != nil || position > ggufMetadataScanLimit {
			return GGUFTemplateIdentity{}, fmt.Errorf("unreadable or oversized GGUF template metadata")
		}
		if wanted {
			seen[key] = true
			if key == "general.architecture" {
				result.Architecture = value.(string)
			} else {
				result.Template = value.(string)
				if len(result.Template) > 128<<10 {
					return GGUFTemplateIdentity{}, fmt.Errorf("GGUF template exceeds adapter limit")
				}
			}
		}
	}
	return result, nil
}
