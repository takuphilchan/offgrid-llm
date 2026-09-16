package server

// resolveContextWindow returns the context window that will actually be
// allocated to llama-server. Explicit context settings are authoritative when
// adaptive context is disabled; otherwise current power and resource limits
// are applied before the model process is launched.
func resolveContextWindow(configured int, adaptive bool, limits ...int) int {
	if configured < 1 {
		configured = 4096
	}
	if !adaptive {
		return configured
	}
	result := configured
	for _, limit := range limits {
		if limit > 0 && limit < result {
			result = limit
		}
	}
	return result
}

func (s *Server) effectiveContextWindow() int {
	if s == nil || s.config == nil {
		return 4096
	}
	limits := make([]int, 0, 2)
	if s.powerManager != nil {
		limits = append(limits, s.powerManager.GetMaxContext())
	}
	if s.degradationMgr != nil {
		limits = append(limits, s.degradationMgr.MaxContextSize())
	}
	return resolveContextWindow(s.config.MaxContextSize, s.config.AdaptiveContext, limits...)
}

// Interactive chat can use a smaller allocation without changing the context
// advertised and allocated to external agent requests. Never truncate history;
// callers can select extended when a conversation exceeds this profile.
func (s *Server) chatContextWindow(profile string) int {
	limit := s.effectiveContextWindow()
	if profile == "extended" {
		return limit
	}
	chat := 8192
	if s.config != nil && s.config.ChatContextSize > 0 {
		chat = s.config.ChatContextSize
	}
	return min(chat, limit)
}
