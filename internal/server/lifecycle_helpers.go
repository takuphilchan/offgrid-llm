package server

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// startRAGAutoRestore starts the existing background restore at its original
// point in Start; the worker remains accounted for during Close.
func (s *Server) startRAGAutoRestore() {
	s.startupWorkers.Add(1)
	go func() {
		defer s.startupWorkers.Done()
		ctx, cancel := context.WithTimeout(s.runtimeCtx, 60*time.Second)
		defer cancel()

		// First try to restore persisted state
		if err := s.ragEngine.AutoRestore(ctx); err != nil {
			log.Printf("Warning: Failed to auto-restore RAG: %v", err)
		}

		// If not enabled yet, try to auto-enable with an available embedding model
		if !s.ragEngine.IsEnabled() {
			var modelNames []string
			for _, m := range s.registry.ListModels() {
				modelNames = append(modelNames, m.ID)
			}
			if err := s.ragEngine.AutoEnableWithModel(ctx, modelNames); err != nil {
				log.Printf("Warning: Failed to auto-enable RAG: %v", err)
			}
		}
	}()
}

// handleShutdown handles graceful shutdown
func (s *Server) handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	select {
	case <-s.runtimeCtx.Done():
		return
	case <-sigChan:
	}
	log.Println("\nShutdown signal received...")
	if err := s.Close(); err != nil {
		log.Printf("Runtime cleanup error: %v", err)
	}

	log.Println("Server stopped")
}

// collectSystemMetrics periodically updates system metrics
func (s *Server) collectSystemMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.runtimeCtx.Done():
			return
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			// Update memory metrics
			s.offgridMetrics.MemoryUsage.Set(float64(memStats.Alloc))

			// Update CPU usage (approximate from goroutine count)
			numGoroutines := runtime.NumGoroutine()
			s.offgridMetrics.CPUUsage.Set(float64(numGoroutines) / 100.0 * 10)

			// Update model loaded status
			if s.engine != nil && s.engine.IsLoaded() {
				s.offgridMetrics.ModelLoaded.Set(1)
			} else {
				s.offgridMetrics.ModelLoaded.Set(0)
			}

			// Update RAG metrics
			if s.ragEngine != nil {
				docs := s.ragEngine.ListDocuments()
				s.offgridMetrics.RAGDocumentsTotal.Set(float64(len(docs)))
			}

			// Update session count
			if s.sessionHandlers != nil {
				s.offgridMetrics.ActiveSessions.Set(float64(s.sessionHandlers.GetActiveSessionCount()))
			}

			// Update WebSocket connections
			s.offgridMetrics.WebSocketConnections.Set(float64(s.wsConnections))

			// Update user counts
			users := s.userStore.ListUsers()
			s.offgridMetrics.TotalUsers.Set(float64(len(users)))
			// Active users approximation - count users who logged in recently
			activeUsers := 0
			for _, u := range users {
				if u.LastLoginAt != nil && time.Since(*u.LastLoginAt) < 24*time.Hour {
					activeUsers++
				}
			}
			s.offgridMetrics.ActiveUsers.Set(float64(activeUsers))
		}
	}
}
