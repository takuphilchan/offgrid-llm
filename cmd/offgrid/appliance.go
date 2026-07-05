package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

type applianceProfile struct {
	Name         string
	Label        string
	BestFor      string
	Hardware     []string
	Models       []string
	Settings     []string
	InstallNotes []string
}

var applianceProfiles = []applianceProfile{
	{
		Name:    "lite",
		Label:   "OffGrid Box Lite",
		BestFor: "small classrooms, workshops, and low-power learning stations",
		Hardware: []string{
			"Raspberry Pi 5 16GB or similar ARM64 board",
			"256GB+ SSD or NVMe storage",
			"Active cooling and reliable 5V/5A power",
			"Optional travel router for local Wi-Fi access",
		},
		Models: []string{
			"tiny",
			"phi",
			"qwen or llama3 in a 3B Q4 quant when RAM allows",
		},
		Settings: []string{
			"OFFGRID_LOW_MEMORY=true",
			"OFFGRID_MAX_CONTEXT=2048",
			"OFFGRID_USE_MMAP=true",
			"OFFGRID_MAX_MODELS=1",
		},
		InstallNotes: []string{
			"Use Linux arm64 as the native target.",
			"Keep the web UI and one model warm for students.",
			"Prefer USB model import over classroom-wide downloads.",
		},
	},
	{
		Name:    "hub",
		Label:   "OffGrid Community Hub",
		BestFor: "school labs, libraries, clinics, and local knowledge centers",
		Hardware: []string{
			"Refurbished x86_64 mini PC or small desktop",
			"32GB RAM",
			"1TB SSD",
			"Ethernet plus local Wi-Fi router or access point",
		},
		Models: []string{
			"llama3",
			"qwen",
			"mistral or codellama when 16GB+ RAM is free",
		},
		Settings: []string{
			"OFFGRID_MAX_CONTEXT=4096",
			"OFFGRID_MAX_MODELS=2",
			"OFFGRID_PREWARM_MODELS=true",
			"OFFGRID_FAST_SWITCH=true",
		},
		InstallNotes: []string{
			"Use Ubuntu Server or Debian as the native target.",
			"Expose the portal on the local LAN as http://offgrid.local.",
			"Use RAG packs for curriculum, health, agriculture, and local policy documents.",
		},
	},
	{
		Name:    "gpu",
		Label:   "OffGrid AI Lab",
		BestFor: "multi-user labs, coding clubs, teacher prep, and faster 7B/8B models",
		Hardware: []string{
			"x86_64 mini tower or workstation",
			"32GB to 64GB RAM",
			"NVIDIA GPU with 12GB+ VRAM",
			"1TB to 2TB SSD",
		},
		Models: []string{
			"llama3:8b",
			"mistral",
			"codellama",
			"llava for vision use cases",
		},
		Settings: []string{
			"OFFGRID_ENABLE_GPU=true",
			"OFFGRID_GPU_LAYERS=99",
			"OFFGRID_MAX_CONTEXT=8192",
			"OFFGRID_MAX_MODELS=3",
		},
		InstallNotes: []string{
			"Use Linux with NVIDIA drivers and CUDA-compatible llama.cpp binaries.",
			"Put the device on a UPS when power cuts are common.",
			"Use admin accounts for teachers and guest access for students.",
		},
	},
	{
		Name:    "jetson",
		Label:   "OffGrid Jetson Edge Kit",
		BestFor: "robotics, camera, voice, and field AI prototypes",
		Hardware: []string{
			"NVIDIA Jetson Orin Nano class device",
			"8GB+ RAM",
			"256GB+ NVMe storage",
			"Active cooling and stable DC power",
		},
		Models: []string{
			"tiny",
			"phi",
			"qwen or llama3 3B Q4",
			"llava only when performance is acceptable",
		},
		Settings: []string{
			"OFFGRID_ENABLE_GPU=true",
			"OFFGRID_LOW_MEMORY=true",
			"OFFGRID_MAX_CONTEXT=2048",
			"OFFGRID_MAX_MODELS=1",
		},
		InstallNotes: []string{
			"Use JetPack Linux as the native target.",
			"Prioritize small models and short context for responsiveness.",
			"Good fit for classroom robotics and offline multimodal demos.",
		},
	},
}

