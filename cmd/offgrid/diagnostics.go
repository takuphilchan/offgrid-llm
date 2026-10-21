package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
	"net/http"
	"strings"
	"time"
)

func handleAutoSelect(args []string) int {
	fmt.Println()
	fmt.Printf("  %s◈ Model Recommendations%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  %sAnalyzing your system for optimal model selection%s\n", colorDim, colorReset)
	fmt.Println()

	// Detect hardware resources
	res, err := resource.DetectResources()
	if err != nil {
		printError(fmt.Sprintf("Failed to detect system resources: %v", err))
		return 1
	}

	// Display system info in a compact format
	fmt.Printf("  %sSystem%s\n", brandPrimary, colorReset)
	fmt.Printf("    OS        %s%s/%s%s\n", colorDim, res.OS, res.Arch, colorReset)
	fmt.Printf("    CPU       %s%d cores%s\n", colorDim, res.CPUCores, colorReset)
	fmt.Printf("    RAM       %s%s total · %s available%s\n", colorDim,
		formatBytes(res.TotalRAM*1024*1024),
		formatBytes(res.AvailableRAM*1024*1024), colorReset)

	if res.GPUAvailable {
		fmt.Printf("    GPU       %s%s · %s VRAM%s\n", colorDim, res.GPUName, formatBytes(res.GPUMemory*1024*1024), colorReset)
	} else {
		fmt.Printf("    GPU       %sNot detected (CPU-only)%s\n", brandMuted, colorReset)
	}
	fmt.Println()

	// Get recommendations
	recommendations := res.RecommendedModels()

	if len(recommendations) == 0 {
		fmt.Println()
		printWarning("Insufficient memory for any models")
		fmt.Println()
		printInfo("Minimum requirements:")
		printItem("RAM", "2 GB available")
		fmt.Println()
		return 1
	}

	// Group by priority
	primary := []resource.ModelRecommendation{}
	alternatives := []resource.ModelRecommendation{}
	supplementary := []resource.ModelRecommendation{}

	for _, rec := range recommendations {
		switch rec.Priority {
		case 1:
			primary = append(primary, rec)
		case 2:
			alternatives = append(alternatives, rec)
		case 3:
			supplementary = append(supplementary, rec)
		}
	}

	// Display recommendations
	fmt.Printf("  %sRecommended Models%s\n", brandPrimary, colorReset)
	fmt.Println()

	if len(primary) > 0 {
		for i, rec := range primary {
			fmt.Printf("    %s%d.%s %s%s%s %s(%s)%s\n",
				brandMuted, i+1, colorReset,
				colorBold, rec.ModelID, colorReset,
				brandMuted, rec.Quantization, colorReset)
			fmt.Printf("       %s%s · %s%s\n",
				colorDim, formatModelSize(rec.SizeGB), rec.Reason, colorReset)
		}
		fmt.Println()
	}

	if len(alternatives) > 0 {
		fmt.Printf("  %sAlternatives%s\n", brandMuted, colorReset)
		for _, rec := range alternatives {
			fmt.Printf("    %s◦%s %s %s(%s)%s\n",
				brandMuted, colorReset,
				rec.ModelID, brandMuted, rec.Quantization, colorReset)
			fmt.Printf("       %s%s · %s%s\n",
				colorDim, formatModelSize(rec.SizeGB), rec.Reason, colorReset)
		}
		fmt.Println()
	}

	if len(supplementary) > 0 {
		fmt.Printf("  %sSupplementary%s\n", brandMuted, colorReset)
		for _, rec := range supplementary {
			fmt.Printf("    %s◦%s %s %s(%s)%s\n",
				brandMuted, colorReset,
				rec.ModelID, brandMuted, rec.Quantization, colorReset)
			fmt.Printf("       %s%s · %s%s\n",
				colorDim, formatModelSize(rec.SizeGB), rec.Reason, colorReset)
		}
		fmt.Println()
	}

	fmt.Printf("  %sNext Steps%s\n", brandPrimary, colorReset)
	if len(primary) > 0 {
		// Use catalog download with proper model ID
		fmt.Printf("    %s$%s offgrid download %s\n", colorDim, colorReset, primary[0].ModelID)
		fmt.Printf("    %s$%s offgrid list %s# View installed models%s\n", colorDim, colorReset, colorDim, colorReset)
	}
	fmt.Println()
	return 0
}

