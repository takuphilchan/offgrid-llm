package integrations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixtureHost struct {
	state                          HostState
	failConfigure                  bool
	installs, configs, tests, runs int
	lastModel                      string
	runError                       error
}

func (*fixtureHost) Profile() Profile {
	return Profile{ID: "fixture-third", Name: "Fixture", MinimumContext: 8192, ContractRevision: "fixture-v1"}
}
func (f *fixtureHost) Inspect(context.Context, HostOptions) (HostState, error) {
	s := f.state
	s.ConfigDigest, _ = ConfigurationDigest(s.ConfigFiles)
	return s, nil
}
func (f *fixtureHost) InstallRuntime(context.Context, HostOptions) error {
	f.installs++
	f.state.Installed = true
	f.state.Version = "fixture-1"
	return nil
}
func (f *fixtureHost) InstallPlugin(context.Context, HostOptions, HostState) error {
	f.state.Plugin = true
	f.state.PluginDigest = "trusted"
	return nil
}
func (f *fixtureHost) Configure(_ context.Context, o HostOptions, _ HostState, setup SetupResponse, tx *Transaction) error {
	f.configs++
	if err := tx.VerifyUnchanged(); err != nil {
		return err
	}
	if f.failConfigure {
		return errors.New("injected config failure")
	}
	f.state.BaseURL = o.Endpoint()
	f.state.DefaultModel = setup.Integration.ModelID
	return nil
}
func (f *fixtureHost) Smoke(context.Context, HostOptions, HostState, SetupResponse) error {
	f.tests++
	return nil
}
func (f *fixtureHost) Run(_ context.Context, _ HostOptions, _ HostState, s SetupResponse, _ []string) error {
	f.runs++
	f.lastModel = s.Integration.ModelID
	return f.runError
}

func TestRunReportsTerminalStateAndLaunchProgress(t *testing.T) {
	for _, tc := range []struct {
		err   error
		phase string
	}{{nil, "process_exited"}, {context.Canceled, "cancelled"}, {context.DeadlineExceeded, "timed_out"}, {errors.New("failed start"), "failed"}} {
		m, host, _, o := managerFixture(t)
		host.runError = tc.err
		announced := false
		m.Progress = func(string) {
			announced = true
			if host.runs != 0 {
				t.Fatal("progress came after waiting")
			}
		}
		report, err := m.Run(context.Background(), "fixture-third", o, nil)
		if !announced || report.Phase != tc.phase || !errors.Is(err, tc.err) {
			t.Fatalf("phase=%s error=%v", report.Phase, err)
		}
	}
}

type fixtureService struct {
	capacity int
	fail     bool
}

func (f *fixtureService) Setup(_ context.Context, id, model, base string) (SetupResponse, error) {
	if f.fail {
		return SetupResponse{}, errors.New("busy")
	}
	if model == "" {
		model = "catalog-first"
	}
	return SetupResponse{Integration: ServiceStatus{Profile: Profile{ID: id, MinimumContext: 8192, ContractRevision: "fixture-v1"}, ModelID: model, ContextWindow: f.capacity, Ready: f.capacity >= 8192, Warnings: []string{}}, Setup: Setup{}}, nil
}

func managerFixture(t *testing.T) (*Manager, *fixtureHost, *fixtureService, HostOptions) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "external.json")
	if err := os.WriteFile(file, []byte(`{"user_field":"keep"}`), 0600); err != nil {
		t.Fatal(err)
	}
	host := &fixtureHost{state: HostState{Installed: true, Version: "fixture-1", Plugin: true, PluginDigest: "trusted", BaseURL: "http://localhost:11611/v1", DefaultModel: "external-default", DynamicModels: true, ConfigFiles: []string{file}}}
	service := &fixtureService{capacity: 65536}
	m := &Manager{Adapters: map[string]HostAdapter{"fixture-third": host}, Service: service, Store: &ConnectionStore{Directory: filepath.Join(dir, "state")}, Confirm: func(context.Context, string, string) (bool, error) { return true, nil }}
	return m, host, service, HostOptions{ServerURL: "http://localhost:11611"}
}

func TestThirdAdapterSharedLifecycleAndSelection(t *testing.T) {
	m, host, _, o := managerFixture(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		report, err := m.Configure(ctx, "fixture-third", withModel(o, "chosen"))
		if err != nil || !report.Ready || report.Phase != "configured" {
			t.Fatalf("configure: %+v %v", report, err)
		}
	}
	data, _ := os.ReadFile(host.state.ConfigFiles[0])
	if string(data) != `{"user_field":"keep"}` {
		t.Fatal("user config lost")
	}
	host.state.DefaultModel = "different-external-default"
	r, err := m.Status(ctx, "fixture-third", o)
	if err != nil || r.ModelID != "chosen" || r.SelectionSource != "managed_connection" {
		t.Fatalf("managed selection lost: %+v %v", r, err)
	}
	if host.tests != 0 || host.runs != 0 {
		t.Fatal("status performed inference")
	}
	if _, err = m.Run(ctx, "fixture-third", withModel(o, "per-run"), []string{"hello"}); err != nil || host.lastModel != "per-run" {
		t.Fatalf("run selection: %s %v", host.lastModel, err)
	}
	if _, err = m.Test(ctx, "fixture-third", withModel(o, "per-run")); err != nil {
		t.Fatal(err)
	}
	saved, _ := m.Store.Load("fixture-third", o)
	if saved.ModelID != "chosen" {
		t.Fatal("temporary selection replaced default")
	}
	r, _ = m.Status(ctx, "fixture-third", o)
	if r.Smoke.State != "stale" || r.Workflow.State != "unverified" {
		t.Fatalf("wrong evidence: %+v", r)
	}
	r, err = m.Test(ctx, "fixture-third", o)
	if err != nil || r.Smoke.State != "passed" {
		t.Fatalf("smoke: %+v %v", r, err)
	}
	host.state.Version = "fixture-2"
	r, _ = m.Status(ctx, "fixture-third", o)
	if r.Smoke.State != "stale" {
		t.Fatal("runtime upgrade reused evidence")
	}
}

