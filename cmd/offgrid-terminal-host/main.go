// offgrid-terminal-host owns the terminal worker's OS process tree. It is a
// local desktop child, not a service endpoint or an agent execution tool.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "terminal supervisor:", err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 3 || !filepath.IsAbs(os.Args[1]) || !filepath.IsAbs(os.Args[2]) {
		return errors.New("expected absolute bundled runner and worker paths")
	}
	command := exec.Command(os.Args[1], os.Args[2])
	command.Env = append(os.Environ(), "ELECTRON_RUN_AS_NODE=1")
	input, err := command.StdinPipe()
	if err != nil {
		return err
	}
	output, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	command.Stderr = os.Stderr
	tree, err := startTree(command)
	if err != nil {
		return err
	}
	defer tree.release()
	var write sync.Mutex
	emit := func(v any) { write.Lock(); defer write.Unlock(); _ = json.NewEncoder(os.Stdout).Encode(v) }
	copied := make(chan struct{})
	go func() {
		defer close(copied)
		scan := bufio.NewScanner(output)
		scan.Buffer(make([]byte, 4096), 1024*1024)
		for scan.Scan() {
			write.Lock()
			_, _ = fmt.Fprintln(os.Stdout, scan.Text())
			write.Unlock()
		}
	}()
	requests := make(chan error, 1)
	go func() {
		scan := bufio.NewScanner(os.Stdin)
		scan.Buffer(make([]byte, 4096), 1024*1024)
		for scan.Scan() {
			var request struct {
				Action string `json:"action"`
			}
			if json.Unmarshal(scan.Bytes(), &request) != nil {
				requests <- errors.New("invalid terminal frame")
				return
			}
			if request.Action == "stop" {
				requests <- nil
				return
			}
			if _, e := input.Write(append(append([]byte{}, scan.Bytes()...), '\n')); e != nil {
				requests <- e
				return
			}
		}
		requests <- scan.Err() // Desktop pipe closure also stops all descendants.
	}()
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	var reason error
	select {
	case reason = <-requests:
	case <-exited:
	}
	if err = tree.stop(); err != nil {
		return err
	}
	_ = input.Close()
	<-copied
	emit(map[string]any{"type": "stopped", "confirmed": true})
	if reason != nil && !errors.Is(reason, io.EOF) {
		return reason
	}
	return nil
}