func handleTest(args []string) int {
	fmt.Println()
	fmt.Printf("  %s◈ System Test%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  %sTesting model switching and chat completions%s\n", colorDim, colorReset)
	fmt.Println()

	cfg := config.LoadConfig()
	serverURL := fmt.Sprintf("http://localhost:%d", cfg.ServerPort)

	// Check if server is running
	fmt.Printf("  %sServer%s\n", brandPrimary, colorReset)
	resp, err := http.Get(serverURL + "/v1/health")
	if err != nil {
		fmt.Printf("%s✗%s Server not running on port %d\n", brandError, colorReset, cfg.ServerPort)
		fmt.Printf("%sℹ%s Start server with: offgrid serve\n", colorDim, colorReset)
		fmt.Printf("%s%s\n", brandPrimary, colorReset)
		fmt.Println()
		return 1
	}
	resp.Body.Close()
	fmt.Printf("%s✓%s Server running\n", brandSuccess, colorReset)

	// Load models
	fmt.Printf("%sLoading Models%s\n", brandPrimary+colorBold, colorReset)
	resp, err = http.Get(serverURL + "/models")
	if err != nil {
		fmt.Printf("%s✗%s Failed to fetch models: %v\n", brandError, colorReset, err)
		fmt.Printf("%s%s\n", brandPrimary, colorReset)
		fmt.Println()
		return 1
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	modelsData := result["data"]
	if modelsData == nil {
		fmt.Printf("%s✗%s No models found\n", brandError, colorReset)
		fmt.Printf("%s%s\n", brandPrimary, colorReset)
		fmt.Println()
		return 1
	}

	modelsArray, ok := modelsData.([]interface{})
	if !ok || len(modelsArray) == 0 {
		fmt.Printf("%s✗%s No models available\n", brandError, colorReset)
		fmt.Printf("%s%s\n", brandPrimary, colorReset)
		fmt.Println()
		return 1
	}

	fmt.Printf("%s✓%s Found %d models\n", brandSuccess, colorReset, len(modelsArray))

	// Test with up to 2 models
	testModels := []string{}
	for i, m := range modelsArray {
		if i >= 2 {
			break
		}
		modelMap := m.(map[string]interface{})
		modelID := modelMap["id"].(string)
		testModels = append(testModels, modelID)
		fmt.Printf("%s• %s%s\n", colorDim, modelID, colorReset)
	}

	// Run tests
	fmt.Printf("%sRunning Tests%s\n", brandPrimary+colorBold, colorReset)

	for i, modelID := range testModels {
		fmt.Printf("%sTest %d/%d:%s %s\n", brandSecondary, i+1, len(testModels), colorReset, modelID)

		startTime := time.Now()

		payload := map[string]interface{}{
			"model": modelID,
			"messages": []map[string]string{
				{"role": "user", "content": "Say just hello"},
			},
			"max_tokens": 10,
			"stream":     false,
		}

		jsonData, _ := json.Marshal(payload)
		resp, err := http.Post(
			serverURL+"/v1/chat/completions",
			"application/json",
			bytes.NewBuffer(jsonData),
		)

		if err != nil {
			fmt.Printf("%s✗%s Request failed: %v\n", brandError, colorReset, err)
			continue
		}

		var chatResult map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&chatResult)
		resp.Body.Close()

		duration := time.Since(startTime)

		if choices, ok := chatResult["choices"].([]interface{}); ok && len(choices) > 0 {
			choice := choices[0].(map[string]interface{})
			message := choice["message"].(map[string]interface{})
			content := message["content"].(string)

			trimmedContent := strings.TrimSpace(content)
			if trimmedContent == "" {
				fmt.Printf("%s⚠%s Response: (empty response)\n", brandAccent, colorReset)
			} else {
				// Limit output to first 60 chars
				displayContent := trimmedContent
				if len(displayContent) > 60 {
					displayContent = displayContent[:60] + "..."
				}
				fmt.Printf("%s✓%s Response: %s\n", brandSuccess, colorReset, displayContent)
			}
			fmt.Printf("%s⏱%s  Time: %s\n", colorDim, colorReset, formatDuration(duration))
		} else {
			fmt.Printf("%s✗%s Invalid response format\n", brandError, colorReset)
		}
	}

	fmt.Printf("%sAll Tests Complete%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println()
	return 0
}

func handlePeers(args []string) {
	cfg := config.LoadConfig()

	fmt.Println()
	fmt.Printf("  %sP2P Network%s\n", brandPrimary+colorBold, colorReset)
	fmt.Println()
	fmt.Printf("    %sMaturity%s    Beta - use USB import/export for critical deployments\n", brandMuted, colorReset)
	fmt.Println()

	if !cfg.EnableP2P {
		fmt.Printf("    %sP2P is disabled.%s\n\n", brandMuted, colorReset)
		fmt.Printf("    %sEnable P2P to share models with other OffGrid instances%s\n", colorDim, colorReset)
		fmt.Printf("    %son your local network without internet access.%s\n\n", colorDim, colorReset)
		fmt.Printf("    %sTo enable:%s\n", colorBold, colorReset)
		fmt.Printf("      %s$%s offgrid config set p2p.enabled true\n", brandMuted, colorReset)
		fmt.Printf("      %s$%s offgrid serve  %s# Restart server%s\n", brandMuted, colorReset, brandMuted, colorReset)
		fmt.Println()
		fmt.Printf("    %sP2P allows you to:%s\n", colorBold, colorReset)
		fmt.Printf("      %s-%s Discover other OffGrid nodes on your network\n", brandPrimary, colorReset)
		fmt.Printf("      %s-%s Transfer models directly between machines\n", brandPrimary, colorReset)
		fmt.Printf("      %s-%s Share models in air-gapped environments\n", brandPrimary, colorReset)
		fmt.Println()
		return
	}

	// Query running server for peer info
	serverURL := fmt.Sprintf("http://%s:%d", cfg.ServerHost, cfg.ServerPort)
	resp, err := http.Get(serverURL + "/v1/p2p/peers")
	if err != nil {
		fmt.Printf("    %sServer not running.%s\n\n", brandMuted, colorReset)
		fmt.Printf("    Start the server first:\n")
		fmt.Printf("      %s$%s offgrid serve\n", brandMuted, colorReset)
		fmt.Println()
		return
	}
	defer resp.Body.Close()

	var peers []struct {
		ID       string   `json:"id"`
		Address  string   `json:"address"`
		Port     int      `json:"port"`
		Models   []string `json:"models"`
		LastSeen string   `json:"last_seen"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
		printError(fmt.Sprintf("Failed to parse response: %v", err))
		return
	}

	fmt.Printf("    %sStatus%s       P2P enabled (port %d, discovery %d)\n", brandMuted, colorReset, cfg.P2PPort, cfg.DiscoveryPort)

	if len(peers) == 0 {
		fmt.Printf("    %sPeers%s        No peers discovered yet\n", brandMuted, colorReset)
		fmt.Println()
		fmt.Printf("    %sWaiting for other OffGrid instances on your network...%s\n", colorDim, colorReset)
		fmt.Printf("    %sEnsure they have P2P enabled and are on the same subnet.%s\n", colorDim, colorReset)
	} else {
		fmt.Printf("    %sPeers%s        %d node(s) discovered\n", brandMuted, colorReset, len(peers))
		fmt.Println()

		for _, peer := range peers {
			fmt.Printf("    %s-%s %s%s%s\n", brandPrimary, colorReset, colorBold, peer.ID, colorReset)
			fmt.Printf("      %sAddress%s  %s:%d\n", brandMuted, colorReset, peer.Address, peer.Port)
			if len(peer.Models) > 0 {
				fmt.Printf("      %sModels%s   %s\n", brandMuted, colorReset, strings.Join(peer.Models, ", "))
			}
		}
	}

	fmt.Println()
	fmt.Printf("    %sTransfer a model from a peer:%s\n", colorBold, colorReset)
	fmt.Printf("      %s$%s offgrid peers download <model> <peer-id>\n", brandMuted, colorReset)
	fmt.Println()
}
