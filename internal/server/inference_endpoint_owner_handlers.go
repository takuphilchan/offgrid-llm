package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

type endpointOwnerEntryView struct {
	endpointOwnerInput
	Managed              bool `json:"managed"`
	CredentialConfigured bool `json:"credential_configured"`
}

type endpointOwnerCredentialView struct {
	ID      string   `json:"id"`
	Origins []string `json:"origins"`
}
type endpointOwnerTemplateView struct {
	ID    string                 `json:"id"`
	Entry endpointOwnerEntryView `json:"entry"`
}
type endpointOwnerSnapshot struct {
	HistoryAvailable  bool                           `json:"history_available"`
	PolicyMaintenance *endpointPolicyMaintenanceView `json:"policy_maintenance,omitempty"`
	JournalRevision   string                         `json:"journal_revision,omitempty"`
	FileRevision      string                         `json:"file_revision"`
	Source            string                         `json:"source"`
	WriteAvailable    bool                           `json:"write_available"`
	WriteReason       string                         `json:"write_reason,omitempty"`
	Actor             string                         `json:"actor"`
	Entries           []endpointOwnerEntryView       `json:"entries"`
	Credentials       []endpointOwnerCredentialView  `json:"credential_references"`
	Templates         []endpointOwnerTemplateView    `json:"managed_templates"`
	Records           []endpointOwnerRecord          `json:"records"`
}

func endpointOwnerView(entry inferenceEndpointEnrollment, host endpointOwnerHost) endpointOwnerEntryView {
	view := endpointOwnerEntryView{endpointOwnerInput: endpointOwnerInput{ID: entry.ID, Revision: entry.Revision, Provider: entry.Provider, URL: entry.URL, AllowedIPs: slices.Clone(entry.AllowedIPs), Actors: slices.Clone(entry.Actors), Model: entry.Model, ModelDigest: entry.ModelDigest, Format: entry.Format, RuntimeVersion: entry.RuntimeVersion, ContextLength: entry.ContextLength, DeploymentAttested: entry.DeploymentAttested}, Managed: entry.ManagedOllama != nil, CredentialConfigured: entry.CredentialEnv != ""}
	for _, credential := range host.Credentials {
		if credential.Environment == entry.CredentialEnv && slices.Contains(credential.Origins, entry.URL) {
			view.Credential = credential.ID
			break
		}
	}
	for _, template := range host.ManagedTemplates {
		candidate := entry
		candidate.ID, candidate.Revision, candidate.Actors = template.Entry.ID, template.Entry.Revision, template.Entry.Actors
		if reflect.DeepEqual(candidate, template.Entry) {
			view.ManagedTemplate = template.ID
			break
		}
	}
	return view
}

func (s *Server) endpointOwnerState(actor string) (endpointOwnerSnapshot, error) {
	state := endpointOwnerSnapshot{HistoryAvailable: true, Source: "workspace", Actor: actor, Entries: []endpointOwnerEntryView{}, Credentials: []endpointOwnerCredentialView{}, Templates: []endpointOwnerTemplateView{}, Records: []endpointOwnerRecord{}}
	owner := s.endpointOwner
	if owner == nil {
		return state, errors.New("endpoint owner configuration is unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if s.config.InferenceEndpointsFile != "" {
		state.Source = "external"
	}
	configuration, pin, err := s.readEndpointConfiguration()
	if err != nil {
		return state, errors.New("endpoint owner file is unavailable")
	}
	state.FileRevision = pin
	host, _, err := readEndpointOwnerHost(s.config.InferenceEndpointOwnerPolicyFile)
	if err != nil {
		return state, err
	}
	for _, entry := range configuration.Endpoints {
		state.Entries = append(state.Entries, endpointOwnerView(entry, host))
	}
	for _, credential := range host.Credentials {
		state.Credentials = append(state.Credentials, endpointOwnerCredentialView{ID: credential.ID, Origins: slices.Clone(credential.Origins)})
	}
	for _, template := range host.ManagedTemplates {
		state.Templates = append(state.Templates, endpointOwnerTemplateView{ID: template.ID, Entry: endpointOwnerView(template.Entry, host)})
	}
	journal, err := readEndpointOwnerJournal(s.config.DataDir)
	if err != nil {
		state.WriteReason = "Endpoint owner journal is unavailable; explicit recovery is required"
		return state, nil
	}
	state.Records = journal.Records
	state.JournalRevision = endpointOwnerJournalRevision(journal)
	if owner.fault != nil {
		state.WriteReason = owner.fault.Error()
		return state, nil
	}
	if err := endpointOwnerJournalReady(s.config.DataDir); err != nil {
		state.WriteReason = err.Error()
		return state, nil
	}
	state.PolicyMaintenance = s.endpointPolicyMaintenanceState(configuration)
	err = verifyEndpointOwnerWriter(s.endpointConfigurationPath(), s.config.DataDir, host.EditableFile)
	if err != nil {
		state.WriteReason = "Installation has not delegated a safely writable owner file"
		return state, nil
	}
	state.WriteAvailable = true
	return state, nil
}

func decodeEndpointOwnerInput(r *http.Request, value any) error {
	return decodeEndpointOwnerInputLimit(r, value, 16<<10)
}

func decodeEndpointOwnerInputLimit(r *http.Request, value any, limit int) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, int64(limit)+1))
	if err != nil || len(raw) > limit || jsonwire.Validate(raw) != nil {
		return errors.New("a bounded endpoint owner request is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil {
		return errors.New("endpoint owner request has unknown or invalid fields")
	}
	return nil
}

