package main

import (
	"context"
	"os"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

// The executable supplies OS capabilities; parsing, requests, and persistence
// remain in the internal CLI command.
func osInteractiveDependencies() cli.InteractiveDependencies {
	return cli.InteractiveDependencies{
		DownloadHF:     runModelDownload,
		EnsureServer:   ensureOffgridServerRunning,
		StartServer:    startOffgridServerInBackground,
		EnsureLlama:    ensureLlamaServerRunning,
		StartLlama:     startLlamaServerInBackground,
		ReadLlamaPort:  config.ReadLlamaPort,
		WaitModelReady: waitForModelReady,
		Wait: func(ctx context.Context, seconds int) error {
			timer := time.NewTimer(time.Duration(seconds) * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		ReadDir:      os.ReadDir,
		ShouldUseTUI: shouldUseChatTUI,
		RunTUI: func(ctx context.Context, state *cli.InteractiveTUIState) error {
			runtime := &ChatRuntime{
				ModelName:        state.ModelName,
				ResolvedModel:    state.ResolvedModel,
				ServerPort:       state.ServerPort,
				Client:           state.Client,
				UseKnowledgeBase: state.UseKnowledgeBase,
				Images:           state.Images,
				Messages:         state.Messages,
				SessionMgr:       state.SessionMgr,
				CurrentSession:   state.CurrentSession,
			}
			defer func() {
				state.Messages = runtime.Messages
				state.Images = runtime.Images
				state.UseKnowledgeBase = runtime.UseKnowledgeBase
			}()
			return runChatTUI(ctx, runtime)
		},
		IsEmbeddingModel:   isLikelyEmbeddingModel,
		EmbeddingChatError: embeddingModelChatError,
		SyncOutput:         os.Stdout.Sync,
		DetectResources:    resource.DetectResources,
	}
}

type ChatMessage = cli.ChatMessage
type ChatCompletionRequest = cli.ChatCompletionRequest
type ChatCompletionChunk = cli.ChatCompletionChunk
