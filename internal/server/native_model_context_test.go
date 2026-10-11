package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestDesktopModelContextBoundsLongURLsAndResolvesExactReferences(t *testing.T) {
	view := desktopContextFixture(computer.TargetIdentity{ID: "surface", OSSession: "os", ProcessGeneration: "generation", Surface: "window", Driver: "windows-uia"}, "Any application")
	view.Elements = nil
	for i := 0; i < 400; i++ {
		text := "https://example.test/"
		for j := 0; j < 40; j++ {
			text += fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%d", i, j))))
		}
		view.Elements = append(view.Elements, computer.NativeElement{ID: fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprint(i)))), Name: strings.Repeat("界", 200), Role: "link", Text: &text, Invokable: true})
	}
	view.Elements[399].Writable = true
	raw, _ := json.Marshal(map[string]any{"observation": view.Observation, "elements": view.Elements, "limited": false, "offgrid_step_id": 2})
	task := &agents.Task{Config: agents.AgentConfig{TaskFirst: true, ComputerScope: "desktop", ComputerDriver: "windows-uia"}, Steps: []agents.Step{
		{ID: 1, Type: "action", ToolName: "computer_applications", ToolResult: `{"applications":[{"id":"application-opaque","name":"Any application"}]}`},
		{ID: 2, Type: "action", ToolName: "computer_observe", ToolResult: string(raw)},
	}}
	tools := &browserRunTools{}
	messages := []api.ChatMessage{{Role: "tool", Name: "computer_observe", Content: string(raw)}}
	got := tools.ProjectModelContext(task, messages)[0].StringContent()
	if len(got) > 5500 || !utf8.ValidString(got) || !strings.Contains(got, "e2_399") || !strings.Contains(got, "step:2") || !strings.Contains(got, "name_limited") || !strings.Contains(got, "text_limited") {
		t.Fatalf("unbounded or incomplete projection: %d bytes", len(got))
	}
	if task.Steps[1].ToolResult != string(raw) || messages[0].StringContent() != string(raw) {
		t.Fatal("canonical evidence changed")
	}
	args := tools.ResolveModelArguments(task, "computer_replace_text", `{"observation_id":"o2","element":"e2_399","text":"e2_399"}`)
	var decoded map[string]string
	json.Unmarshal([]byte(args), &decoded)
	if decoded["observation_id"] != view.Observation.ID || decoded["element"] != view.Elements[399].ID || decoded["text"] != "e2_399" {
		t.Fatal("references incorrectly resolved", args)
	}
	if other := tools.ResolveModelArguments(&agents.Task{Config: task.Config}, "computer_replace_text", `{"element":"e2_399"}`); other != `{"element":"e2_399"}` {
		t.Fatal("foreign task reference resolved")
	}
	if got := tools.ResolveModelArguments(task, "mcp__external__edit", `{"element":"e2_399"}`); got != `{"element":"e2_399"}` {
		t.Fatal("MCP arguments rewritten")
	}
	if got := tools.ResolveModelArguments(task, "computer_open_application", `{"application_id":"a1_0"}`); !strings.Contains(got, "application-opaque") {
		t.Fatal("application alias not restored")
	}
}

