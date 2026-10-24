package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type preparedFixtureService struct {
	*fixtureService
	err    error
	models []string
}

func (s *preparedFixtureService) Prepare(_ context.Context, model string) error {
	s.models = append(s.models, model)
	return s.err
}

func TestSharedRunPreparesSelectedModelAndDoesNotLaunchAfterFailure(t *testing.T) {
	for _, failure := range []error{nil, context.Canceled, errors.New("capacity rejected"), &serviceclient.Error{Status: 404}} {
		m, host, service, o := managerFixture(t)
		prepared := &preparedFixtureService{fixtureService: service, err: failure}
		m.Service = prepared
		o.ModelID = "new-installed-model"
		var progress []string
		m.Progress = func(value string) { progress = append(progress, value) }
		r, err := m.Run(context.Background(), "fixture-third", o, []string{"hello"})
		_, legacy := failure.(*serviceclient.Error)
		if failure == nil || legacy {
			if err != nil || host.runs != 1 {
				t.Fatal(r, err)
			}
		} else if !errors.Is(err, failure) || host.runs != 0 {
			t.Fatal("preparation failure reached external launch", r, err)
		}
		if len(prepared.models) != 1 || prepared.models[0] != o.ModelID || len(progress) == 0 || !strings.Contains(progress[0], "Preparing") {
			t.Fatal(prepared.models, progress)
		}
		prepared.models = nil
		if _, err := m.Status(context.Background(), "fixture-third", o); err != nil || len(prepared.models) != 0 {
			t.Fatal("status loaded a model", err)
		}
	}
}

func TestSharedSmokePreparesBeforeAuxiliaryInference(t *testing.T) {
	for _, failure := range []error{nil, errors.New("capacity rejected"), &serviceclient.Error{Status: 404}} {
		m, host, service, o := managerFixture(t)
		prepared := &preparedFixtureService{fixtureService: service, err: failure}
		m.Service = prepared
		o.ModelID = "new-installed-model"
		result, err := m.Test(context.Background(), "fixture-third", o)
		_, legacy := failure.(*serviceclient.Error)
		if failure == nil || legacy {
			if err != nil || result.Smoke.State != "passed" {
				t.Fatal(result, err)
			}
		} else if !errors.Is(err, failure) || result.Phase != "preparation_failed" || host.tests != 0 {
			t.Fatal("preparation failure reached smoke test", result, err)
		}
		if len(prepared.models) != 1 || prepared.models[0] != o.ModelID {
			t.Fatal("smoke prepared wrong model", prepared.models)
		}
	}
}

func TestPreparationClientIsAuthenticatedAndChecksSelectedIdentity(t *testing.T) {
	wrong := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/inference/prepare" || r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("preparation transport changed")
		}
		var request map[string]string
		if json.NewDecoder(r.Body).Decode(&request) != nil || len(request) != 1 || request["model"] != "chosen" {
			t.Error("preparation sent unexpected data")
		}
		model := "chosen"
		if wrong {
			model = "different"
		}
		fmt.Fprintf(w, `{"prepared":true,"model":%q}`, model)
	}))
	defer server.Close()
	client, err := NewServiceClient(server.URL, "fixture-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Prepare(context.Background(), "chosen"); err != nil {
		t.Fatal(err)
	}
	wrong = true
	if err := client.Prepare(context.Background(), "chosen"); err == nil {
		t.Fatal("another model accepted")
	}
}
