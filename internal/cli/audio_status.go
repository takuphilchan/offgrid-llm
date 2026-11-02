package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/takuphilchan/offgrid-llm/internal/audio"
)

// Status uses the same selected service and credentials as Chat. It must not
// inspect a different localhost installation or suggest downloading unrelated
// Whisper weights when an installed Qwen package lacks its runtime.
func audioStatusCommand(ctx context.Context, env Environment) error {
	if env.Service.Client == nil {
		return fmt.Errorf("authenticated audio service client unavailable")
	}
	client, err := env.Service.Client()
	if err != nil {
		return err
	}
	var status struct {
		ASR      audioReadiness        `json:"asr"`
		TTS      audioReadiness        `json:"tts"`
		Profiles []audio.SpeechProfile `json:"profiles,omitempty"`
	}
	if err := client.JSON(ctx, http.MethodGet, "/v1/audio/status", nil, &status); err != nil {
		return err
	}
	if env.Options.JSON {
		return json.NewEncoder(env.Stdout).Encode(status)
	}
	fmt.Fprintln(env.Stdout, "\nAudio System Status")
	fmt.Fprintln(env.Stdout, "  Service: "+client.Origin())
	for _, item := range []struct {
		name  string
		state audioReadiness
	}{{"Dictation", status.ASR}, {"Read aloud", status.TTS}} {
		label := "Needs setup"
		if item.state.Available {
			label = "Runtime available"
		}
		fmt.Fprintf(env.Stdout, "  %s: %s\n", item.name, label)
		if !item.state.Available && item.state.Issue != "" {
			fmt.Fprintln(env.Stdout, "    "+TerminalSafe(item.state.Issue))
		}
	}
	for _, profile := range status.Profiles {
		label := "runtime unavailable"
		if profile.Available {
			label = "runtime available; inference untested"
		}
		if profile.Available && profile.SmokeTested {
			label = "speech request passed this service session"
		}
		fmt.Fprintf(env.Stdout, "  %s · %s · %s\n", TerminalSafe(profile.Name), TerminalSafe(profile.Adapter), label)
	}
	if !status.ASR.Available || !status.TTS.Available {
		fmt.Fprintln(env.Stdout, "\n  Model files and speech runtimes are separate. Re-downloading installed weights will not repair a missing runtime.")
		fmt.Fprintln(env.Stdout, "  Open Voice settings to select an available model. Runtime setup belongs on the service host.")
	}
	return nil
}

type audioReadiness struct {
	Available bool   `json:"available"`
	Runtime   bool   `json:"runtime"`
	Issue     string `json:"issue,omitempty"`
}
