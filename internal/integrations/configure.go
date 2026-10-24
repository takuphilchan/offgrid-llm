package integrations

import (
	"context"
	"fmt"
)

func (m *Manager) Configure(ctx context.Context, id string, o HostOptions) (HostReport, error) {
	a, err := m.adapter(id)
	if err != nil {
		return HostReport{}, err
	}
	r, err := m.inspect(ctx, id, o)
	if err != nil {
		return r.report, err
	}
	if !r.setup.Integration.Ready {
		return r.report, blockedReport(r.report)
	}
	for _, check := range r.report.Checks {
		if (check.ID == "service" || check.ID == "model") && check.State == "blocked" {
			return r.report, blockedReport(r.report)
		}
	}
	o = r.options
	for _, check := range r.report.Checks {
		if check.Code == "runtime_inspection_failed" {
			return r.report, fmt.Errorf("%s", check.Message)
		}
	}
	r.report.Phase = "configuration_plan"
	if o.DryRun {
		return r.report, nil
	}
	if m.Confirm == nil {
		return r.report, fmt.Errorf("installation consent handler unavailable")
	}
	accepted, err := m.Confirm(ctx, id, "provider")
	if err != nil {
		return r.report, err
	}
	if !accepted {
		r.report.Phase = "cancelled"
		return r.report, nil
	}
	if !r.report.Runtime.Installed {
		accepted, err = m.Confirm(ctx, id, "runtime")
		if err != nil {
			return r.report, err
		}
		if !accepted {
			r.report.Phase = "cancelled"
			return r.report, nil
		}
		r.report.Phase = "runtime_install"
		if err = a.InstallRuntime(ctx, o); err != nil {
			return r.report, err
		}
		r.report.Runtime, err = a.Inspect(ctx, o)
		if err != nil {
			return r.report, err
		}
		if !r.report.Runtime.Installed {
			return r.report, fmt.Errorf("external installer exited without a discoverable runtime")
		}
	}
	tx, err := m.Store.Begin(id, r.report.Runtime.ConfigFiles)
	if err != nil {
		return r.report, err
	}
	defer tx.Close()
	r.report.RecoveryPath = tx.Directory
	phase := func(name string) error {
		r.report.Phase = name
		if m.Progress != nil {
			m.Progress(name)
		}
		return tx.Record(name)
	}
	if err = phase("plugin_install"); err != nil {
		return r.report, err
	}
	if err = tx.VerifyUnchanged(); err != nil {
		return r.report, err
	}
	if err = a.InstallPlugin(ctx, o, r.report.Runtime); err != nil {
		_ = tx.Record("plugin_failed")
		return r.report, fmt.Errorf("provider installation failed; recovery record: %s: %w", tx.Directory, err)
	}
	// Plugin registration can legitimately update the config. Keep a second
	// snapshot for the model-setting transaction without losing the first.
	if err = tx.Checkpoint(); err != nil {
		return r.report, err
	}
	if err = phase("configuration_apply"); err != nil {
		return r.report, err
	}
	if err = a.Configure(ctx, o, r.report.Runtime, r.setup, tx); err != nil {
		_ = tx.Record("configuration_failed")
		return r.report, fmt.Errorf("configuration stopped; inspect recovery record %s before retrying: %w", tx.Directory, err)
	}
	if err = phase("configuration_verify"); err != nil {
		return r.report, err
	}
	after, err := m.inspect(ctx, id, withModel(o, r.setup.Integration.ModelID))
	if err != nil {
		return r.report, err
	}
	after.report.RecoveryPath = tx.Directory
	if !after.report.Ready {
		_ = tx.Record("verification_failed")
		return after.report, blockedReport(after.report)
	}
	connection := Connection{Version: 1, ModelID: after.report.ModelID, BaseURL: o.Endpoint(), ContractRevision: a.Profile().ContractRevision, Smoke: Evidence{State: "unverified"}}
	if err = m.Store.Save(id, o, connection); err != nil {
		return after.report, err
	}
	after.report.Smoke = connection.Smoke
	if err = tx.Record("configured"); err != nil {
		return after.report, err
	}
	after.report.Phase = "configured"
	return after.report, nil
}
