package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type ModelPackageDependencies struct {
	NewRequestID func() string
}

type modelPackageOptions struct {
	command, subject, category, revision, variant, architecture, requestID string
	yes, detach                                                            bool
}

func parseModelPackageOptions(args []string) (modelPackageOptions, error) {
	o := modelPackageOptions{command: "help"}
	if len(args) == 0 {
		return o, nil
	}
	o.command = args[0]
	for i := 1; i < len(args); i++ {
		switch a := args[i]; a {
		case "--yes", "-y":
			o.yes = true
		case "--detach":
			o.detach = true
		case "--category", "--revision", "--variant", "--architecture", "--request-id":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return o, Usage("%s requires a value", a)
			}
			i++
			switch a {
			case "--category":
				o.category = args[i]
			case "--revision":
				o.revision = args[i]
			case "--variant":
				o.variant = args[i]
			case "--architecture":
				o.architecture = args[i]
			case "--request-id":
				o.requestID = args[i]
			}
		default:
			if strings.HasPrefix(a, "-") || o.subject != "" {
				return o, Usage("unexpected model option: %s", a)
			}
			o.subject = a
		}
	}
	switch o.category {
	case "asr":
		o.category = string(models.CategoryRecognition)
	case "tts":
		o.category = string(models.CategoryGeneration)
	case "chat":
		o.category = string(models.CategoryLanguage)
	}
	if o.category != "" && !models.ModelCategory(o.category).Valid() {
		return o, Usage("category must be language, embeddings, asr or tts")
	}
	return o, nil
}
func printModelJSON(out io.Writer, v any) error { return json.NewEncoder(out).Encode(v) }

