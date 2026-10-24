package inference

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

const optionalToolsAdapter = "llama31-optional-tools-v1"
const resultToolsAdapter = "llama31-result-fidelity-v2"
const optionalToolsSourceSHA = "93c0e9aa3629bbd77e68dbc0f5621f6e6b23aa8d74b932595cdb8d64684526d7"
const optionalToolsGGUFSHA = "e10ca381b1ccc5cf9db52e371f3b6651576caee0a630b452e2816b2d404d4b65"
const optionalToolsAdaptedSHA = "16097d6b9fcdeb4f0feccd05d1cfbe2104014e2942aec584309301b37a45c8c4"
const resultToolsAdaptedSHA = "2df5298901b69af9c7d8a405c9499031df23f92b5785ce6f7c57c8a14e367c13"

//go:embed tool_result_guidance.txt
var toolResultGuidance string

// Selection is deliberately opt-in while real workflow qualification is open.
// No name/filename guessing, model file rewrite, download, or client template.
func prepareToolTemplate(ctx context.Context, binaryPath, modelPath string) ([]string, func(), error) {
	cleanup := func() {}
	selection := os.Getenv("OFFGRID_TOOL_TEMPLATE_ADAPTER")
	if selection == "" || selection == "native" {
		return nil, cleanup, nil
	}
	if selection != optionalToolsAdapter && selection != resultToolsAdapter {
		return nil, cleanup, fmt.Errorf("unknown OFFGRID_TOOL_TEMPLATE_ADAPTER selection")
	}
	identity, err := models.ReadGGUFTemplateIdentity(modelPath)
	// llama's /props trims the GGUF template's final newline. Match both exact
	// reviewed representations; never normalize arbitrary unknown templates.
	if templateSHA(identity.Template) == optionalToolsGGUFSHA {
		identity.Template = strings.TrimSuffix(identity.Template, "\n")
	}
	if err != nil || identity.Architecture != "llama" || templateSHA(identity.Template) != optionalToolsSourceSHA {
		return nil, cleanup, nil // Preserve unknown/malformed metadata's existing path.
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(probeCtx, binaryPath, "--version")
	configureBackgroundProcess(command)
	version, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return nil, cleanup, ctx.Err()
	}
	if err != nil || !toolTemplateRuntimeMatches(string(version), selection) {
		return nil, cleanup, nil
	}
	template, err := selectedToolFraming(identity.Template, selection)
	if err != nil {
		return nil, cleanup, err
	}
	if toolTemplateAdapterIdentity(templateSHA(template)) != selection {
		return nil, cleanup, fmt.Errorf("tool template adapter revision does not match its pinned output")
	}
	path, cleanup, err := privateToolTemplate(template)
	if err != nil {
		return nil, cleanup, err
	}
	return []string{"--chat-template-file", path}, cleanup, nil
}

func toolTemplateAdapterIdentity(digest string) string {
	switch digest {
	case optionalToolsAdaptedSHA:
		return optionalToolsAdapter
	case resultToolsAdaptedSHA:
		return resultToolsAdapter
	default:
		return ""
	}
}

func selectedToolFraming(source, selection string) (string, error) {
	if selection != optionalToolsAdapter && selection != resultToolsAdapter {
		return "", fmt.Errorf("unknown tool template adapter")
	}
	adapted, err := optionalToolFraming(source)
	if err != nil || selection == optionalToolsAdapter {
		return adapted, err
	}
	const anchor = "You may use the following functions when needed to answer the user."
	if strings.Count(adapted, anchor) != 1 {
		return "", fmt.Errorf("pinned tool-result framing did not match")
	}
	// This text is inside a native Jinja string literal. Do not add template
	// syntax or reinterpret any user/tool message as part of the template.
	guidance := strings.TrimSpace(toolResultGuidance)
	if strings.ContainsAny(guidance, "'\"\\\r\n") || strings.Contains(guidance, "{{") || strings.Contains(guidance, "{%") {
		return "", fmt.Errorf("invalid bundled tool-result framing")
	}
	return strings.Replace(adapted, anchor, guidance+" "+anchor, 1), nil
}

func templateSHA(value string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(value))) }

func toolTemplateRuntimeMatches(version, selection string) bool {
	if selection == optionalToolsAdapter {
		return optionalToolsRuntime(version)
	}
	if selection == resultToolsAdapter {
		for _, line := range strings.Split(version, "\n") {
			if strings.TrimSpace(line) == "version: 0.1.2-dev (build 1, commit b95502b-offgrid-seq2)" {
				return true
			}
		}
	}
	return false
}

func optionalToolsRuntime(version string) bool {
	// This is a compatibility pin, not a replacement for runtime asset trust.
	for _, line := range strings.Split(version, "\n") {
		if strings.TrimSpace(line) == "version: 0.1.2-dev (build 1, commit b95502b)" ||
			strings.TrimSpace(line) == "version: 0.1.2-dev (build 1, commit b95502b-offgrid-seq1)" ||
			strings.TrimSpace(line) == "version: 0.1.2-dev (build 1, commit b95502b-offgrid-seq2)" {
			return true
		}
	}
	return false
}

// Called only after the complete original template matches the reviewed hash.
// Keep native role tokens, all messages, tool schemas and result serialization.
func optionalToolFraming(source string) (string, error) {
	replacements := [][2]string{
		{"set tools_in_user_message = true", "set tools_in_user_message = false"},
		{"You have access to the following functions. To call a function, please respond with JSON for a function call.", "You may use the following functions when needed to answer the user. If no function is needed, answer the user normally in natural language. To call a function, respond with JSON for that one call."},
		{"Given the following functions, please respond with a JSON for a function call ", "Use a function only when needed. Otherwise answer normally in natural language. To call a function, respond with JSON "},
		{"with its proper arguments that best answers the given prompt.", "with its proper arguments. Wait for the actual function result before continuing; never invent a result or use a result placeholder as an argument."},
	}
	for _, pair := range replacements {
		if strings.Count(source, pair[0]) != 1 {
			return "", fmt.Errorf("pinned tool template framing did not match")
		}
		source = strings.Replace(source, pair[0], pair[1], 1)
	}
	return source, nil
}

func privateToolTemplate(template string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "offgrid-tool-template-")
	if err != nil {
		return "", func() {}, err
	}
	name := toolTemplateAdapterIdentity(templateSHA(template))
	if name == "" {
		name = "tool-template"
	}
	path := filepath.Join(dir, name+".jinja")
	cleanup := func() { _ = os.Remove(path); _ = os.Remove(dir) }
	if err := os.WriteFile(path, []byte(template), 0o600); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return path, cleanup, nil
}
