package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	s.syncRegistryLoadState()
	models := s.registry.ListModels()
	for index := range models {
		if models[index].Type == "embedding" {
			models[index].Capabilities = []string{"embeddings"}
			models[index].CapabilityStatus = map[string]string{"embeddings": "declared"}
			continue
		}
		if !models[index].SupportsChat() {
			continue
		}
		contextWindow := s.modelContextWindow(models[index].ID)
		models[index].ContextWindow = contextWindow
		models[index].ContextLength = contextWindow
		models[index].Capabilities = []string{"chat", "streaming"}
		models[index].CapabilityStatus = map[string]string{"chat": "declared", "streaming": "declared", "tools": "unknown"}
		if metadata, err := s.registry.GetModel(models[index].ID); err == nil && metadata.ProjectorPath != "" {
			// An installed projector is unknown until the current runtime passes
			// the synthetic image/tool-call check. "tested" remains a smoke-test
			// status, not end-to-end grounding qualification.
			models[index].CapabilityStatus["vision"] = "unknown"
			if s.computerVisionReady(models[index].ID) {
				models[index].CapabilityStatus["vision"] = "tested"
			}
		}
	}
	response := api.ModelListResponse{
		Object: "list",
		Data:   models,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleRefreshModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Rescan models directory
	if err := s.registry.ScanModels(); err != nil {
		log.Printf("Error scanning models: %v", err)
		http.Error(w, "Failed to refresh models", http.StatusInternalServerError)
		return
	}

	// Return updated model list
	models := s.registry.ListModels()
	response := api.ModelListResponse{
		Object: "list",
		Data:   models,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleDeleteModel deletes a model from the registry and filesystem
func (s *Server) handleDeleteModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		ModelID string `json:"model_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ModelID == "" {
		writeError(w, "model_id is required", http.StatusBadRequest)
		return
	}

	remove := func(owner *inference.LegacyMaintenance) error {
		s.downloadMutex.Lock()
		defer s.downloadMutex.Unlock()
		if len(s.downloadCancelFuncs) > 0 {
			return inference.ErrRuntimeBusy
		}
		s.modelMutex.Lock()
		defer s.modelMutex.Unlock()
		if owner != nil {
			if err := owner.UnloadModel(req.ModelID); err != nil {
				return err
			}
		} else if s.modelCache != nil && s.modelCache.IsModelAlive(req.ModelID) {
			if err := s.modelCache.Unload(req.ModelID); err != nil {
				return err
			}
		}
		if s.currentModelID == req.ModelID {
			if owner != nil {
				if err := owner.DisconnectEngine(); err != nil {
					return err
				}
			} else if s.engine != nil && s.engine.IsLoaded() {
				if err := s.engine.Unload(); err != nil {
					return err
				}
			}
			s.currentModelID = ""
			s.currentPort = 0
			s.currentContext = 0
			s.currentGeneration = 0
		}
		return s.registry.DeleteModel(req.ModelID)
	}
	guardKnowledge := func(owner *inference.LegacyMaintenance) error {
		if s.ragEngine != nil {
			return s.ragEngine.WithModelRemoval(req.ModelID, func() error { return remove(owner) })
		}
		return remove(owner)
	}
	var removeErr error
	if s.providerScheduler != nil {
		_, removeErr = s.providerScheduler.WithLegacyMaintenance(r.Context(), func(owner inference.LegacyMaintenance) error {
			if s.inferenceLifecycle != nil {
				return s.inferenceLifecycle.Maintain(func() error { return guardKnowledge(&owner) })
			}
			return guardKnowledge(&owner)
		})
	} else if s.inferenceLifecycle != nil {
		removeErr = s.inferenceLifecycle.Maintain(func() error { return guardKnowledge(nil) })
	} else {
		removeErr = guardKnowledge(nil)
	}
	if removeErr != nil {
		writeError(w, fmt.Sprintf("Model could not be removed: %v. Stop active work or disable knowledge, then retry.", removeErr), http.StatusConflict)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Model %s deleted successfully", req.ModelID),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleModelCatalog returns the curated model catalog
func (s *Server) handleModelCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Get the default catalog
	catalog := models.DefaultCatalog()

	// Transform catalog entries to a simpler format for the UI
	type SimpleCatalogEntry struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Parameters  string `json:"parameters"`
		Size        string `json:"size"` // backwards-compatible parameter label
		SizeBytes   int64  `json:"size_bytes"`
		Repo        string `json:"repo"`
		File        string `json:"file"`
		Quant       string `json:"quant"`
		MinRAMGB    int    `json:"min_ram_gb"`
		Recommended bool   `json:"recommended"`
		Provider    string `json:"provider"`
		Type        string `json:"type"`
		License     string `json:"license"`
	}

	simpleModels := []SimpleCatalogEntry{}
	for _, entry := range catalog.Models {
		if len(entry.Variants) > 0 {
			// Q4_K_M is the practical quality/size default for local inference.
			// Catalog ordering is not a recommendation signal.
			variant := &entry.Variants[0]
			if preferred := entry.FindVariant("Q4_K_M"); preferred != nil {
				variant = preferred
			}
			if quant := r.URL.Query().Get("quant"); quant != "" {
				if requested := entry.FindVariant(quant); requested != nil {
					variant = requested
				} else {
					continue
				}
			}
			var repo, file string

			// Extract repo and file from sources
			if len(variant.Sources) > 0 {
				for _, source := range variant.Sources {
					if source.Type == "huggingface" {
						if parsed, err := url.Parse(source.URL); err == nil {
							parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
							if len(parts) >= 5 && parts[2] == "resolve" {
								repo = parts[0] + "/" + parts[1]
								file = strings.Join(parts[4:], "/")
							}
							break
						}
					}
				}
			}

			simpleModels = append(simpleModels, SimpleCatalogEntry{
				ID:          entry.ID,
				Name:        entry.Name,
				Description: entry.Description,
				Category:    strings.Join(entry.Tags, ", "),
				Parameters:  entry.Parameters,
				Size:        entry.Parameters,
				SizeBytes:   variant.Size,
				Repo:        repo,
				File:        file,
				Quant:       variant.Quantization,
				MinRAMGB:    entry.MinRAM,
				Recommended: entry.Recommended,
				Provider:    entry.Provider,
				Type:        entry.Type,
				License:     entry.License,
			})
		}
	}

	response := map[string]interface{}{
		"total":  len(simpleModels),
		"models": simpleModels,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
