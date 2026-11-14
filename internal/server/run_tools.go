package server

import "github.com/takuphilchan/offgrid-llm/internal/agents"

// All conversation execution uses this shared service-owned capability broker.
type browserRunTools struct {
	agents.RunTools
	server *Server
	// Accessed only by the runner's serialized native preparation controller.
	integrationObservationCursor string
}