func TestManagerContextAndOperationalStates(t *testing.T) {
	m, host, service, o := managerFixture(t)
	host.state.ContextWindow = 32768
	r, err := m.Status(context.Background(), "fixture-third", o)
	if err != nil || !r.Ready {
		t.Fatalf("spare context rejected: %+v %v", r, err)
	}
	service.capacity = 16384
	r, err = m.Run(context.Background(), "fixture-third", o, nil)
	if err == nil || host.runs != 0 {
		t.Fatal("oversized context dispatched")
	}
	if !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("misleading error: %v", err)
	}
	service.fail = true
	r, err = m.Status(context.Background(), "fixture-third", o)
	if err != nil || r.Ready {
		t.Fatalf("operational state: %+v %v", r, err)
	}
	if r.Checks[0].Code != "service_unavailable" {
		t.Fatal("busy classified as incompatibility")
	}
}

func TestConfigurationFailureIsRecoverableAndConsentPrecedesEffects(t *testing.T) {
	m, host, _, o := managerFixture(t)
	ctx := context.Background()
	m.Confirm = func(context.Context, string, string) (bool, error) { return false, nil }
	r, err := m.Configure(ctx, "fixture-third", o)
	if err != nil || r.Phase != "cancelled" || host.configs != 0 {
		t.Fatalf("consent: %+v %v", r, err)
	}
	m.Confirm = func(context.Context, string, string) (bool, error) { return true, nil }
	host.failConfigure = true
	r, err = m.Configure(ctx, "fixture-third", o)
	if err == nil || r.RecoveryPath == "" {
		t.Fatal("missing partial-state recovery")
	}
	journal, _ := os.ReadFile(filepath.Join(r.RecoveryPath, "journal.json"))
	if !strings.Contains(string(journal), "configuration_failed") {
		t.Fatalf("journal: %s", journal)
	}
	backup, _ := os.ReadFile(filepath.Join(r.RecoveryPath, "snapshot-0-0"))
	if string(backup) != `{"user_field":"keep"}` {
		t.Fatal("snapshot missing")
	}
	host.failConfigure = false
	if _, err = m.Configure(ctx, "fixture-third", o); err != nil {
		t.Fatalf("failed transaction retained lock: %v", err)
	}
}

func TestTransactionRejectsConcurrentWritesAndConnectionScopes(t *testing.T) {
	m, host, _, o := managerFixture(t)
	tx, err := m.Store.Begin("fixture-third", host.state.ConfigFiles)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Close()
	if _, err = m.Store.Begin("fixture-third", host.state.ConfigFiles); err == nil {
		t.Fatal("concurrent config accepted")
	}
	if err = os.WriteFile(host.state.ConfigFiles[0], []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if tx.VerifyUnchanged() == nil {
		t.Fatal("concurrent write accepted")
	}
	if err = m.Store.Save("fixture-third", o, Connection{ModelID: "private", BaseURL: o.Endpoint()}); err != nil {
		t.Fatal(err)
	}
	o.APIKey = "different-account"
	c, err := m.Store.Load("fixture-third", o)
	if err != nil || c.ModelID != "" {
		t.Fatal("cross-account connection reused")
	}
}

func TestManagedEndpointPersistsWithoutRepeatingBaseURL(t *testing.T) {
	m, _, _, o := managerFixture(t)
	o.BaseURL = "http://agent-reachable:11611"
	if _, err := m.Configure(context.Background(), "fixture-third", o); err != nil {
		t.Fatal(err)
	}
	o.BaseURL = ""
	r, err := m.Status(context.Background(), "fixture-third", o)
	if err != nil || !r.Ready {
		t.Fatalf("saved endpoint lost: %+v %v", r, err)
	}
}

func TestSmokeEvidenceInvalidatesOnConfigPluginAndContractChanges(t *testing.T) {
	for _, change := range []string{"config", "plugin", "contract"} {
		t.Run(change, func(t *testing.T) {
			m, host, _, o := managerFixture(t)
			if _, err := m.Configure(context.Background(), "fixture-third", o); err != nil {
				t.Fatal(err)
			}
			if _, err := m.Test(context.Background(), "fixture-third", o); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "config":
				if err := os.WriteFile(host.state.ConfigFiles[0], []byte(`{"user_field":"changed"}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "plugin":
				host.state.PluginDigest = "changed"
			case "contract":
				c, _ := m.Store.Load("fixture-third", o)
				c.ContractRevision = "old"
				if err := m.Store.Save("fixture-third", o, c); err != nil {
					t.Fatal(err)
				}
			}
			r, err := m.Status(context.Background(), "fixture-third", o)
			if err != nil || r.Smoke.State != "stale" {
				t.Fatalf("stale evidence reused: %+v %v", r, err)
			}
		})
	}
}

func TestLegacyHostSmokeDoesNotRequireReconfiguration(t *testing.T) {
	m, _, _, o := managerFixture(t)
	if _, err := m.Test(context.Background(), "fixture-third", o); err != nil {
		t.Fatal(err)
	}
	r, err := m.Status(context.Background(), "fixture-third", o)
	if err != nil || r.Smoke.State != "passed" {
		t.Fatalf("successful legacy-host test was lost: %+v %v", r, err)
	}
	r, err = m.Configure(context.Background(), "fixture-third", o)
	if err != nil || r.Smoke.State != "unverified" {
		t.Fatalf("configuration reported old smoke: %+v %v", r, err)
	}
}