func handleAppliance(args []string) {
	if len(args) == 0 || isHelpArg(args[0]) {
		printApplianceHelp()
		return
	}

	switch strings.ToLower(args[0]) {
	case "status":
		handleApplianceStatus()
	case "plan":
		profile := ""
		if len(args) > 1 {
			profile = args[1]
		}
		handleAppliancePlan(profile)
	case "profile", "profiles":
		profile := ""
		if len(args) > 1 {
			profile = args[1]
		}
		handleApplianceProfile(profile)
	case "init":
		profile := "hub"
		if len(args) > 1 {
			profile = args[1]
		}
		handleApplianceInit(profile)
	default:
		printError("unknown appliance command: " + args[0])
		printApplianceHelp()
		os.Exit(1)
	}
}

func printApplianceHelp() {
	fmt.Println()
	fmt.Printf("  %sOffGrid Appliance%s\n", brandPrimary+colorBold, colorReset)
	fmt.Printf("  %sPlan and prepare offline AI boxes for schools and communities%s\n", brandMuted, colorReset)
	fmt.Println()
	fmt.Printf("  %sUsage%s  offgrid appliance %s<command>%s\n", colorBold, colorReset, brandPrimary, colorReset)
	fmt.Println()
	printOption("status", "Show this machine's appliance fit")
	printOption("plan [profile]", "Show hardware and deployment plan")
	printOption("profile [name]", "Show profile tuning settings")
	printOption("init [profile]", "Create local appliance metadata")
	fmt.Println()
	fmt.Printf("  %sProfiles%s  lite, hub, gpu, jetson\n", brandPrimary, colorReset)
	fmt.Println()
	fmt.Printf("  %sExamples%s\n", brandPrimary, colorReset)
	fmt.Printf("    %s$%s offgrid appliance status\n", brandMuted, colorReset)
	fmt.Printf("    %s$%s offgrid appliance plan hub\n", brandMuted, colorReset)
	fmt.Printf("    %s$%s offgrid appliance init lite\n", brandMuted, colorReset)
	fmt.Println()
}

func handleApplianceStatus() {
	printSection("Appliance Status")

	res, err := resource.DetectResources()
	if err != nil {
		printWarning(fmt.Sprintf("Hardware detection incomplete: %v", err))
	}

	if res != nil {
		printItem("OS", fmt.Sprintf("%s/%s", res.OS, res.Arch))
		printItem("CPU", fmt.Sprintf("%d logical cores", res.CPUCores))
		printItem("RAM", fmt.Sprintf("%d MB total, %d MB available", res.TotalRAM, res.AvailableRAM))
		if res.GPUAvailable {
			printItem("GPU", fmt.Sprintf("%s (%d MB VRAM)", res.GPUName, res.GPUMemory))
		} else {
			printItem("GPU", "not detected")
		}
	}

	cfg := config.LoadConfig()
	paths := config.GetDefaultPaths()
	printItem("Models", cfg.ModelsDir)
	printItem("Config", paths.ConfigDir)
	printItem("Portal", fmt.Sprintf("http://localhost:%d", cfg.ServerPort))

	profile := recommendApplianceProfile(res)
	fmt.Println()
	printSuccess(fmt.Sprintf("Recommended profile: %s (%s)", profile.Name, profile.Label))
	fmt.Printf("  %sBest for:%s %s\n", brandMuted, colorReset, profile.BestFor)
	fmt.Println()
	fmt.Printf("  %sNext:%s offgrid appliance plan %s\n", brandMuted, colorReset, profile.Name)
	fmt.Println()
}

func handleAppliancePlan(name string) {
	if name == "" {
		name = "hub"
	}
	profile, ok := findApplianceProfile(name)
	if !ok {
		printError("unknown profile: " + name)
		printApplianceProfiles()
		os.Exit(1)
	}

	printSection(profile.Label)
	printItem("Profile", profile.Name)
	printItem("Best for", profile.BestFor)
	fmt.Println()
	printApplianceList("Hardware", profile.Hardware)
	printApplianceList("Models", profile.Models)
	printApplianceList("Deployment notes", profile.InstallNotes)

	fmt.Printf("  %sRecommended flow%s\n", brandPrimary, colorReset)
	fmt.Printf("    %s1.%s Install Linux and OffGrid\n", brandMuted, colorReset)
	fmt.Printf("    %s2.%s Run offgrid appliance init %s\n", brandMuted, colorReset, profile.Name)
	fmt.Printf("    %s3.%s Import models from USB or download once\n", brandMuted, colorReset)
	fmt.Printf("    %s4.%s Start the portal with offgrid serve\n", brandMuted, colorReset)
	fmt.Printf("    %s5.%s Add curriculum and community documents with offgrid kb add\n", brandMuted, colorReset)
	fmt.Println()
}

