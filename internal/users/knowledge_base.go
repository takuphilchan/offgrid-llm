package users

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// KnowledgeBase represents a user's isolated knowledge base
type KnowledgeBase struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Documents   []string       `json:"documents"`             // Document IDs
	IsShared    bool           `json:"is_shared"`             // Shared with other users
	SharedWith  []string       `json:"shared_with,omitempty"` // User IDs
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// KnowledgeBaseDocument represents a document in a knowledge base
type KnowledgeBaseDocument struct {
	ID              string         `json:"id"`
	KnowledgeBaseID string         `json:"knowledge_base_id"`
	UserID          string         `json:"user_id"`
	Filename        string         `json:"filename"`
	ContentType     string         `json:"content_type"`
	Size            int64          `json:"size"`
	ChunkCount      int            `json:"chunk_count"`
	CreatedAt       time.Time      `json:"created_at"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// KnowledgeBaseManager manages per-user knowledge bases
type KnowledgeBaseManager struct {
	mu        sync.RWMutex
	bases     map[string]*KnowledgeBase         // KB ID -> KnowledgeBase
	byUser    map[string][]string               // UserID -> KB IDs
	documents map[string]*KnowledgeBaseDocument // Doc ID -> Document
	docsByKB  map[string][]string               // KB ID -> Doc IDs
	dataDir   string
}

// NewKnowledgeBaseManager creates a new knowledge base manager
func NewKnowledgeBaseManager(dataDir string) *KnowledgeBaseManager {
	mgr := &KnowledgeBaseManager{
		bases:     make(map[string]*KnowledgeBase),
		byUser:    make(map[string][]string),
		documents: make(map[string]*KnowledgeBaseDocument),
		docsByKB:  make(map[string][]string),
		dataDir:   dataDir,
	}
	mgr.load()
	return mgr
}

// CreateKnowledgeBase creates a new knowledge base for a user
func (m *KnowledgeBaseManager) CreateKnowledgeBase(userID, name, description string) (*KnowledgeBase, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID()
	kb := &KnowledgeBase{
		ID:          id,
		UserID:      userID,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Documents:   []string{},
		Metadata:    make(map[string]any),
	}

	m.bases[id] = kb
	m.byUser[userID] = append(m.byUser[userID], id)
	m.save()

	return kb, nil
}

// GetKnowledgeBase gets a knowledge base by ID
func (m *KnowledgeBaseManager) GetKnowledgeBase(id string) (*KnowledgeBase, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	kb, ok := m.bases[id]
	return kb, ok
}

// GetUserKnowledgeBases gets all knowledge bases for a user
func (m *KnowledgeBaseManager) GetUserKnowledgeBases(userID string) []*KnowledgeBase {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byUser[userID]
	result := make([]*KnowledgeBase, 0, len(ids))
	for _, id := range ids {
		if kb, ok := m.bases[id]; ok {
			result = append(result, kb)
		}
	}
	return result
}

// GetAccessibleKnowledgeBases gets all knowledge bases accessible to a user
func (m *KnowledgeBaseManager) GetAccessibleKnowledgeBases(userID string) []*KnowledgeBase {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*KnowledgeBase, 0)

	// Add user's own knowledge bases
	for _, id := range m.byUser[userID] {
		if kb, ok := m.bases[id]; ok {
			result = append(result, kb)
		}
	}

	// Add shared knowledge bases
	for _, kb := range m.bases {
		if kb.UserID == userID {
			continue // Already added
		}
		if kb.IsShared {
			result = append(result, kb)
		}
		for _, sharedWith := range kb.SharedWith {
			if sharedWith == userID {
				result = append(result, kb)
				break
			}
		}
	}

	return result
}

// CanAccess checks if a user can access a knowledge base
func (m *KnowledgeBaseManager) CanAccess(userID, kbID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	kb, ok := m.bases[kbID]
	if !ok {
		return false
	}

	// Owner can always access
	if kb.UserID == userID {
		return true
	}

	// Check if shared
	if kb.IsShared {
		return true
	}

	// Check if specifically shared with user
	for _, shared := range kb.SharedWith {
		if shared == userID {
			return true
		}
	}

	return false
}

// UpdateKnowledgeBase updates a knowledge base
func (m *KnowledgeBaseManager) UpdateKnowledgeBase(id string, updates map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	kb, ok := m.bases[id]
	if !ok {
		return fmt.Errorf("knowledge base not found")
	}

	if name, ok := updates["name"].(string); ok {
		kb.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		kb.Description = desc
	}
	if shared, ok := updates["is_shared"].(bool); ok {
		kb.IsShared = shared
	}
	if sharedWith, ok := updates["shared_with"].([]string); ok {
		kb.SharedWith = sharedWith
	}
	if metadata, ok := updates["metadata"].(map[string]any); ok {
		for k, v := range metadata {
			kb.Metadata[k] = v
		}
	}

	kb.UpdatedAt = time.Now()
	m.save()

	return nil
}

// ShareKnowledgeBase shares a knowledge base with specific users
func (m *KnowledgeBaseManager) ShareKnowledgeBase(id string, userIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	kb, ok := m.bases[id]
	if !ok {
		return fmt.Errorf("knowledge base not found")
	}

	// Add unique user IDs
	existing := make(map[string]bool)
	for _, uid := range kb.SharedWith {
		existing[uid] = true
	}
	for _, uid := range userIDs {
		if !existing[uid] && uid != kb.UserID {
			kb.SharedWith = append(kb.SharedWith, uid)
		}
	}

	kb.UpdatedAt = time.Now()
	m.save()

	return nil
}

// UnshareKnowledgeBase removes sharing from users
func (m *KnowledgeBaseManager) UnshareKnowledgeBase(id string, userIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	kb, ok := m.bases[id]
	if !ok {
		return fmt.Errorf("knowledge base not found")
	}

	// Remove specified user IDs
	toRemove := make(map[string]bool)
	for _, uid := range userIDs {
		toRemove[uid] = true
	}

	newShared := make([]string, 0)
	for _, uid := range kb.SharedWith {
		if !toRemove[uid] {
			newShared = append(newShared, uid)
		}
	}
	kb.SharedWith = newShared

	kb.UpdatedAt = time.Now()
	m.save()

	return nil
}

// DeleteKnowledgeBase deletes a knowledge base
func (m *KnowledgeBaseManager) DeleteKnowledgeBase(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	kb, ok := m.bases[id]
	if !ok {
		return fmt.Errorf("knowledge base not found")
	}

	// Remove from user's list
	newIDs := make([]string, 0)
	for _, kbID := range m.byUser[kb.UserID] {
		if kbID != id {
			newIDs = append(newIDs, kbID)
		}
	}
	m.byUser[kb.UserID] = newIDs

	// Delete documents
	for _, docID := range m.docsByKB[id] {
		delete(m.documents, docID)
	}
	delete(m.docsByKB, id)

	// Delete knowledge base
	delete(m.bases, id)
	m.save()

	return nil
}

// AddDocument adds a document to a knowledge base
func (m *KnowledgeBaseManager) AddDocument(kbID, userID, filename, contentType string, size int64, chunkCount int) (*KnowledgeBaseDocument, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	kb, ok := m.bases[kbID]
	if !ok {
		return nil, fmt.Errorf("knowledge base not found")
	}

	// Verify user owns the knowledge base
	if kb.UserID != userID {
		return nil, fmt.Errorf("not authorized to add documents to this knowledge base")
	}

	docID := generateID()
	doc := &KnowledgeBaseDocument{
		ID:              docID,
		KnowledgeBaseID: kbID,
		UserID:          userID,
		Filename:        filename,
		ContentType:     contentType,
		Size:            size,
		ChunkCount:      chunkCount,
		CreatedAt:       time.Now(),
		Metadata:        make(map[string]any),
	}

	m.documents[docID] = doc
	m.docsByKB[kbID] = append(m.docsByKB[kbID], docID)
	kb.Documents = append(kb.Documents, docID)
	kb.UpdatedAt = time.Now()

	m.save()

	return doc, nil
}

// GetDocument gets a document by ID
func (m *KnowledgeBaseManager) GetDocument(docID string) (*KnowledgeBaseDocument, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	doc, ok := m.documents[docID]
	return doc, ok
}

// GetKnowledgeBaseDocuments gets all documents in a knowledge base
func (m *KnowledgeBaseManager) GetKnowledgeBaseDocuments(kbID string) []*KnowledgeBaseDocument {
	m.mu.RLock()
	defer m.mu.RUnlock()

	docIDs := m.docsByKB[kbID]
	result := make([]*KnowledgeBaseDocument, 0, len(docIDs))
	for _, id := range docIDs {
		if doc, ok := m.documents[id]; ok {
			result = append(result, doc)
		}
	}
	return result
}

// DeleteDocument deletes a document from a knowledge base
func (m *KnowledgeBaseManager) DeleteDocument(docID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, ok := m.documents[docID]
	if !ok {
		return fmt.Errorf("document not found")
	}

	// Verify user owns the document
	if doc.UserID != userID {
		return fmt.Errorf("not authorized to delete this document")
	}

	kb := m.bases[doc.KnowledgeBaseID]
	if kb != nil {
		// Remove from knowledge base
		newDocs := make([]string, 0)
		for _, id := range kb.Documents {
			if id != docID {
				newDocs = append(newDocs, id)
			}
		}
		kb.Documents = newDocs
		kb.UpdatedAt = time.Now()
	}

	// Remove from docsByKB
	newDocIDs := make([]string, 0)
	for _, id := range m.docsByKB[doc.KnowledgeBaseID] {
		if id != docID {
			newDocIDs = append(newDocIDs, id)
		}
	}
	m.docsByKB[doc.KnowledgeBaseID] = newDocIDs

	delete(m.documents, docID)
	m.save()

	return nil
}

// GetUserDocumentCount gets the total document count for a user
func (m *KnowledgeBaseManager) GetUserDocumentCount(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, doc := range m.documents {
		if doc.UserID == userID {
			count++
		}
	}
	return count
}

// GetStorageStats gets storage statistics for a user
func (m *KnowledgeBaseManager) GetStorageStats(userID string) map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalSize int64
	var docCount int
	var chunkCount int

	for _, doc := range m.documents {
		if doc.UserID == userID {
			totalSize += doc.Size
			docCount++
			chunkCount += doc.ChunkCount
		}
	}

	kbCount := len(m.byUser[userID])

	return map[string]any{
		"user_id":              userID,
		"knowledge_bases":      kbCount,
		"documents":            docCount,
		"chunks":               chunkCount,
		"total_size_bytes":     totalSize,
		"total_size_formatted": formatBytes(totalSize),
	}
}

// save persists data to disk
func (m *KnowledgeBaseManager) save() {
	if m.dataDir == "" {
		return
	}

	data := struct {
		Bases     map[string]*KnowledgeBase         `json:"bases"`
		Documents map[string]*KnowledgeBaseDocument `json:"documents"`
	}{
		Bases:     m.bases,
		Documents: m.documents,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	path := filepath.Join(m.dataDir, "knowledge_bases.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, jsonData, 0600)
}

// load loads data from disk
func (m *KnowledgeBaseManager) load() {
	if m.dataDir == "" {
		return
	}

	path := filepath.Join(m.dataDir, "knowledge_bases.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var stored struct {
		Bases     map[string]*KnowledgeBase         `json:"bases"`
		Documents map[string]*KnowledgeBaseDocument `json:"documents"`
	}

	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}

	m.bases = stored.Bases
	m.documents = stored.Documents

	// Rebuild indexes
	m.byUser = make(map[string][]string)
	m.docsByKB = make(map[string][]string)

	for id, kb := range m.bases {
		m.byUser[kb.UserID] = append(m.byUser[kb.UserID], id)
	}

	for id, doc := range m.documents {
		m.docsByKB[doc.KnowledgeBaseID] = append(m.docsByKB[doc.KnowledgeBaseID], id)
	}
}

// formatBytes formats bytes to human readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// KnowledgeBaseFilter provides filtering for RAG queries based on user access
type KnowledgeBaseFilter struct {
	manager *KnowledgeBaseManager
}

// NewKnowledgeBaseFilter creates a new knowledge base filter
func NewKnowledgeBaseFilter(manager *KnowledgeBaseManager) *KnowledgeBaseFilter {
	return &KnowledgeBaseFilter{manager: manager}
}

// GetAccessibleDocumentIDs gets all document IDs accessible to a user
func (f *KnowledgeBaseFilter) GetAccessibleDocumentIDs(userID string) []string {
	accessible := f.manager.GetAccessibleKnowledgeBases(userID)

	docIDs := make([]string, 0)
	for _, kb := range accessible {
		docs := f.manager.GetKnowledgeBaseDocuments(kb.ID)
		for _, doc := range docs {
			docIDs = append(docIDs, doc.ID)
		}
	}

	return docIDs
}

// FilterDocumentIDs filters document IDs to only those accessible to a user
func (f *KnowledgeBaseFilter) FilterDocumentIDs(userID string, docIDs []string) []string {
	accessible := make(map[string]bool)
	for _, id := range f.GetAccessibleDocumentIDs(userID) {
		accessible[id] = true
	}

	result := make([]string, 0)
	for _, id := range docIDs {
		if accessible[id] {
			result = append(result, id)
		}
	}

	return result
}
