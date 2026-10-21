// model-package-metadata resolves public repository metadata and small data
// dependencies only. It never fetches weights or executes repository code.
// Output is a reviewable catalog candidate, not qualification evidence.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/takuphilchan/offgrid-llm/internal/models"
)

func main() {
	repo := flag.String("repository", "", "public owner/repository")
	revision := flag.String("revision", "", "immutable full commit")
	variant := flag.String("variant", "", "discovered variant path")
	architecture := flag.String("architecture", "", "supported recipe")
	flag.Parse()
	r, err := models.NewHuggingFaceClient().ResolvePackage(context.Background(), models.PackageResolutionRequest{Repository: *repo, Revision: *revision, Variant: *variant, Architecture: *architecture})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