func TestNativeModelContextIsLosslessAndNeverChangesEvidence(t *testing.T) {
	for _, driver := range []string{"windows-uia", "macos-accessibility", "linux-atspi"} {
		for _, name := range []string{"Microsoft Edge", "LibreOffice Writer", "旅行 🧭 محرر"} {
			t.Run(driver+"/"+name, func(t *testing.T) {
				target := computer.TargetIdentity{ID: "surface", OSSession: "session", ProcessGeneration: "process", Surface: "surface", Driver: driver}
				view := desktopContextFixture(target, name)
				view.Limited = true
				for i := range view.Elements {
					e := &view.Elements[i]
					e.Writable, e.Invokable, e.Checkable, e.TextLimited = i&1 != 0, i&2 != 0, i&4 != 0, i&8 != 0
					if e.Checkable {
						checked := i&16 != 0
						e.Checked = &checked
					}
					if i%3 == 0 {
						text := "旅行 🧭 \"quoted\"\n\t<untrusted>\\path"
						e.Text = &text
					}
				}
				raw, _ := json.Marshal(view)
				messages := []api.ChatMessage{{Role: "system", Content: "instructions"}, {Role: "tool", Name: "computer_observe", ToolCallID: "call", Content: string(raw)}}
				task := &agents.Task{Config: agents.AgentConfig{TaskFirst: true, ComputerDriver: driver}, Checkpoint: &agents.Checkpoint{Messages: messages}}
				projected := (&browserRunTools{}).ProjectModelContext(task, messages)
				if messages[1].StringContent() != string(raw) || !reflect.DeepEqual(projected[0], messages[0]) || projected[1].ToolCallID != "call" {
					t.Fatal("canonical messages or identity changed")
				}
				var compact struct {
					Observation computer.ControlObservation `json:"observation"`
					Limited     bool                        `json:"limited"`
					Rows        [][]json.RawMessage         `json:"elements"`
					Columns     []string                    `json:"element_columns"`
				}
				if err := json.Unmarshal([]byte(projected[1].StringContent()), &compact); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(compact.Columns, []string{"id", "name", "role", "flags", "text", "checked", "identity"}) || compact.Observation != view.Observation || compact.Limited != view.Limited || len(compact.Rows) != len(view.Elements) {
					t.Fatal("lost observation data")
				}
				for i, row := range compact.Rows {
					var got computer.NativeElement
					json.Unmarshal(row[0], &got.ID)
					json.Unmarshal(row[1], &got.Name)
					json.Unmarshal(row[2], &got.Role)
					var flags []string
					json.Unmarshal(row[3], &flags)
					for _, flag := range flags {
						switch flag {
						case "writable":
							got.Writable = true
						case "invokable":
							got.Invokable = true
						case "checkable":
							got.Checkable = true
						case "text_limited":
							got.TextLimited = true
						default:
							t.Fatal("unknown flag")
						}
					}
					json.Unmarshal(row[4], &got.Text)
					json.Unmarshal(row[5], &got.Checked)
					json.Unmarshal(row[6], &got.Identity)
					if !reflect.DeepEqual(got, view.Elements[i]) {
						t.Fatalf("control %d changed: %+v", i, got)
					}
				}
				if len(projected[1].StringContent()) >= len(raw) {
					t.Fatal("not compact")
				}
			})
		}
	}
}

func TestNativeModelContextPreservesUnknownProtocolsAndOtherTools(t *testing.T) {
	view := desktopContextFixture(computer.TargetIdentity{ID: "target"}, "fixture")
	raw, _ := json.Marshal(view)
	for _, raw := range []string{string(raw[:len(raw)-1]) + `,"future":true}`, strings.Replace(string(raw), `"role":"region"`, `"role":"region","protected":true`, 1), `{"status":"failed"}`, "invalid JSON"} {
		messages := []api.ChatMessage{{Role: "tool", Name: "computer_observe", Content: raw}}
		projected := (&browserRunTools{}).ProjectModelContext(&agents.Task{Config: agents.AgentConfig{TaskFirst: true, ComputerDriver: "windows-uia"}}, messages)
		if projected[0].StringContent() != raw {
			t.Fatal("unknown field/error disappeared")
		}
	}
	for _, name := range []string{"mcp__remote__observe", "browser_capture"} {
		messages := []api.ChatMessage{{Role: "tool", Name: name, Content: string(raw)}}
		projected := (&browserRunTools{}).ProjectModelContext(&agents.Task{Config: agents.AgentConfig{TaskFirst: true, ComputerDriver: "windows-uia"}}, messages)
		if projected[0].StringContent() != string(raw) {
			t.Fatal("unrelated provider changed")
		}
	}
}
