package agents

// SkillRevision resolves an owned immutable revision. Revision zero selects the
// current revision once; callers must persist the returned reference before use.
func (m *Manager) SkillRevision(workspace, actor, id string, revision int) (*AgentSkill, error) {
	if !validProfileOwner(workspace, actor) || !validProfileText(id, 128, true) || revision < 0 {
		return nil, ErrContextNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	db, err := m.contextDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	item, _, err := scanSkill(db.QueryRow(`SELECT s.id,r.revision,r.digest,r.bundle,s.draft,s.source_job,r.created_at FROM agent_context_skills s JOIN agent_context_skill_revisions r ON s.id=r.skill_id WHERE s.id=? AND s.workspace=? AND s.actor=? AND r.revision=CASE WHEN ?=0 THEN s.revision ELSE ? END`, id, workspace, actor, revision, revision))
	return item, err
}
