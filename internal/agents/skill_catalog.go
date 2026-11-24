package agents

import (
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
)

// SkillReferenceAvailable checks only owner/revision metadata. Unlike GetSkill
// and SkillRevision it never loads package contents, even for an older pin.
func (m *Manager) SkillReferenceAvailable(workspace, actor string, ref agentcontext.SkillReference) bool {
	if !validProfileOwner(workspace, actor) || !ref.Valid() {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.contextDatabase()
	if err != nil {
		return false
	}
	defer db.Close()
	var exists int
	err = db.QueryRow(`SELECT 1 FROM agent_context_skills s JOIN agent_context_skill_revisions r ON r.skill_id=s.id WHERE s.workspace=? AND s.actor=? AND s.id=? AND r.revision=? AND r.digest=? AND s.draft=0`, workspace, actor, ref.ID, ref.Revision, ref.Digest).Scan(&exists)
	return err == nil && exists == 1
}

// Catalog discovery transfers metadata, never package contents. Content and
// digest validation still happen through GetSkill when a selected file loads.
// The bounded snapshot is pinned into the run before the model sees it.
func (m *Manager) SkillCatalog(workspace, actor string) ([]AgentSkill, error) {
	if !validProfileOwner(workspace, actor) {
		return nil, ErrContextNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.contextDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT s.id,s.revision,r.digest,json_extract(r.bundle,'$.name'),json_extract(r.bundle,'$.description'),COALESCE(json_extract(r.bundle,'$.response_contract'),'null'),r.created_at,
(SELECT json_group_array(json_extract(f.value,'$.path')) FROM json_each(r.bundle,'$.files') f)
FROM agent_context_skills s JOIN agent_context_skill_revisions r ON r.skill_id=s.id AND r.revision=s.revision
WHERE s.workspace=? AND s.actor=? AND s.draft=0 AND json_valid(r.bundle) ORDER BY s.name,s.id LIMIT 128`, workspace, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AgentSkill{}
	for rows.Next() {
		var item AgentSkill
		var contract, created, paths string
		if err = rows.Scan(&item.ID, &item.Revision, &item.Digest, &item.Name, &item.Description, &contract, &created, &paths); err != nil {
			return nil, err
		}
		if !item.SkillReference.Valid() || !validProfileText(item.Name, 128, true) || !validProfileText(item.Description, 4096, false) || json.Unmarshal([]byte(paths), &item.ResourcePaths) != nil {
			continue
		}
		valid := len(item.ResourcePaths) > 0 && len(item.ResourcePaths) <= 64
		for _, path := range item.ResourcePaths {
			valid = valid && agentcontext.SafePath(path)
		}
		if !valid || json.Unmarshal([]byte(contract), &item.ResponseContract) != nil {
			continue
		}
		if item.ResponseContract != nil && item.ResponseContract.Validate() != nil {
			continue
		}
		item.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
