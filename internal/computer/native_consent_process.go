package computer

import (
	"errors"
	"io"
	"os/exec"
	"sync"
	"time"
)

// NativeConsentProcess is a visible, owned GUI process, separate from provider
// calls. Its Stop event can revoke dispatch even when an accessibility provider
// hangs. Private pipe replies never come from the renderer, model or service.
type NativeConsentProcess struct {
	child   *exec.Cmd
	in      io.WriteCloser
	mu      sync.Mutex
	write   sync.Mutex
	pending map[string]chan bool
	done    chan struct{}
	stop    sync.Once
}

func StartNativeConsentProcess(executable string, revoke func()) (*NativeConsentProcess, error) {
	child := exec.Command(executable)
	in, err := child.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := child.StdoutPipe()
	if err != nil {
		return nil, err
	}
	child.Stderr = io.Discard
	if err := child.Start(); err != nil {
		return nil, err
	}
	ui := &NativeConsentProcess{child: child, in: in, pending: map[string]chan bool{}, done: make(chan struct{})}
	ready := make(chan bool, 1)
	go func() {
		defer func() { revoke(); in.Close(); child.Process.Kill(); child.Wait(); close(ui.done) }()
		booted := false
		for {
			var message struct {
				Kind    string `json:"kind"`
				ID      string `json:"id,omitempty"`
				Allowed bool   `json:"allowed,omitempty"`
			}
			if ReadControlFrame(out, &message) != nil {
				return
			}
			switch message.Kind {
			case "ready":
				if booted || message.ID != "" || message.Allowed {
					return
				}
				booted = true
				ready <- true
			case "stop":
				return
			case "consent":
				if !booted {
					return
				}
				ui.mu.Lock()
				answer := ui.pending[message.ID]
				delete(ui.pending, message.ID)
				ui.mu.Unlock()
				if answer == nil {
					return
				}
				answer <- message.Allowed
			default:
				return
			}
		}
	}()
	select {
	case <-ready:
		return ui, nil
	case <-ui.done:
		return nil, errors.New("computer_stop_surface_unavailable")
	case <-time.After(10 * time.Second):
		ui.Close()
		return nil, errors.New("computer_stop_surface_unavailable")
	}
}
func (ui *NativeConsentProcess) Confirm(text string) bool {
	select {
	case <-ui.done:
		return false
	default:
	}
	id := randomID()
	answer := make(chan bool, 1)
	ui.mu.Lock()
	ui.pending[id] = answer
	ui.mu.Unlock()
	defer func() { ui.mu.Lock(); delete(ui.pending, id); ui.mu.Unlock() }()
	ui.write.Lock()
	err := WriteControlFrame(ui.in, map[string]string{"kind": "confirm", "id": id, "text": text})
	ui.write.Unlock()
	if err != nil {
		return false
	}
	select {
	case allowed := <-answer:
		return allowed
	case <-ui.done:
		return false
	case <-time.After(5 * time.Minute):
		ui.Close()
		return false
	}
}
func (ui *NativeConsentProcess) Close() {
	ui.stop.Do(func() {
		ui.in.Close()
		select {
		case <-ui.done:
			return
		case <-time.After(time.Second):
			ui.child.Process.Kill()
		}
		select {
		case <-ui.done:
		case <-time.After(time.Second):
		}
	})
}
