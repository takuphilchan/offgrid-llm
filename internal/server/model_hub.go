//go:build !modeltestfixtures

package server

import "github.com/takuphilchan/offgrid-llm/internal/models"

func newModelHub() *models.HuggingFaceClient { return models.NewHuggingFaceClient() }
