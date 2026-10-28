package execution

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

func ParseProcessSpec(arguments json.RawMessage) (ProcessSpec, error) {
	var spec ProcessSpec
	if err := decodeStrict(arguments, &spec); err != nil {
		return spec, err
	}
	return spec, ValidateProcessSpec(spec)
}

// RoutineProcess is a deliberately small, versioned invocation grammar, not
// an English risk classifier or a sandbox. Tests/builds execute project code
// with the approving account's authority. Unknown invocations require exact
// per-call approval; neither a tool argument nor a profile can supply it.
func RoutineProcess(spec ProcessSpec) bool {
	if ValidateProcessSpec(spec) != nil {
		return false
	}
	a := spec.Arguments
	if len(a) == 0 {
		return false
	}
	switch spec.Program {
	case "go", "go.exe":
		if a[0] != "test" {
			return false
		}
		for _, arg := range a[1:] {
			if arg == "-race" || arg == "-v" || arg == "-short" || arg == "-failfast" {
				continue
			}
			if strings.HasPrefix(arg, "-count=") || strings.HasPrefix(arg, "-run=") || strings.HasPrefix(arg, "-timeout=") {
				continue
			}
			if arg == "." || arg == "./..." || strings.HasPrefix(arg, "./") && !strings.Contains(arg, "..") && !strings.ContainsAny(arg, `\:=`) {
				continue
			}
			return false
		}
		return true
	case "npm", "npm.cmd":
		return len(a) == 1 && a[0] == "test" || len(a) == 2 && a[0] == "run" && (a[1] == "test" || a[1] == "build" || a[1] == "check" || a[1] == "lint")
	case "git", "git.exe":
		if a[0] == "status" {
			for _, arg := range a[1:] {
				if arg != "--short" && arg != "--porcelain" && arg != "--branch" {
					return false
				}
			}
			return true
		}
		return len(a) == 1 && (a[0] == "diff" || a[0] == "log")
	}
	return false
}

// RepeatsRoutineProgram detects an ambiguous proposal, not a normalized command
// or new grant. The caller may ask the model to correct it before dispatch.
// General/custom commands (for example echo echo) are not classified this way.
func RepeatsRoutineProgram(spec ProcessSpec) bool {
	if len(spec.Arguments) < 2 || spec.Arguments[0] != spec.Program || ValidateProcessSpec(spec) != nil {
		return false
	}
	candidate := spec
	candidate.Arguments = spec.Arguments[1:]
	return RoutineProcess(candidate)
}

func ValidateProcessSpec(spec ProcessSpec) error {
	if spec.Program == "" || !utf8.ValidString(spec.Program) || strings.ContainsRune(spec.Program, 0) || len(spec.Program) > 4096 || len(spec.Arguments) > 128 || spec.TimeoutSeconds < 0 || spec.TimeoutSeconds > 3600 {
		return ErrLimit
	}
	size := len(spec.Program)
	for _, arg := range spec.Arguments {
		size += len(arg)
		if !utf8.ValidString(arg) || strings.ContainsRune(arg, 0) {
			return ErrProtocol
		}
	}
	if size > 65536 {
		return ErrLimit
	}
	return nil
}

// TrustedProcessPolicy is installed only by the native supervisor after the
// local person approved account-wide process authority for this session.
func TrustedProcessPolicy(_ context.Context, _ Binding, spec ProcessSpec, approved bool) error {
	if err := ValidateProcessSpec(spec); err != nil {
		return err
	}
	if !approved && !RoutineProcess(spec) {
		return ErrAuthority
	}
	return nil
}
