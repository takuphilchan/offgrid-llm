// Package agentcontext validates context data, never executable authority.
package agentcontext

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var ErrSkill = errors.New("invalid skill: provide SKILL.md with valid name/description and at most 64 safe relative files, 64 KiB each and 512 KiB total")

type SkillFile struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}

type SkillReference struct {
	ID       string `json:"id"`
	Revision int    `json:"revision"`
	Digest   string `json:"digest"`
}

func (r SkillReference) Valid() bool {
	b, err := hex.DecodeString(r.Digest)
	return len(r.ID) > 0 && len(r.ID) <= 128 && !strings.ContainsAny(r.ID, "/\\\x00") && r.Revision > 0 && err == nil && len(b) == 32 && r.Digest == strings.ToLower(r.Digest)
}

type SkillBundle struct {
	ResponseContract *ResponseContract `json:"response_contract,omitempty"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Digest           string            `json:"digest"`
	Files            []SkillFile       `json:"files"`
}

var skillName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func SafePath(p string) bool {
	if p == "" || len(p) > 256 || !utf8.ValidString(p) || strings.ContainsAny(p, "\\:") || path.IsAbs(p) || path.Clean(p) != p || p == "." || strings.HasPrefix(p, "../") {
		return false
	}
	for _, c := range p {
		if unicode.IsControl(c) {
			return false
		}
	}
	for _, part := range strings.Split(p, "/") {
		if strings.TrimSpace(part) != part || strings.HasSuffix(part, ".") {
			return false
		}
	}
	return true
}

// Parse copies and sorts the supplied bytes. Unknown frontmatter is inert;
// allowed-tools, scripts and metadata never become executable configuration.
func Parse(files []SkillFile) (SkillBundle, error) {
	return parse(files, true)
}

// ParseStored preserves the old meaning of previously inert metadata. Only a
// persisted reviewed contract opts an existing revision into validation.
func ParseStored(files []SkillFile, reviewed *ResponseContract) (SkillBundle, error) {
	bundle, err := parse(files, reviewed != nil)
	if err != nil {
		return bundle, err
	}
	if reviewed != nil && (bundle.ResponseContract == nil || *reviewed != *bundle.ResponseContract) {
		return SkillBundle{}, ErrSkill
	}
	return bundle, nil
}

func parse(files []SkillFile, contracts bool) (SkillBundle, error) {
	var bundle SkillBundle
	if len(files) == 0 || len(files) > 64 {
		return bundle, ErrSkill
	}
	seen, total := map[string]bool{}, 0
	var instructions []byte
	for _, f := range files {
		key := strings.ToLower(f.Path)
		total += len(f.Content)
		if !SafePath(f.Path) || seen[key] || len(f.Content) > 64<<10 || total > 512<<10 {
			return bundle, ErrSkill
		}
		seen[key] = true
		bundle.Files = append(bundle.Files, SkillFile{Path: f.Path, Content: append([]byte{}, f.Content...)})
		if f.Path == "SKILL.md" {
			instructions = f.Content
		}
	}
	if len(instructions) > 32<<10 || !utf8.Valid(instructions) || bytes.ContainsRune(instructions, 0) {
		return SkillBundle{}, ErrSkill
	}
	text := strings.ReplaceAll(string(instructions), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return SkillBundle{}, ErrSkill
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return SkillBundle{}, ErrSkill
	}
	var node yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(text[4 : 4+end]))
	if decoder.Decode(&node) != nil || len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return SkillBundle{}, ErrSkill
	}
	var extra yaml.Node
	if decoder.Decode(&extra) != io.EOF {
		return SkillBundle{}, ErrSkill
	}
	if !safeYAML(&node, 0) {
		return SkillBundle{}, ErrSkill
	}
	var fields map[string]interface{}
	if node.Decode(&fields) != nil {
		return SkillBundle{}, ErrSkill
	}
	name, okName := fields["name"].(string)
	description, okDescription := fields["description"].(string)
	if !okName || !okDescription || len(name) > 64 || !skillName.MatchString(name) || strings.TrimSpace(description) == "" || len(description) > 1024 {
		return SkillBundle{}, ErrSkill
	}
	if c, exists := fields["compatibility"]; exists {
		s, ok := c.(string)
		if !ok || len(s) > 500 {
			return SkillBundle{}, ErrSkill
		}
	}
	if m, exists := fields["metadata"]; exists {
		values, ok := m.(map[string]interface{})
		if !ok {
			return SkillBundle{}, ErrSkill
		}
		for _, value := range values {
			if _, ok := value.(string); !ok {
				return SkillBundle{}, ErrSkill
			}
		}
	}
	bundle.Name, bundle.Description = name, description
	if value, exists := fields["response_contract"]; exists && contracts {
		var err error
		bundle.ResponseContract, err = parseResponseContract(value)
		if err != nil {
			return SkillBundle{}, err
		}
	}
	sort.Slice(bundle.Files, func(i, j int) bool { return bundle.Files[i].Path < bundle.Files[j].Path })
	encoded, err := json.Marshal(bundle.Files)
	if err != nil {
		return SkillBundle{}, err
	}
	digest := sha256.Sum256(encoded)
	bundle.Digest = hex.EncodeToString(digest[:])
	return bundle, nil
}

func safeYAML(node *yaml.Node, depth int) bool {
	if depth > 8 || node.Kind == yaml.AliasNode || node.Anchor != "" {
		return false
	}
	if node.Kind == yaml.MappingNode {
		keys := map[string]bool{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || keys[key.Value] {
				return false
			}
			keys[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if !safeYAML(child, depth+1) {
			return false
		}
	}
	return true
}
