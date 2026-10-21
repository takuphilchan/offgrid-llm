package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func runInteractive(ctx context.Context, args []string, env Environment) error {
	s := env.Style
	brandPrimary, brandSecondary, brandMuted := s.Primary, s.Secondary, s.Muted
	brandError, brandSuccess := s.Error, s.Success
	colorBold, colorDim, colorReset := s.Bold, s.Dim, s.Reset
	iconModel, iconCross := "model", s.Cross
	presenter := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, Style: s}

	// Check for help flag first
	helpRequested := false
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		helpRequested = true
		args = []string{} // Trigger help display
	}

	if len(args) < 1 {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sInteractive Chat%s\n", brandPrimary+colorBold, colorReset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sUsage%s  offgrid run %s<model>%s %s[options]%s\n", colorBold, colorReset, brandPrimary, colorReset, brandMuted, colorReset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sQuick Start (auto-downloads if needed)%s\n", colorBold, colorReset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid run %sllama3%s       %s# Llama 3.2 3B (~4GB RAM)%s\n", brandMuted, colorReset, brandPrimary, colorReset, colorDim, colorReset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid run %sqwen%s         %s# Qwen 2.5 3B (~4GB RAM)%s\n", brandMuted, colorReset, brandPrimary, colorReset, colorDim, colorReset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid run %smistral%s      %s# Mistral 7B (~8GB RAM)%s\n", brandMuted, colorReset, brandPrimary, colorReset, colorDim, colorReset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid run %stiny%s         %s# TinyLlama 1.1B (~2GB RAM)%s\n", brandMuted, colorReset, brandPrimary, colorReset, colorDim, colorReset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sOptions%s\n", colorBold, colorReset)
		fmt.Fprintf(env.Stdout, "    %s--save%s <name>      Save conversation to session\n", brandPrimary, colorReset)
		fmt.Fprintf(env.Stdout, "    %s--load%s <name>      Load and continue existing session\n", brandPrimary, colorReset)
		fmt.Fprintf(env.Stdout, "    %s--image%s <path>     Attach an image (for VLM models)\n", brandPrimary, colorReset)
		fmt.Fprintf(env.Stdout, "    %s--rag%s              Enable knowledge base (RAG)\n", brandPrimary, colorReset)
		fmt.Fprintf(env.Stdout, "    %s--allow-embedding-chat%s  Force chat with an embedding-like model\n", brandPrimary, colorReset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sMore Aliases%s\n", colorBold, colorReset)
		fmt.Fprintf(env.Stdout, "    %sphi%s, %sgemma%s, %sqwen2.5-coder%s, %scodellama%s, %sllava%s (vision)\n", brandPrimary, colorReset, brandPrimary, colorReset, brandPrimary, colorReset, brandPrimary, colorReset, brandPrimary, colorReset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sTip:%s Run %soffgrid alias list%s to see all shortcuts\n", brandMuted, colorReset, colorBold, colorReset)
		fmt.Fprintln(env.Stdout)
		if helpRequested {
			return nil
		}
		return ReportedError{}
	}

	modelName := args[0]
	var sessionName string
	var loadSession bool
	var saveSession bool
	var imagePath string
	var useKnowledgeBase bool
	var allowEmbeddingChat bool

	// Parse flags
	for i := 1; i < len(args); i++ {
		if args[i] == "--save" && i+1 < len(args) {
			sessionName = args[i+1]
			saveSession = true
			i++
		} else if args[i] == "--load" && i+1 < len(args) {
			sessionName = args[i+1]
			loadSession = true
		} else if args[i] == "--image" && i+1 < len(args) {
			imagePath = args[i+1]
			i++
		} else if args[i] == "--rag" {
			useKnowledgeBase = true
		} else if args[i] == "--allow-embedding-chat" {
			allowEmbeddingChat = true
		}
	}

	cfg, cfgErr := localConfig(env)
	if cfgErr != nil {
		return cfgErr
	}
	registry := models.NewRegistry(cfg.ModelsDir)

	// Scan for local models
	if err := registry.ScanModels(); err != nil {
		fmt.Fprintf(env.Stdout, "Error scanning models: %v\n", err)
		return ReportedError{}
	}

	// Try to resolve the model name (could be alias, partial name, or full name)
	resolvedModelPath := ""
	resolvedModelName := modelName

	// 1. First check if it's a direct model match
	model, err := registry.GetModel(modelName)
	if err == nil && model != nil {
		resolvedModelPath = model.Path
		resolvedModelName = model.ID
	} else {
		// 2. Check if it's a built-in alias
		if alias, ok := models.GetBuiltInAlias(modelName); ok {
			// Try to find a local model matching this alias
			localPath, err := models.FindLocalModelByAlias(cfg.ModelsDir, modelName)
			if err == nil {
				resolvedModelPath = localPath
				resolvedModelName = strings.TrimSuffix(filepath.Base(localPath), filepath.Ext(localPath))
			} else {
				// Model not found locally - offer to download
				fmt.Fprintln(env.Stdout)
				fmt.Fprintf(env.Stdout, "  %s%s Model Not Found%s\n", brandPrimary+colorBold, iconModel, colorReset)
				fmt.Fprintln(env.Stdout)
				fmt.Fprintf(env.Stdout, "  %s'%s'%s is an alias for %s%s%s\n", brandSecondary, modelName, colorReset, brandPrimary, alias.Pattern, colorReset)
				fmt.Fprintf(env.Stdout, "  %sRecommended:%s %s/%s\n", colorDim, colorReset, alias.HFRepo, alias.HFFile)
				fmt.Fprintf(env.Stdout, "  %sRAM needed:%s  ~%dGB\n", colorDim, colorReset, alias.RAM)
				fmt.Fprintln(env.Stdout)
				fmt.Fprintf(env.Stdout, "  %sDownload now?%s (Y/n): ", brandPrimary, colorReset)

				var response string
				fmt.Fscanln(env.Stdin, &response)
				response = strings.ToLower(strings.TrimSpace(response))
				if response == "" || response == "y" || response == "yes" {
					fmt.Fprintln(env.Stdout)
					// Download the model
					downloadArgs := []string{alias.HFRepo, "--file", alias.HFFile}
					if err := env.Interactive.DownloadHF(ctx, downloadArgs); err != nil {
						return err
					}

					// Re-scan and find the model
					registry.ScanModels()
					localPath, err = models.FindLocalModelByAlias(cfg.ModelsDir, modelName)
					if err != nil {
						fmt.Fprintln(env.Stdout)
						presenter.Error("Failed to find downloaded model")
						return ReportedError{}
					}
					resolvedModelPath = localPath
					resolvedModelName = strings.TrimSuffix(filepath.Base(localPath), filepath.Ext(localPath))
				} else {
					fmt.Fprintln(env.Stdout)
					presenter.Info("Aborted. To download manually:")
					fmt.Fprintf(env.Stdout, "  %s$%s offgrid download-hf %s %s\n", brandMuted, colorReset, alias.HFRepo, alias.HFFile)
					fmt.Fprintln(env.Stdout)
					return nil
				}
			}
		} else {
			// 3. Try fuzzy matching against local models
			localModels := registry.ListModels()
			var matches []string
			searchTerm := strings.ToLower(modelName)
			for _, m := range localModels {
				if strings.Contains(strings.ToLower(m.ID), searchTerm) {
					matches = append(matches, m.ID)
				}
			}

			if len(matches) == 1 {
				// Exact single match
				model, _ := registry.GetModel(matches[0])
				resolvedModelPath = model.Path
				resolvedModelName = model.ID
			} else if len(matches) > 1 {
				fmt.Fprintln(env.Stdout)
				presenter.Info(fmt.Sprintf("Multiple models match '%s':", modelName))
				for _, m := range matches {
					fmt.Fprintf(env.Stdout, "  • %s\n", m)
				}
				fmt.Fprintln(env.Stdout)
				presenter.Info("Please specify the full model name")
				return ReportedError{}
			} else {
				// No matches - show available aliases
				fmt.Fprintln(env.Stdout)
				fmt.Fprintf(env.Stdout, "  %s%s Model Not Found: %s%s\n", brandError, iconCross, modelName, colorReset)
				fmt.Fprintln(env.Stdout)
				fmt.Fprintf(env.Stdout, "  %sAvailable shortcuts:%s\n", colorBold, colorReset)
				fmt.Fprintf(env.Stdout, "    %sllama3%s      Llama 3.2 3B (~4GB RAM)\n", brandPrimary, colorReset)
				fmt.Fprintf(env.Stdout, "    %sqwen%s        Qwen 2.5 3B (~4GB RAM)\n", brandPrimary, colorReset)
				fmt.Fprintf(env.Stdout, "    %sphi%s         Phi 3.5 mini (~4GB RAM)\n", brandPrimary, colorReset)
				fmt.Fprintf(env.Stdout, "    %smistral%s    Mistral 7B (~8GB RAM)\n", brandPrimary, colorReset)
				fmt.Fprintf(env.Stdout, "    %stiny%s        TinyLlama 1.1B (~2GB RAM)\n", brandPrimary, colorReset)
				fmt.Fprintln(env.Stdout)
				fmt.Fprintf(env.Stdout, "  %sExample:%s offgrid run llama3\n", colorDim, colorReset)
				fmt.Fprintf(env.Stdout, "  %sList all:%s offgrid alias list\n", colorDim, colorReset)
				fmt.Fprintln(env.Stdout)
				return ReportedError{}
			}
		}
	}

	// Create a temporary model struct for the rest of the function
	if model == nil {
		model = &api.ModelMetadata{
			ID:   resolvedModelName,
			Path: resolvedModelPath,
		}
	}

	// Check for vision model
	isVLM := strings.Contains(strings.ToLower(resolvedModelName), "llava") ||
		strings.Contains(strings.ToLower(resolvedModelName), "vision") ||
		strings.Contains(strings.ToLower(resolvedModelName), "vlm")

	// Find projector file if this is a VLM
	var projectorPath string
	if isVLM {
		// Look for projector file in models directory
		files, _ := env.Interactive.ReadDir(cfg.ModelsDir)
		for _, f := range files {
			name := strings.ToLower(f.Name())
			if strings.Contains(name, "mmproj") || strings.Contains(name, "projector") {
				projectorPath = filepath.Join(cfg.ModelsDir, f.Name())
				break
			}
		}
	}
	_ = projectorPath // Silence unused warning if not VLM

	// Check available RAM before running the model
	sysResources, err := env.Interactive.DetectResources()
	if err == nil && sysResources.AvailableRAM > 0 {
		// Estimate RAM requirement for this model
		requiredRAM := EstimateRAMFromModel(modelName, "")

		// Convert available RAM from MB to GB for comparison
		availableGB := float64(sysResources.AvailableRAM) / 1024.0

		// Show warning if model requires more RAM than available
		if requiredRAM > 0 && requiredRAM > availableGB {
			fmt.Fprintln(env.Stdout)
			presenter.Warning(fmt.Sprintf("This model requires ~%.1fGB RAM, but you have ~%.1fGB available", requiredRAM, availableGB))
			fmt.Fprintln(env.Stdout)
			presenter.Info("The model may run slowly or fail to load. Consider:")
			fmt.Fprintln(env.Stdout, "  • Closing other applications to free memory")
			fmt.Fprintln(env.Stdout, "  • Using a smaller model (1B or 3B parameters)")
			fmt.Fprintln(env.Stdout, "  • Using a more aggressive quantization (Q2_K or Q3_K)")
			fmt.Fprintln(env.Stdout, "  • Upgrading your RAM")
			fmt.Fprintln(env.Stdout)
			presenter.Info("See docs/4GB_RAM.md for model recommendations")
			fmt.Fprintln(env.Stdout)
			fmt.Fprintf(env.Stdout, "%sContinue anyway?%s (y/N): ", brandMuted, colorReset)

			var response string
			fmt.Fscanln(env.Stdin, &response)
			if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
				fmt.Fprintln(env.Stdout)
				fmt.Fprintf(env.Stdout, "%sℹ%s Aborted. Use 'offgrid search --ram %.0f' to find suitable models\n", brandSecondary, colorReset, availableGB)
				fmt.Fprintln(env.Stdout)
				return nil
			}
			fmt.Fprintln(env.Stdout)
		}
	}

	if models.IsManagedPackagePath(model.Path) {
		presenter.Error("Speech packages are not chat models. Use their declared speech adapter.")
		return ReportedError{}
	}
	if env.Interactive.IsEmbeddingModel(modelName, resolvedModelName, resolvedModelPath, model.Path) && !allowEmbeddingChat {
		fmt.Fprintln(env.Stdout)
		presenter.Warning(env.Interactive.EmbeddingChatError(resolvedModelName))
		fmt.Fprintln(env.Stdout)
		presenter.Info("Use this model to power retrieval:")
		presenter.Item("Add documents", "offgrid kb add ./documents/")
		presenter.Item("Search locally", "offgrid kb search \"your question\"")
		presenter.Item("Ask with context", "offgrid run llama3 --rag")
		fmt.Fprintln(env.Stdout)
		presenter.Info("Force chat only if this model can generate text: offgrid run <model> --allow-embedding-chat")
		fmt.Fprintln(env.Stdout)
		return ReportedError{}
	}

	// Check if OffGrid API server is running and start it if needed
	if err := env.Interactive.EnsureServer(); err != nil {
		fmt.Fprintln(env.Stdout)
		presenter.Warning(fmt.Sprintf("OffGrid server not running: %v", err))
		presenter.Info("Starting OffGrid server in background...")
		if err := env.Interactive.StartServer(); err != nil {
			fmt.Fprintln(env.Stdout)
			presenter.Error("Failed to start OffGrid server")
			presenter.Info("Please start the server manually:")
			presenter.Item("Start server", "offgrid serve &")
			fmt.Fprintln(env.Stdout)
			return ReportedError{}
		}
		// Wait for server to be ready
		if err := env.Interactive.Wait(ctx, 2); err != nil {
			return err
		}
	}

	// Check if llama-server is running and start it if needed
	if err := env.Interactive.EnsureLlama(); err != nil {
		printRunHeader(env, filepath.Base(model.Path), useKnowledgeBase, "")
		fmt.Fprintf(env.Stdout, "  %sStatus:%s  Loading...", brandMuted, colorReset) // No newline - will be overwritten
		if err := env.Interactive.StartLlama(model.Path); err != nil {
			fmt.Fprintln(env.Stdout) // Add newline before error
			presenter.Error("Failed to start llama-server")
			fmt.Fprintln(env.Stdout)
			fmt.Fprintf(env.Stdout, "%sStart manually:%s\n", colorDim, colorReset)
			fmt.Fprintf(env.Stdout, "  %sllama-server -m %s --port 42382 &%s\n", brandSecondary, model.Path, colorReset)
			fmt.Fprintln(env.Stdout)
			return ReportedError{}
		}
		// Wait for llama-server to load the model

		// Read llama-server port from platform-specific config
		llamaPort := env.Interactive.ReadLlamaPort("42382")

		// Use the new robust readiness check (10 minutes for low-end machines and larger models)
		if err := env.Interactive.WaitModelReady(llamaPort, 600); err != nil {
			fmt.Fprintf(env.Stdout, "\r  %sStatus:%s  %sFailed%s\n", brandMuted, colorReset, brandError, colorReset)
			presenter.Error(fmt.Sprintf("Model failed to load: %v", err))
			return ReportedError{}
		}
		fmt.Fprintf(env.Stdout, "\r  %sStatus:%s  %sReady%s  \n", brandMuted, colorReset, brandSuccess, colorReset)

		// Show hardware info after loading
		if sysInfo, err := env.Interactive.DetectResources(); err == nil {
			if sysInfo.GPUAvailable {
				fmt.Fprintf(env.Stdout, "  %sGPU:%s     %s%s%s\n", brandMuted, colorReset, brandSuccess, sysInfo.GPUName, colorReset)
			} else {
				fmt.Fprintf(env.Stdout, "  %sDevice:%s  CPU (%d cores)\n", brandMuted, colorReset, sysInfo.CPUCores)
			}
		}
		fmt.Fprintln(env.Stdout)
	} else {
		// llama-server already running, show header
		printRunHeader(env, resolvedModelName, useKnowledgeBase, "")
		fmt.Fprintf(env.Stdout, "  %sStatus:%s  %sReady%s\n", brandMuted, colorReset, brandSuccess, colorReset)

		// Show hardware info
		if sysInfo, err := env.Interactive.DetectResources(); err == nil {
			if sysInfo.GPUAvailable {
				fmt.Fprintf(env.Stdout, "  %sGPU:%s     %s%s%s (%dMB VRAM)\n", brandMuted, colorReset, brandSuccess, sysInfo.GPUName, colorReset, sysInfo.GPUMemory)
			} else {
				fmt.Fprintf(env.Stdout, "  %sDevice:%s  CPU (%d cores, %dMB RAM available)\n", brandMuted, colorReset, sysInfo.CPUCores, sysInfo.AvailableRAM)
			}
		}
		fmt.Fprintln(env.Stdout)
	}

	// Setup session management
	sessionsDir := filepath.Join(cfg.DataDir, "sessions")
	sessionMgr := sessions.NewSessionManager(sessionsDir)

	var currentSession *sessions.Session

	// Load existing session or create new one
	if loadSession {
		sess, err := sessionMgr.Load(sessionName)
		if err != nil {
			presenter.Error(fmt.Sprintf("Failed to load session '%s': %v", sessionName, err))
			fmt.Fprintln(env.Stdout)
			presenter.Info("Available sessions:")
			sessionList, _ := sessionMgr.List()
			for _, s := range sessionList {
				fmt.Fprintf(env.Stdout, "  - %s\n", s.Name)
			}
			fmt.Fprintln(env.Stdout)
			return ReportedError{}
		}
		currentSession = sess
		presenter.Success(fmt.Sprintf("Loaded session '%s' (%d messages)", sessionName, sess.MessageCount()))
		fmt.Fprintln(env.Stdout)

		// Display previous conversation
		if sess.MessageCount() > 0 {
			presenter.Info("Previous conversation:")
			fmt.Fprintln(env.Stdout)
			for _, msg := range sess.Messages {
				if msg.Role == "user" {
					fmt.Fprintf(env.Stdout, "  %sYou:%s %s\n", brandPrimary, colorReset, msg.Content)
				} else {
					fmt.Fprintf(env.Stdout, "  %sAssistant:%s %s\n", brandSuccess, colorReset, msg.Content)
				}
			}
			fmt.Fprintln(env.Stdout)
		}
	} else if saveSession {
		currentSession = sessions.NewSession(sessionName, modelName)
		presenter.Info(fmt.Sprintf("Starting new session '%s' (will auto-save)", sessionName))
		fmt.Fprintln(env.Stdout)
	}

	printRunCommands(env)
	if currentSession != nil {
		fmt.Fprintf(env.Stdout, "  %sSession:%s   %s\n", brandMuted, colorReset, currentSession.Name)
	}
	if useKnowledgeBase {
		fmt.Fprintf(env.Stdout, "  %sRAG:%s       Enabled\n", brandMuted, colorReset)
	}
	fmt.Fprintln(env.Stdout)

	// Start chat session
	// Import required packages for HTTP client
	client := new(http.Client)
	*client = *env.HTTP
	client.Timeout = 300 * time.Second

	// Check if server is running
	healthURL := fmt.Sprintf("http://localhost:%d/health", cfg.ServerPort)
	resp, err := legacyGet(ctx, client, healthURL)
	if err != nil {
		fmt.Fprintln(env.Stdout)
		presenter.Error("Server not running")
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sStart server:%s\n", colorBold, colorReset)
		fmt.Fprintf(env.Stdout, "    %s$%s offgrid serve\n", brandMuted, colorReset)
		fmt.Fprintf(env.Stdout, "    %s$%s sudo systemctl start offgrid-llm\n", brandMuted, colorReset)
		fmt.Fprintln(env.Stdout)
		return ReportedError{}
	}
	resp.Body.Close()

	// Refresh server's model list to pick up newly downloaded models
	refreshURL := fmt.Sprintf("http://localhost:%d/models/refresh", cfg.ServerPort)
	if refreshResp, err := legacyPost(ctx, client, refreshURL, "application/json", nil); err == nil {
		refreshResp.Body.Close()
	}

	// Check which model is currently loaded in the server
	modelsURL := fmt.Sprintf("http://localhost:%d/v1/models", cfg.ServerPort)
	resp, err = legacyGet(ctx, client, modelsURL)
	if err == nil {
		defer resp.Body.Close()
		var modelsResp struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err == nil {
			// Check if requested model is in the active models list
			foundActive := false
			var activeModel string
			for _, m := range modelsResp.Data {
				if m.ID == resolvedModelName {
					foundActive = true
					break
				}
				activeModel = m.ID
			}

			if !foundActive && len(modelsResp.Data) > 0 {
				fmt.Fprintln(env.Stdout)
				presenter.Info(fmt.Sprintf("Switching model: %s -> %s", activeModel, resolvedModelName))
				fmt.Fprintf(env.Stdout, "%sLoading model...%s ", colorDim, colorReset)

				// Let the OffGrid server handle model switching by making a test request
				// The server's model cache will automatically load the new model
				testPayload := map[string]interface{}{
					"model": resolvedModelName,
					"messages": []map[string]string{
						{"role": "user", "content": "Hi"},
					},
					"max_tokens": 1,
					"stream":     false,
				}
				payloadBytes, _ := json.Marshal(testPayload)

				// Give the server time to switch models (may take a while for large models)
				switchClient := new(http.Client)
				*switchClient = *env.HTTP
				switchClient.Timeout = 180 * time.Second // 3 minute timeout for model loading

				switchResp, err := legacyPost(ctx, switchClient,
					fmt.Sprintf("http://localhost:%d/v1/chat/completions", cfg.ServerPort),
					"application/json",
					bytes.NewReader(payloadBytes),
				)

				if err != nil {
					fmt.Fprintf(env.Stdout, "%sFailed%s\n", brandError, colorReset)
					presenter.Error(fmt.Sprintf("Failed to switch model: %v", err))
					return ReportedError{}
				}
				switchResp.Body.Close()

				if switchResp.StatusCode != http.StatusOK {
					fmt.Fprintf(env.Stdout, "%sFailed%s\n", brandError, colorReset)
					presenter.Error(fmt.Sprintf("Failed to switch model (status %d)", switchResp.StatusCode))
					return ReportedError{}
				}

				fmt.Fprintf(env.Stdout, "%sReady%s\n", brandSuccess, colorReset)
				fmt.Fprintln(env.Stdout)
			}
		}
	}

	fmt.Fprintln(env.Stdout)

	// Interactive chat loop
	messages := []ChatMessage{}

	// Load messages from session if continuing
	if currentSession != nil && currentSession.MessageCount() > 0 {
		for _, msg := range currentSession.Messages {
			messages = append(messages, ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Handle initial image if provided
	if imagePath != "" {
		// Verify image file exists and is readable
		_, err := env.ReadFile(imagePath)
		if err != nil {
			presenter.Error(fmt.Sprintf("Failed to read image file: %v", err))
			return ReportedError{}
		}

		fmt.Fprintf(env.Stdout, "%sImage attached:%s %s\n", brandSuccess, colorReset, filepath.Base(imagePath))
		fmt.Fprintln(env.Stdout)

		// We'll attach this to the first user message
	}

	chatRuntime := &InteractiveTUIState{
		ModelName:        resolvedModelName,
		ResolvedModel:    resolvedModelName,
		ServerPort:       cfg.ServerPort,
		Client:           client,
		UseKnowledgeBase: useKnowledgeBase,
		ImagePath:        imagePath,
		Messages:         messages,
		SessionMgr:       sessionMgr,
		CurrentSession:   currentSession,
	}

	usedTUI := false
	if env.Interactive.ShouldUseTUI() {
		if err := env.Interactive.RunTUI(ctx, chatRuntime); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintln(env.Stdout)
			presenter.Warning(fmt.Sprintf("TUI unavailable, falling back to plain chat: %v", err))
		} else {
			usedTUI = true
			messages = chatRuntime.Messages
			imagePath = chatRuntime.ImagePath
			useKnowledgeBase = chatRuntime.UseKnowledgeBase
		}
	}

	if !usedTUI {
		if err := runPlainChat(ctx, env, cfg, client, resolvedModelName, useKnowledgeBase, imagePath, messages, currentSession, sessionMgr); err != nil {
			return err
		}
	}

	// Save session one final time on exit
	if currentSession != nil && (saveSession || loadSession) {
		if err := sessionMgr.Save(currentSession); err != nil {
			presenter.Warning(fmt.Sprintf("Failed to save session: %v", err))
		} else {
			presenter.Success(fmt.Sprintf("Session '%s' saved (%d messages)", currentSession.Name, currentSession.MessageCount()))
		}
		fmt.Fprintln(env.Stdout)
	}
	return nil
}
