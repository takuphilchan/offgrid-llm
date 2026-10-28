package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

const agentContextHelp = `Owned context resources on the connected service:
  offgrid agent skills list
  offgrid agent skills import DIRECTORY
  offgrid agent skills revise ID REVISION DIRECTORY
  offgrid agent skills show ID REVISION DIGEST
  offgrid agent skills review ID REVISION DIGEST
  offgrid agent skills attach PROFILE PROFILE_REVISION ID REVISION DIGEST
  offgrid agent memory show PROFILE
  offgrid agent memory consent PROFILE CONSENT_REVISION on|off
  offgrid agent memory put PROFILE FILE
  offgrid agent memory delete PROFILE ID ENTRY_REVISION

Import reads only the selected local directory, with bounded files and no links.
Scripts are data, never executed. Review creates an unattached reviewed copy.
Attach saves a new profile revision; existing jobs stay unchanged.
Memory defaults off. Re-enable never restores an older task's consent.
FILE is AgentMemoryWrite JSON (operation, optional id, expected_revision, label,
content); - reads stdin. Deletion excludes future retrieval, not saved history.
Results are JSON to preserve exact bytes and escape terminal controls.
`

// ReadSkillDirectory is injected by the process adapter. An os.Root prevents
// path races from widening explicit input reads beyond the chosen folder.
func ReadSkillDirectory(directory string) ([]agentcontext.SkillFile, error) {
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, agentcontext.ErrSkill
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	files := []agentcontext.SkillFile{}
	total := 0
	err = fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return agentcontext.ErrSkill
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > 64<<10 || len(files) >= 64 {
			return agentcontext.ErrSkill
		}
		file, err := root.Open(path)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, 65537))
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		total += len(data)
		if len(data) > 64<<10 || total > 512<<10 {
			return agentcontext.ErrSkill
		}
		files = append(files, agentcontext.SkillFile{Path: path, Content: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	bundle, err := agentcontext.Parse(files)
	if err != nil {
		return nil, err
	}
	return bundle.Files, nil
}

func contextRevision(raw string, zero bool) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n == 0 && !zero {
		return 0, Usage("Provide the exact nonnegative reviewed revision (positive for an existing entry)")
	}
	return n, nil
}