func (s *Server) handleEndpointOwner(w http.ResponseWriter, r *http.Request, actor string) {
	w.Header().Set("Cache-Control", "no-store")
	var result any
	var err error
	action := strings.TrimPrefix(r.URL.Path, inferenceEndpointsPath+"/configuration")
	if action == "" && r.Method == http.MethodGet {
		result, err = s.endpointOwnerState(actor)
	} else if action == "/history" && r.Method == http.MethodGet {
		result, err = s.endpointOwnerHistory(r)
	} else if action == "/policy-history" && r.Method == http.MethodGet {
		result, err = s.endpointPolicyHistoryView(r)
	} else if r.Method == http.MethodPost {
		switch action {
		case "/retire-decisions":
			var input endpointPolicyMaintenanceInput
			if err = decodeEndpointOwnerInputLimit(r, &input, 20<<10); err == nil {
				result, err = s.retireEndpointDecisions(r, input)
			}
		case "/archive":
			var input struct {
				Expected string `json:"expected_revision"`
				Journal  string `json:"expected_journal_revision"`
			}
			if err = decodeEndpointOwnerInput(r, &input); err == nil {
				result, err = s.archiveEndpointOwner(r, input.Expected, input.Journal)
			}
		case "/prepare":
			var input endpointOwnerInput
			if err = decodeEndpointOwnerInput(r, &input); err == nil {
				var draft endpointOwnerDraft
				draft, err = s.prepareEndpointOwner(r, input)
				if err == nil {
					host, _, hostErr := readEndpointOwnerHost(s.config.InferenceEndpointOwnerPolicyFile)
					if hostErr != nil {
						err = hostErr
					} else {
						result = map[string]any{"review_id": draft.ID, "expected_revision": draft.Expected, "expires_at": draft.Expires.UTC().Format(time.RFC3339Nano), "entry": endpointOwnerView(draft.Entry, host), "restart_required": true, "access_review_required": true}
					}
				}
			}
		case "/commit":
			var input struct {
				ReviewID  string `json:"review_id"`
				RequestID string `json:"request_id"`
			}
			if err = decodeEndpointOwnerInput(r, &input); err == nil {
				result, err = s.commitEndpointOwner(r, input.ReviewID, input.RequestID)
			}
		case "/reconcile":
			var input struct {
				RequestID string `json:"request_id"`
				Expected  string `json:"expected_revision"`
			}
			if err = decodeEndpointOwnerInput(r, &input); err == nil {
				result, err = s.reconcileEndpointOwner(r, input.RequestID, input.Expected)
			}
		default:
			writeError(w, "Endpoint owner action is unavailable", http.StatusMethodNotAllowed)
			return
		}
	} else {
		writeError(w, "Endpoint owner action is unavailable", http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		if errors.Is(err, errEndpointPolicyPublicationUncertain) {
			writeErrorWithCode(w, "Endpoint maintenance publication is uncertain; restart and inspect actual policy without replay", http.StatusServiceUnavailable, "endpoint_policy_publication_uncertain")
			return
		}
		if errors.Is(err, errEndpointOwnerUncertain) {
			writeErrorWithCode(w, errEndpointOwnerUncertain.Error(), http.StatusServiceUnavailable, "endpoint_owner_publication_uncertain")
			return
		}
		writeError(w, err.Error(), http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
