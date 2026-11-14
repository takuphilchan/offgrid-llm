package agents

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// ProjectFile is the entire accepted .offgrid/project.toml surface. Keep this
// struct separate from execution configuration: TOML cannot carry grants,
// credentials, shell commands, enrollment, executable hooks or memory consent.
type ProjectFile struct {
	Version     int `toml:"version"`
	Preferences struct {
		Model      string `toml:"model"`
		Delegation *bool  `toml:"delegation"`
	} `toml:"preferences"`
	Resources []api.ProjectResourceReference `toml:"resources"`
}

type ParsedProjectFile struct {
	Preferences ConversationPreferences
	Resources   []api.ProjectResourceReference
	Digest      string
}

// The capability broker supplies these bytes from an already approved context.
// Parsing is deliberately pure: it cannot read a path, install, or execute.
func ParseProjectFile(data []byte) (*ParsedProjectFile, error) {
	if len(data) == 0 || len(data) > 64<<10 || !utf8.Valid(data) {
		return nil, fmt.Errorf("project configuration must be UTF-8 and at most 64 KiB")
	}
	var file ProjectFile
	decoder := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("invalid project configuration: %w", err)
	}
	if file.Version != 1 {
		return nil, fmt.Errorf("unsupported project configuration version")
	}
	preferences := ConversationPreferences{Model: file.Preferences.Model, Delegation: file.Preferences.Delegation}
	if err := validateProjectPreferences(preferences, file.Resources); err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	return &ParsedProjectFile{Preferences: preferences, Resources: file.Resources, Digest: hex.EncodeToString(hash[:])}, nil
}

func validateProjectPreferences(p ConversationPreferences, resources []api.ProjectResourceReference) error {
	if validateConversationPreferences(p) != nil || p.ProfileID != "" || p.Restrictions.Tools != nil || p.Restrictions.Knowledge != nil || p.Restrictions.Computer != nil || p.Restrictions.Execution != nil || len(resources) > 128 {
		return ErrControlCommand
	}
	seen := map[string]bool{}
	for _, ref := range resources {
		if !validProfileText(ref.ID, 512, true) || !validProfileText(ref.Revision, 128, false) || strings.ContainsAny(ref.ID, "\r\n") || seen[ref.Kind+"\x00"+ref.ID] {
			return ErrControlCommand
		}
		switch ref.Kind {
		case "skill", "knowledge", "connection":
		case "instructions":
			if path.IsAbs(ref.ID) || strings.ContainsAny(ref.ID, ":\\") || path.Clean(ref.ID) != ref.ID || ref.ID == ".." || strings.HasPrefix(ref.ID, "../") {
				return ErrControlCommand
			}
		default:
			return ErrControlCommand
		}
		seen[ref.Kind+"\x00"+ref.ID] = true
	}
	return nil
}