func agentContextCommand(ctx context.Context, kind string, args []string, env Environment) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		_, err := fmt.Fprint(env.Stdout, agentContextHelp)
		return err
	}
	client, err := serviceClient(env)
	if err != nil {
		return err
	}
	if err = client.RequireCapability(ctx, "agent-context-resources-v1"); err != nil {
		return err
	}
	method, path := http.MethodGet, "/api/v2/agent-skills"
	var body any
	if kind == "skills" {
		switch args[0] {
		case "list":
			if len(args) != 1 {
				return Usage("agent skills list")
			}
		case "import", "revise":
			directory := ""
			revision := 0
			if args[0] == "import" {
				if len(args) != 2 {
					return Usage("agent skills import DIRECTORY")
				}
				directory = args[1]
				method = http.MethodPost
			} else {
				if len(args) != 4 || !ValidProfileID(args[1]) {
					return Usage("agent skills revise ID REVISION DIRECTORY")
				}
				revision, err = contextRevision(args[2], false)
				if err != nil {
					return err
				}
				path += "/" + url.PathEscape(args[1])
				directory = args[3]
				method = http.MethodPut
			}
			if env.Service.ReadSkillDirectory == nil {
				return fmt.Errorf("local skill reader unavailable")
			}
			files, err := env.Service.ReadSkillDirectory(directory)
			if err != nil {
				return err
			}
			body = map[string]any{"expected_revision": revision, "files": files}
		case "show", "review":
			if len(args) != 4 || !ValidProfileID(args[1]) {
				return Usage("agent skills %s ID REVISION DIGEST", args[0])
			}
			revision, err := contextRevision(args[2], false)
			if err != nil {
				return err
			}
			ref := agentcontext.SkillReference{ID: args[1], Revision: revision, Digest: args[3]}
			if !ref.Valid() {
				return agentcontext.ErrSkill
			}
			path += "/" + url.PathEscape(ref.ID)
			if args[0] == "show" {
				path += "?revision=" + strconv.Itoa(revision) + "&digest=" + url.QueryEscape(ref.Digest)
			} else {
				path += "/review"
				method = http.MethodPost
				body = map[string]any{"expected_revision": revision, "digest": ref.Digest}
			}
		case "attach":
			return attachSkill(ctx, client, args, env.Stdout)
		default:
			return Usage("Unknown agent skills command %q", args[0])
		}
	} else {
		if len(args) < 2 || !ValidProfileID(args[1]) {
			return Usage("agent memory show|consent|put|delete PROFILE ...")
		}
		path = "/api/v2/agent-memory/" + url.PathEscape(args[1])
		switch args[0] {
		case "show":
			if len(args) != 2 {
				return Usage("agent memory show PROFILE")
			}
		case "consent":
			if len(args) != 4 || args[3] != "on" && args[3] != "off" {
				return Usage("agent memory consent PROFILE CONSENT_REVISION on|off")
			}
			revision, err := contextRevision(args[2], true)
			if err != nil {
				return err
			}
			method = http.MethodPut
			path += "/consent"
			body = map[string]any{"expected_revision": revision, "enabled": args[3] == "on"}
		case "put":
			if len(args) != 3 {
				return Usage("agent memory put PROFILE FILE")
			}
			input := env.Stdin
			if args[2] != "-" {
				if env.Service.OpenProfileFile == nil {
					return fmt.Errorf("local input reader unavailable")
				}
				file, err := env.Service.OpenProfileFile(args[2])
				if err != nil {
					return err
				}
				defer file.Close()
				input = file
			}
			data, err := io.ReadAll(io.LimitReader(input, 32769))
			if err != nil {
				return err
			}
			if len(data) > 32768 {
				return Usage("Memory write JSON exceeds 32 KiB")
			}
			var write struct {
				Operation        string `json:"operation"`
				ID               string `json:"id,omitempty"`
				ExpectedRevision int    `json:"expected_revision"`
				Label            string `json:"label"`
				Content          string `json:"content"`
			}
			decoder := json.NewDecoder(strings.NewReader(string(data)))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&write) != nil || decoder.Decode(new(any)) != io.EOF || strings.TrimSpace(write.Operation) == "" {
				return Usage("Provide one AgentMemoryWrite JSON object without provenance or authority fields")
			}
			body = write
			method = http.MethodPost
		case "delete":
			if len(args) != 4 || !ValidProfileID(args[2]) {
				return Usage("agent memory delete PROFILE ID ENTRY_REVISION")
			}
			revision, err := contextRevision(args[3], false)
			if err != nil {
				return err
			}
			method = http.MethodDelete
			path += "/" + url.PathEscape(args[2]) + "?revision=" + strconv.Itoa(revision)
		default:
			return Usage("Unknown agent memory command %q", args[0])
		}
	}
	var result json.RawMessage
	if err = client.JSON(ctx, method, path, body, &result); err != nil {
		return err
	}
	return json.NewEncoder(env.Stdout).Encode(result)
}

func attachSkill(ctx context.Context, client *serviceclient.Client, args []string, out io.Writer) error {
	if len(args) != 6 || !ValidProfileID(args[1]) {
		return Usage("agent skills attach PROFILE PROFILE_REVISION ID REVISION DIGEST")
	}
	profileRevision, err := contextRevision(args[2], false)
	if err != nil {
		return err
	}
	revision, err := contextRevision(args[4], false)
	if err != nil {
		return err
	}
	ref := agentcontext.SkillReference{ID: args[3], Revision: revision, Digest: args[5]}
	if !ref.Valid() {
		return agentcontext.ErrSkill
	}
	path := "/api/v2/agent-profiles/" + url.PathEscape(args[1])
	var profile agents.AgentProfile
	if err = client.JSON(ctx, http.MethodGet, path+"?revision="+strconv.Itoa(profileRevision), nil, &profile); err != nil {
		return err
	}
	if profile.ID != args[1] || profile.Revision != profileRevision || profile.Archived {
		return fmt.Errorf("profile changed; inspect the current revision")
	}
	found := false
	for i, item := range profile.Settings.Skills {
		if item.ID == ref.ID {
			profile.Settings.Skills[i] = ref
			found = true
		}
	}
	if !found {
		profile.Settings.Skills = append(profile.Settings.Skills, ref)
	}
	var result json.RawMessage
	if err = client.JSON(ctx, http.MethodPut, path, map[string]any{"expected_revision": profileRevision, "settings": profile.Settings}, &result); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(result)
}
