package rag

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const collectionPolicyVersion = 1
const WorkspaceCollection = "workspace"

var ErrCollectionUnavailable = errors.New("Knowledge collection unavailable")
var ErrCollectionConflict = errors.New("Knowledge collection changed; reload before editing")

// CollectionPolicy supplements, never replaces, the service's Knowledge read
// and manage permissions. Only the reserved workspace policy has shared access.
// Private collection APIs remain disabled until all document paths enforce it.
type CollectionPolicy struct {
	ID       string   `json:"id"`
	Version  int      `json:"version"`
	Revision int      `json:"revision"`
	Name     string   `json:"name"`
	Owner    string   `json:"owner,omitempty"`
	Readers  []string `json:"readers"`
	Editors  []string `json:"editors"`
}

func validKnowledgeActor(actor string) bool {
	return actor != "" && len(actor) <= 256 && utf8.ValidString(actor) && strings.TrimSpace(actor) == actor && !strings.ContainsAny(actor, "\x00\r\n\t")
}

func validCollectionName(name string) bool {
	return name != "" && len(name) <= 256 && utf8.ValidString(name) && strings.TrimSpace(name) == name && !strings.ContainsAny(name, "\x00\r\n")
}

func canonicalCollectionMembers(members []string) ([]string, error) {
	if len(members) > 128 {
		return nil, fmt.Errorf("too many collection members")
	}
	out := append([]string{}, members...)
	sort.Strings(out)
	for i, actor := range out {
		if !validKnowledgeActor(actor) || i > 0 && out[i-1] == actor {
			return nil, fmt.Errorf("invalid or duplicate collection member")
		}
	}
	encoded, _ := json.Marshal(out)
	if len(encoded) > 40000 {
		return nil, fmt.Errorf("collection membership exceeds storage limit")
	}
	return out, nil
}

func (p CollectionPolicy) valid() bool {
	if p.Version != collectionPolicyVersion || p.Revision < 1 || !validCollectionName(p.Name) {
		return false
	}
	if p.ID == WorkspaceCollection {
		return p.Owner == "" && len(p.Readers) == 0 && len(p.Editors) == 0 && p.Revision == 1
	}
	if !strings.HasPrefix(p.ID, "collection-") || len(p.ID) != len("collection-")+32 || !validKnowledgeActor(p.Owner) {
		return false
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(p.ID, "collection-")); err != nil {
		return false
	}
	_, readErr := canonicalCollectionMembers(p.Readers)
	_, editErr := canonicalCollectionMembers(p.Editors)
	return readErr == nil && editErr == nil
}

func (p CollectionPolicy) Allows(actor string, write bool) bool {
	if !p.valid() || !validKnowledgeActor(actor) {
		return false
	}
	if p.ID == WorkspaceCollection || actor == p.Owner {
		return true
	}
	for _, editor := range p.Editors {
		if editor == actor {
			return true
		}
	}
	if !write {
		for _, reader := range p.Readers {
			if reader == actor {
				return true
			}
		}
	}
	return false
}

const collectionColumns = `id,schema_version,revision,name,owner,readers,editors`

func scanCollection(row rowScanner) (*CollectionPolicy, error) {
	var p CollectionPolicy
	var readers, editors string
	if err := row.Scan(&p.ID, &p.Version, &p.Revision, &p.Name, &p.Owner, &readers, &editors); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCollectionUnavailable
		}
		return nil, err
	}
	if len(readers) > 40000 || len(editors) > 40000 || json.Unmarshal([]byte(readers), &p.Readers) != nil || json.Unmarshal([]byte(editors), &p.Editors) != nil || !p.valid() {
		return nil, ErrCollectionUnavailable
	}
	return &p, nil
}

func (s *SQLiteStore) Collection(actor, id string) (*CollectionPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, err := scanCollection(s.db.QueryRow(`SELECT `+collectionColumns+` FROM knowledge_collections WHERE id=?`, id))
	if err != nil {
		return nil, err
	}
	if !p.Allows(actor, false) {
		return nil, ErrCollectionUnavailable
	}
	return p, nil
}

func (s *SQLiteStore) CreateCollection(actor, name string, readers, editors []string) (*CollectionPolicy, error) {
	if !validKnowledgeActor(actor) || !validCollectionName(name) {
		return nil, fmt.Errorf("invalid collection identity")
	}
	readers, err := canonicalCollectionMembers(readers)
	if err != nil {
		return nil, err
	}
	editors, err = canonicalCollectionMembers(editors)
	if err != nil {
		return nil, err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	p := &CollectionPolicy{ID: "collection-" + hex.EncodeToString(nonce[:]), Version: collectionPolicyVersion, Revision: 1, Name: name, Owner: actor, Readers: readers, Editors: editors}
	readJSON, _ := json.Marshal(readers)
	editJSON, _ := json.Marshal(editors)
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.db.Exec(`INSERT INTO knowledge_collections (`+collectionColumns+`) SELECT ?,?,?,?,?,?,? WHERE (SELECT COUNT(*) FROM knowledge_collections)<1024`, p.ID, p.Version, p.Revision, p.Name, p.Owner, string(readJSON), string(editJSON))
	if err != nil {
		return nil, err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return nil, fmt.Errorf("collection storage limit reached")
	}
	return p, nil
}

func (s *SQLiteStore) UpdateCollection(actor, id string, revision int, name string, readers, editors []string) (*CollectionPolicy, error) {
	if !validKnowledgeActor(actor) || id == WorkspaceCollection || revision < 1 || !validCollectionName(name) {
		return nil, ErrCollectionUnavailable
	}
	readers, err := canonicalCollectionMembers(readers)
	if err != nil {
		return nil, err
	}
	editors, err = canonicalCollectionMembers(editors)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := scanCollection(s.db.QueryRow(`SELECT `+collectionColumns+` FROM knowledge_collections WHERE id=?`, id))
	if err != nil || current.Owner != actor {
		return nil, ErrCollectionUnavailable
	}
	if current.Revision != revision {
		return nil, ErrCollectionConflict
	}
	readJSON, _ := json.Marshal(readers)
	editJSON, _ := json.Marshal(editors)
	result, err := s.db.Exec(`UPDATE knowledge_collections SET name=?,readers=?,editors=?,revision=revision+1 WHERE id=? AND owner=? AND revision=? AND schema_version=?`, name, string(readJSON), string(editJSON), id, actor, revision, collectionPolicyVersion)
	if err != nil {
		return nil, err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return nil, ErrCollectionConflict
	}
	return &CollectionPolicy{ID: id, Version: collectionPolicyVersion, Revision: revision + 1, Name: name, Owner: actor, Readers: readers, Editors: editors}, nil
}
