package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/internal/output"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

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
				return o, usage("%s requires a value", a)
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
				return o, usage("unexpected model option: %s", a)
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
		return o, usage("category must be language, embeddings, asr or tts")
	}
	return o, nil
}
func printModelJSON(v any) error { return json.NewEncoder(os.Stdout).Encode(v) }

func runModelPackageCommand(ctx context.Context, args []string) error {
	o, err := parseModelPackageOptions(args)
	if err != nil {
		return err
	}
	if o.command == "help" || o.command == "--help" || o.command == "-h" {
		usageText := "offgrid model list|catalog [--category language|embeddings|asr|tts]; search <query> --category asr|tts; discover <owner/repo>; preview|install <catalog-id|owner/repo> [--revision COMMIT --variant FILE --architecture RECIPE] [--yes --detach --request-id ID]; status [operation-id]; cancel|resume|discard <operation-id>; repair <source-operation-id> --yes; verify|remove <package-id> --revision REVISION [--yes]"
		if output.JSONMode {
			return printModelJSON(map[string]string{"usage": usageText})
		}
		fmt.Println(usageText)
		fmt.Println("All operations use OFFGRID_SERVER_URL / OFFGRID_API_KEY. Speech model installation does not install a runtime.")
		return nil
	}
	client, err := newCommandClient()
	if err != nil {
		return err
	}
	endpoint := ""
	method := http.MethodGet
	var input any
	switch o.command {
	case "list", "catalog":
		if o.subject != "" {
			return usage("%s does not take a model ID", o.command)
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
			return usage("a model search query is required")
		}
		endpoint = "/api/v2/models/catalog?source=huggingface&q=" + url.QueryEscape(o.subject) + "&category=" + url.QueryEscape(o.category)
	case "discover":
		if !models.ValidHubRepository(o.subject) {
			return usage("provide a public owner/repository")
		}
		endpoint = "/api/v2/models/discover?repository=" + url.QueryEscape(o.subject)
	case "status":
		endpoint = "/api/v2/models/operations"
		if o.subject != "" {
			endpoint += "/" + url.PathEscape(o.subject)
		}
	case "cancel", "resume", "discard":
		if o.subject == "" {
			return usage("an operation ID is required")
		}
		if o.command == "discard" && !o.yes {
			return usage("discard removes retained model download data; pass --yes to confirm")
		}
		endpoint = "/api/v2/models/operations/" + url.PathEscape(o.subject) + "/" + o.command
		method = http.MethodPost
	case "verify", "remove":
		if o.subject == "" || o.revision == "" {
			return usage("package ID and --revision are required")
		}
		if o.command == "remove" && !o.yes {
			return usage("remove deletes this installed package revision; pass --yes to confirm")
		}
		endpoint = "/api/v2/models/packages/" + url.PathEscape(o.subject) + "/" + url.PathEscape(o.revision) + "/" + o.command
		method = http.MethodPost
	case "preview", "install":
		if o.subject == "" {
			return usage("a catalog ID or repository is required")
		}
		if o.command == "install" && !o.yes {
			return usage("preview the package first, then use install --yes to approve its download and license notices")
		}
		payload := map[string]string{"catalog_id": o.subject}
		if strings.Contains(o.subject, "/") {
			if o.revision == "" || o.variant == "" || o.architecture == "" {
				return usage("repository install requires --revision, --variant and --architecture from discover")
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
			return printModelJSON(preview)
		}
		if preview.ID == "" {
			return fmt.Errorf("service returned no model preview identity")
		}
		fmt.Fprintf(os.Stderr, "Downloading %s (%s); required free space %s. Speech runtime is not available in this build.\n", terminalSafe(preview.Resolution.Manifest.Name), formatBytes(preview.Preflight.TransferBytes), formatBytes(preview.Preflight.RequiredFreeBytes))
		for _, notice := range preview.Resolution.SourceNotices {
			fmt.Fprintln(os.Stderr, terminalSafe(notice))
		}
		endpoint = "/api/v2/models/operations"
		method = "POST"
		if o.requestID == "" {
			o.requestID = uuid.NewString()
		}
		input = map[string]string{"request_id": o.requestID, "action": "install", "resolution_id": preview.ID}
	case "repair":
		if o.subject == "" || !o.yes {
			return usage("repair requires a completed source operation ID and --yes")
		}
		if o.requestID == "" {
			o.requestID = uuid.NewString()
		}
		endpoint = "/api/v2/models/operations"
		method = "POST"
		input = map[string]string{"request_id": o.requestID, "action": "repair", "source_operation_id": o.subject}
	default:
		return usage("unknown model command: %s", o.command)
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
		return waitForModelOperation(ctx, client, id)
	}
	return printModelJSON(result)
}

func waitForModelOperation(ctx context.Context, client *serviceclient.Client, id string) error {
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
				return printModelJSON(result)
			case "cancelled":
				return context.Canceled
			case "failed", "interrupted":
				return fmt.Errorf("model operation %s: %s; inspect status before Resume", terminalSafe(id), terminalSafe(fmt.Sprint(result["message"])))
			}
			line := fmt.Sprintf("%s · %s", terminalSafe(id), terminalSafe(state))
			if line != previous {
				fmt.Fprintln(os.Stderr, line)
				previous = line
			}
		}
		select {
		case <-ctx.Done():
			stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := client.JSON(stop, "POST", "/api/v2/models/operations/"+url.PathEscape(id)+"/cancel", nil, nil)
			cancel()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Cancellation could not be confirmed; inspect the operation on the service.")
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
