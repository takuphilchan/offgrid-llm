package server

import (
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

var contextTiers = [...]int{512, 1024, 2048, 4096, 8192, 16384, 32768, 64000, 65536, 131072, 262144, 524288, 1048576}

type contextPlanningInput struct {
	OperatorMax    int
	Adaptive       bool
	PolicyLimits   []int
	Model          *api.ModelMetadata
	Resources      *resource.SystemResources
	KVCacheType    string
	InferenceSlots int
	GPUEnabled     bool
}

type contextPlan struct {
	Window      int
	ModelMax    int
	ResourceMax int
}

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

func planContextWindow(input contextPlanningInput) contextPlan {
	window := resolveContextWindow(input.OperatorMax, input.Adaptive, input.PolicyLimits...)
	plan := contextPlan{Window: window}
	if input.Model != nil && input.Model.ContextSize > 0 {
		plan.ModelMax = input.Model.ContextSize
		if plan.ModelMax < plan.Window {
			plan.Window = plan.ModelMax
		}
	}
	if input.Adaptive {
		plan.ResourceMax = estimateContextCapacity(input.Model, input.Resources, input.KVCacheType, input.InferenceSlots, input.GPUEnabled)
		if plan.ResourceMax > 0 && plan.ResourceMax < plan.Window {
			plan.Window = plan.ResourceMax
		}
	}
	if input.Adaptive {
		plan.Window = contextTierAtOrBelow(plan.Window)
	}
	return plan
}

func estimateContextCapacity(model *api.ModelMetadata, resources *resource.SystemResources, cacheType string, slots int, gpuEnabled bool) int {
	// Unknown evidence stays useful for ordinary chat but cannot create a
	// long-context readiness claim.
	if model == nil || resources == nil || model.Size <= 0 {
		return 8192
	}
	modelMB := (model.Size + (1 << 20) - 1) >> 20
	availableRAM := resources.AvailableRAM
	if availableRAM <= 0 && resources.TotalRAM > 0 {
		availableRAM = resources.TotalRAM * 3 / 4
	}
	ramReserve := max(int64(1024), resources.TotalRAM/10)
	ramBudget := availableRAM - modelMB - ramReserve
	bestBudget := ramBudget
	if gpuEnabled && resources.GPUAvailable && resources.GPUMemory > 0 {
		availableGPU := resources.GPUAvailableMemory
		if availableGPU <= 0 {
			availableGPU = resources.GPUMemory
		}
		gpuBudget := availableGPU - modelMB - 768
		if gpuBudget > bestBudget {
			bestBudget = gpuBudget
		}
	}
	if bestBudget <= 0 {
		return 512
	}

	bytesPerToken := estimatedKVBytesPerToken(model, cacheType)
	if slots < 1 {
		slots = 1
	}
	bytesPerToken *= int64(slots)
	if bytesPerToken <= 0 {
		return 8192
	}
	raw := (bestBudget << 20) / bytesPerToken
	if raw > int64(^uint(0)>>1) {
		raw = int64(^uint(0) >> 1)
	}
	return contextTierAtOrBelow(int(raw))
}

func estimatedKVBytesPerToken(model *api.ModelMetadata, cacheType string) int64 {
	elements := model.KVElementsPerToken
	if elements <= 0 {
		switch {
		case model.Size <= 2<<30:
			elements = 16 << 10
		case model.Size <= 6<<30:
			elements = 32 << 10
		default:
			elements = 64 << 10
		}
	}
	switch strings.ToLower(strings.TrimSpace(cacheType)) {
	case "q4_0", "q4_1":
		return (elements*9 + 15) / 16
	case "q8_0":
		return (elements*17 + 15) / 16
	default: // f16 and unknown cache formats use the conservative two-byte size.
		return elements * 2
	}
}

func contextTierAtOrBelow(value int) int {
	if value < contextTiers[0] {
		return contextTiers[0]
	}
	result := contextTiers[0]
	for _, tier := range contextTiers {
		if tier > value {
			break
		}
		result = tier
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

func (s *Server) modelContextWindow(modelID string) int {
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
	var metadata *api.ModelMetadata
	if s.registry != nil && modelID != "" {
		metadata, _ = s.registry.GetChatModel(modelID)
	}
	return planContextWindow(contextPlanningInput{
		OperatorMax: s.config.MaxContextSize, Adaptive: s.config.AdaptiveContext,
		PolicyLimits: limits, Model: metadata, Resources: s.systemResources,
		KVCacheType: s.config.KVCacheType, InferenceSlots: s.config.InferenceSlots,
		GPUEnabled: s.config.EnableGPU,
	}).Window
}

func (s *Server) taskContextWindow(task *agents.Task) int {
	if task != nil && task.Config.ContextWindow > 0 {
		return task.Config.ContextWindow
	}
	if task != nil {
		return s.modelContextWindow(task.Model)
	}
	return s.effectiveContextWindow()
}

// Interactive chat can use a smaller allocation without changing the context
// advertised and allocated to external agent requests. Never truncate history;
// callers can select extended when a conversation exceeds this profile.
func (s *Server) chatContextWindowForModel(profile, modelID string) int {
	limit := s.modelContextWindow(modelID)
	if profile == "extended" {
		return limit
	}
	chat := 8192
	if s.config != nil && s.config.ChatContextSize > 0 {
		chat = s.config.ChatContextSize
	}
	return min(chat, limit)
}

func (s *Server) chatContextWindow(profile string) int {
	return s.chatContextWindowForModel(profile, "")
}