func handleApplianceProfile(name string) {
	if name == "" {
		printApplianceProfiles()
		return
	}

	profile, ok := findApplianceProfile(name)
	if !ok {
		printError("unknown profile: " + name)
		printApplianceProfiles()
		os.Exit(1)
	}

	printSection(profile.Label + " Settings")
	printApplianceList("Environment", profile.Settings)
	printApplianceList("Good models", profile.Models)
	fmt.Printf("  %sApply manually in your shell, service file, or appliance env file.%s\n", brandMuted, colorReset)
	fmt.Println()
}

func handleApplianceInit(name string) {
	profile, ok := findApplianceProfile(name)
	if !ok {
		printError("unknown profile: " + name)
		printApplianceProfiles()
		os.Exit(1)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		printError("could not locate home directory")
		os.Exit(1)
	}

	dir := filepath.Join(homeDir, ".offgrid", "appliance")
	if err := os.MkdirAll(dir, 0755); err != nil {
		printError(fmt.Sprintf("failed to create appliance directory: %v", err))
		os.Exit(1)
	}

	envPath := filepath.Join(dir, "appliance.env")
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(envPath, []byte(applianceEnv(profile)), 0644); err != nil {
		printError(fmt.Sprintf("failed to write %s: %v", envPath, err))
		os.Exit(1)
	}
	if err := os.WriteFile(readmePath, []byte(applianceReadme(profile)), 0644); err != nil {
		printError(fmt.Sprintf("failed to write %s: %v", readmePath, err))
		os.Exit(1)
	}

	printSection("Appliance Initialized")
	printItem("Profile", profile.Name)
	printItem("Directory", dir)
	printItem("Env file", envPath)
	printItem("Guide", readmePath)
	fmt.Println()
	fmt.Printf("  %sNext:%s review the env file, import models, then run offgrid serve\n", brandMuted, colorReset)
	fmt.Println()
}

func printApplianceProfiles() {
	printSection("Appliance Profiles")
	for _, profile := range applianceProfiles {
		fmt.Printf("  %s%-8s%s %s\n", colorBold, profile.Name, colorReset, profile.Label)
		fmt.Printf("           %s%s%s\n", brandMuted, profile.BestFor, colorReset)
	}
	fmt.Println()
}

func printApplianceList(title string, items []string) {
	fmt.Printf("  %s%s%s\n", brandPrimary, title, colorReset)
	for _, item := range items {
		fmt.Printf("    %s-%s %s\n", brandMuted, colorReset, item)
	}
	fmt.Println()
}

func findApplianceProfile(name string) (applianceProfile, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, profile := range applianceProfiles {
		if profile.Name == name {
			return profile, true
		}
	}
	return applianceProfile{}, false
}

func recommendApplianceProfile(res *resource.SystemResources) applianceProfile {
	if res == nil {
		profile, _ := findApplianceProfile("hub")
		return profile
	}
	if res.GPUAvailable && res.GPUMemory >= 10000 {
		profile, _ := findApplianceProfile("gpu")
		return profile
	}
	if res.Arch == "arm64" || res.Arch == "arm" {
		if runtime.GOOS == "linux" && res.GPUAvailable {
			profile, _ := findApplianceProfile("jetson")
			return profile
		}
		profile, _ := findApplianceProfile("lite")
		return profile
	}
	if res.TotalRAM >= 24000 {
		profile, _ := findApplianceProfile("hub")
		return profile
	}
	profile, _ := findApplianceProfile("lite")
	return profile
}

func applianceEnv(profile applianceProfile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# OffGrid appliance profile\n")
	fmt.Fprintf(&b, "# Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "OFFGRID_APPLIANCE_PROFILE=%s\n", profile.Name)
	for _, setting := range profile.Settings {
		fmt.Fprintf(&b, "%s\n", setting)
	}
	return b.String()
}

func applianceReadme(profile applianceProfile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", profile.Label)
	fmt.Fprintf(&b, "Best for: %s\n\n", profile.BestFor)
	fmt.Fprintf(&b, "## Hardware\n\n")
	for _, item := range profile.Hardware {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	fmt.Fprintf(&b, "\n## Models\n\n")
	for _, item := range profile.Models {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	fmt.Fprintf(&b, "\n## Start\n\n")
	fmt.Fprintf(&b, "```bash\n")
	fmt.Fprintf(&b, "set -a\n")
	fmt.Fprintf(&b, ". ./appliance.env\n")
	fmt.Fprintf(&b, "set +a\n")
	fmt.Fprintf(&b, "offgrid serve\n")
	fmt.Fprintf(&b, "```\n")
	return b.String()
}

func isHelpArg(arg string) bool {
	return arg == "help" || arg == "--help" || arg == "-h"
}
