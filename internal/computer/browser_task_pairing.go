package computer

import "slices"

// The earlier stopped session is a resource identity, never renewed authority.
// A task-bound code copies its exact scope and cannot enroll a native executor.
type browserPairScope struct {
	previous string
	origin   string
	origins  []string
	features BrowserFeatures
}

func (h *BrowserHub) PairBrowserTaskCode(actor, run, input, workspace, previous string, mode ApprovalMode, requestedURLs ...string) (string, error) {
	if !opaqueID(run) || !opaqueID(input) || !opaqueID(workspace) || previous != "" && !opaqueID(previous) || !mode.Valid() || len(requestedURLs) > 1 {
		return "", ErrControlScope
	}
	var requested string
	if len(requestedURLs) == 1 {
		var err error
		requested, err = browserRequestedOrigin(requestedURLs[0])
		if err != nil {
			return "", err
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if previous == "" {
		if !validBrowserTaskBinding(actor, workspace, run, input) {
			return "", ErrControlScope
		}
		return h.pairCodeLocked(pairing{actor: actor, run: run, input: input, workspace: workspace, mode: mode, browser: &browserPairScope{origin: requested}})
	}
	s := h.sessions[previous]
	if s == nil || s.Actor != actor || s.Driver != "browser" || !s.stopped || s.run != run || s.ApprovalMode != mode {
		return "", ErrControlScope
	}
	p := pairing{actor: actor, run: run, input: input, workspace: workspace, mode: mode, browser: &browserPairScope{previous: previous, origin: s.Origin, origins: slices.Clone(s.BrowserOrigins), features: s.BrowserFeatures}}
	return h.pairCodeLocked(p)
}

func (h *BrowserHub) checkBrowserPairLocked(p pairing, origin, driver string, mode ApprovalMode, desktop bool, features BrowserFeatures, origins []string) error {
	scope := p.browser
	if scope.previous == "" {
		if desktop || driver != "browser" || mode != p.mode || features.Transfers != 1 || (scope.origin != "" && origin != scope.origin) || !validBrowserTaskBinding(p.actor, p.workspace, p.run, p.input) {
			return ErrControlScope
		}
		return nil
	}
	previous := h.sessions[scope.previous]
	if desktop || driver != "browser" || mode != p.mode || origin != scope.origin || features != scope.features || !slices.Equal(origins, scope.origins) || previous == nil || !previous.stopped || previous.Actor != p.actor || previous.run != p.run || previous.Driver != "browser" {
		return ErrControlScope
	}
	return nil
}
