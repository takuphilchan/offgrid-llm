package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/templates"
)

// handleTemplates returns all available prompt templates
func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	templateList := templates.ListTemplates()
	result := make([]map[string]interface{}, 0, len(templateList))

	for _, name := range templateList {
		tpl, err := templates.GetTemplate(name)
		if err != nil {
			continue
		}

		result = append(result, map[string]interface{}{
			"name":        tpl.Name,
			"description": tpl.Description,
			"system":      tpl.System,
			"variables":   tpl.Variables,
			"examples":    tpl.Examples,
		})
	}

	response := map[string]interface{}{
		"total":     len(result),
		"templates": result,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleTemplateDetails returns details for a specific template
func (s *Server) handleTemplateDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Extract template name from URL path
	path := strings.TrimPrefix(r.URL.Path, "/v1/templates/")
	templateName := strings.TrimSpace(path)

	if templateName == "" {
		writeError(w, "Template name required", http.StatusBadRequest)
		return
	}

	tpl, err := templates.GetTemplate(templateName)
	if err != nil {
		writeError(w, fmt.Sprintf("Template not found: %s", templateName), http.StatusNotFound)
		return
	}

	// If POST, apply variables
	if r.Method == http.MethodPost {
		var request struct {
			Variables map[string]string `json:"variables"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		prompt, err := tpl.Apply(request.Variables)
		if err != nil {
			writeError(w, fmt.Sprintf("Failed to apply template: %v", err), http.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"template": tpl.Name,
			"prompt":   prompt,
			"system":   tpl.System,
		}

		json.NewEncoder(w).Encode(response)
		return
	}

	// GET request - return template details
	response := map[string]interface{}{
		"name":        tpl.Name,
		"description": tpl.Description,
		"system":      tpl.System,
		"template":    tpl.Template,
		"variables":   tpl.Variables,
		"examples":    tpl.Examples,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
