package execution

import (
	"context"
	"encoding/json"
	"os/exec"

	"github.com/takuphilchan/offgrid-llm/internal/processowner"
)

type supervisedRuntime struct {
	Version int
	Runtime processowner.Runtime
	Storage *supervisedStorageAncestry `json:",omitempty"`
}

func supervisedProcessEnvironment(journal ...string) ([]string, error) {
	runtime, err := linuxNamespaceRuntime()
	if err != nil {
		return nil, err
	}
	pin := supervisedRuntime{Version: 2, Runtime: runtime}
	if len(journal) != 0 {
		if len(journal) != 1 {
			return nil, ErrAuthority
		}
		pin.Storage, err = captureSupervisedStorageAncestry(journal[0])
		if err != nil {
			return nil, err
		}
	}
	data, err := json.Marshal(pin)
	if err != nil {
		return nil, err
	}
	if len(data) > 65536 {
		return nil, ErrLimit
	}
	return []string{"OFFGRID_EXECUTOR_PROCESS_RUNTIME=" + string(data)}, nil
}

func supervisedProcessStarter(privateRuntime string) (ownedProcessStarter, error) {
	if privateRuntime == "" {
		return startOwned, nil
	}
	var pin supervisedRuntime
	if len(privateRuntime) > 65536 || decodeStrict([]byte(privateRuntime), &pin) != nil || pin.Version != 2 {
		return nil, ErrAuthority
	}
	if err := pin.Runtime.Verify(context.Background()); err != nil {
		return nil, err
	}
	return func(command *exec.Cmd) (func() error, func() error, func() error, error) {
		return startOwnedRuntime(command, pin.Runtime)
	}, nil
}