func modelPackageCommand(ctx context.Context, args []string, env Environment) error {
	o, err := parseModelPackageOptions(args)
	if err != nil {
		return err
	}
	if o.command == "help" || o.command == "--help" || o.command == "-h" {
		usageText := "offgrid model list|catalog [--category language|embeddings|asr|tts]; search <query> --category asr|tts; discover <owner/repo>; preview|install <catalog-id|owner/repo> [--revision COMMIT --variant FILE --architecture RECIPE] [--yes --detach --request-id ID]; status [operation-id]; cancel|resume|discard <operation-id>; repair <source-operation-id> --yes; verify|remove <package-id> --revision REVISION [--yes]"
		if env.Options.JSON {
			return printModelJSON(env.Stdout, map[string]string{"usage": usageText})
		}
		fmt.Fprintln(env.Stdout, usageText)
		fmt.Fprintln(env.Stdout, "All operations use OFFGRID_SERVER_URL / OFFGRID_API_KEY. Importing a model package does not install or qualify its runtime.")
		return nil
	}
	client, err := serviceClient(env)
	if err != nil {
		return err
	}
	endpoint := ""
	method := http.MethodGet
	var input any
	switch o.command {
	case "list", "catalog":
		if o.subject != "" {
			return Usage("%s does not take a model ID", o.command)
		}
		endpoint = "/api/v2/models"
		if o.command == "catalog" {
			endpoint += "/catalog"
		}
		if o.category != "" {
			endpoint += "?category=" + url.QueryEscape(o.category)
		}
	case "search":
		if o.subject == "" {
			return Usage("a model search query is required")
		}
		endpoint = "/api/v2/models/catalog?source=huggingface&q=" + url.QueryEscape(o.subject) + "&category=" + url.QueryEscape(o.category)
	case "discover":
		if !models.ValidHubRepository(o.subject) {
			return Usage("provide a public owner/repository")
		}
		endpoint = "/api/v2/models/discover?repository=" + url.QueryEscape(o.subject)
	case "status":
		endpoint = "/api/v2/models/operations"
		if o.subject != "" {
			endpoint += "/" + url.PathEscape(o.subject)
		}
	case "cancel", "resume", "discard":
		if o.subject == "" {
			return Usage("an operation ID is required")
		}
		if o.command == "discard" && !o.yes {
			return Usage("discard removes retained model download data; pass --yes to confirm")
		}
		endpoint = "/api/v2/models/operations/" + url.PathEscape(o.subject) + "/" + o.command
		method = http.MethodPost
	case "verify", "remove":
		if o.subject == "" || o.revision == "" {
			return Usage("package ID and --revision are required")
		}
		if o.command == "remove" && !o.yes {
			return Usage("remove deletes this installed package revision; pass --yes to confirm")
		}
		endpoint = "/api/v2/models/packages/" + url.PathEscape(o.subject) + "/" + url.PathEscape(o.revision) + "/" + o.command
		method = http.MethodPost
	case "preview", "install":
		if o.subject == "" {
			return Usage("a catalog ID or repository is required")
		}
		if o.command == "install" && !o.yes {
			return Usage("preview the package first, then use install --yes to approve its download and license notices")
		}
		payload := map[string]string{"catalog_id": o.subject}
		if strings.Contains(o.subject, "/") {
			if o.revision == "" || o.variant == "" || o.architecture == "" {
				return Usage("repository install requires --revision, --variant and --architecture from discover")
			}
			payload = map[string]string{"repository": o.subject, "revision": o.revision, "variant": o.variant, "architecture": o.architecture}
		}
		var preview struct {
			ID         string                   `json:"id"`
			Resolution models.PackageResolution `json:"resolution"`
			Preflight  models.PackagePreflight  `json:"preflight"`
		}
		if err = client.JSON(ctx, "POST", "/api/v2/models/resolve", payload, &preview); err != nil {
			return err
		}
		if o.command == "preview" {
			return printModelJSON(env.Stdout, preview)
		}
		if preview.ID == "" {
			return fmt.Errorf("service returned no model preview identity")
		}
		fmt.Fprintf(env.Stderr, "Downloading %s (%s); required free space %s. Package installation does not install or qualify a runtime.\n", TerminalSafe(preview.Resolution.Manifest.Name), FormatBytes(preview.Preflight.TransferBytes), FormatBytes(preview.Preflight.RequiredFreeBytes))
		for _, notice := range preview.Resolution.SourceNotices {
			fmt.Fprintln(env.Stderr, TerminalSafe(notice))
		}
		endpoint = "/api/v2/models/operations"
		method = "POST"
		if o.requestID == "" {
			if env.ModelPackage.NewRequestID == nil {
				return fmt.Errorf("model request ID adapter unavailable")
			}
			o.requestID = env.ModelPackage.NewRequestID()
		}
		input = map[string]string{"request_id": o.requestID, "action": "install", "resolution_id": preview.ID}
	case "repair":
		if o.subject == "" || !o.yes {
			return Usage("repair requires a completed source operation ID and --yes")
		}
		if o.requestID == "" {
			if env.ModelPackage.NewRequestID == nil {
				return fmt.Errorf("model request ID adapter unavailable")
			}
			o.requestID = env.ModelPackage.NewRequestID()
		}
		endpoint = "/api/v2/models/operations"
		method = "POST"
		input = map[string]string{"request_id": o.requestID, "action": "repair", "source_operation_id": o.subject}
	default:
		return Usage("unknown model command: %s", o.command)
	}
	var result map[string]any
	if err = client.JSON(ctx, method, endpoint, input, &result); err != nil {
		return err
	}
	if (o.command == "install" || o.command == "repair" || o.command == "resume") && !o.detach {
		id, _ := result["id"].(string)
		if id == "" {
			return fmt.Errorf("service returned no operation ID")
		}
		return waitForModelOperation(ctx, client, id, env)
	}
	return printModelJSON(env.Stdout, result)
}

func waitForModelOperation(ctx context.Context, client *serviceclient.Client, id string, env Environment) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	previous := ""
	for {
		var result map[string]any
		err := client.JSON(ctx, "GET", "/api/v2/models/operations/"+url.PathEscape(id), nil, &result)
		if err != nil && ctx.Err() == nil {
			return err
		}
		if err == nil {
			state, _ := result["state"].(string)
			switch state {
			case "complete":
				return printModelJSON(env.Stdout, result)
			case "cancelled":
				return context.Canceled
			case "failed", "interrupted":
				return fmt.Errorf("model operation %s: %s; inspect status before Resume", TerminalSafe(id), TerminalSafe(fmt.Sprint(result["message"])))
			}
			line := fmt.Sprintf("%s · %s", TerminalSafe(id), TerminalSafe(state))
			if line != previous {
				fmt.Fprintln(env.Stderr, line)
				previous = line
			}
		}
		select {
		case <-ctx.Done():
			stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := client.JSON(stop, "POST", "/api/v2/models/operations/"+url.PathEscape(id)+"/cancel", nil, nil)
			cancel()
			if err != nil {
				fmt.Fprintln(env.Stderr, "Cancellation could not be confirmed; inspect the operation on the service.")
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func RunModelPackageCommand(ctx context.Context, args []string, env Environment) error {
	return modelPackageCommand(ctx, args, env)
}

func WaitForModelOperation(ctx context.Context, client *serviceclient.Client, id string, env Environment) error {
	return waitForModelOperation(ctx, client, id, env)
}
