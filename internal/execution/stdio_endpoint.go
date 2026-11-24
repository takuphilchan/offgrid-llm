package execution

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// StdioEndpoint owns one pinned, locally approved worker. Start is a trusted
// supervisor operation, not an HTTP/renderer/model entry point. The expected
// digest must come from the matching trusted application asset manifest.
type StdioEndpoint struct {
	identity          WorkerIdentity
	input             io.WriteCloser
	output            io.ReadCloser
	token             string
	gate              chan struct{}
	done              chan struct{}
	stop              func() error
	closeOnce         sync.Once
	closeErr          error
	cleanupResult     chan error
	protectionMu      sync.Mutex
	protection        *stdioProtectionOwner
	protectionCleanup *stdioNativeCleanup
}

func StartStdio(ctx context.Context, binary, digest, grant, state string) (*StdioEndpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(binary) || !filepath.IsAbs(grant) || !filepath.IsAbs(state) || len(digest) != 64 {
		return nil, ErrAuthority
	}
	// Capture only a private native configuration. It never authorizes new
	// operations: after worker exit the supervisor may only stop owned objects.
	approved, err := readProtectionGrant(grant)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(binary)
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrAuthority
	}
	f, err := os.Open(binary)
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	_, hashErr := io.Copy(hash, f)
	closeErr := f.Close()
	if hashErr != nil || closeErr != nil || hex.EncodeToString(hash.Sum(nil)) != digest {
		return nil, ErrAuthority
	}
	var secret [32]byte
	if _, err = rand.Read(secret[:]); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(secret[:])
	runtimeEnvironment, err := supervisedProcessEnvironment(state)
	if err != nil {
		return nil, err
	}
	command := exec.Command(binary, "--stdio", "--grant-file", grant, "--state-dir", state)
	command.Env = append(processEnvironment(), "OFFGRID_EXECUTOR_TOKEN="+token)
	command.Env = append(command.Env, runtimeEnvironment...)
	if approved.Backend == "local" && nativeCleanupVerified(nil) {
		command.Env = append(command.Env, protectionRecoveryEnvironment+"=1")
	}
	input, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := command.StdoutPipe()
	if err != nil {
		input.Close()
		return nil, err
	}
	stop, wait, release, err := startOwned(command)
	if err != nil {
		input.Close()
		output.Close()
		return nil, err
	}
	e := &StdioEndpoint{input: input, output: output, token: token, gate: make(chan struct{}, 1), done: make(chan struct{}), stop: stop, cleanupResult: make(chan error, 1)}
	go func() {
		waitErr := wait()
		releaseErr := release()
		e.recordProtectionCleanup(releaseErr)
		input.Close()
		output.Close()
		cleanupErr := errors.Join(waitErr, releaseErr)
		if runtime.GOOS == "linux" && approved.Backend == "isolated_linux" {
			cleanupErr = errors.Join(cleanupErr, cleanupStoppedWorker(approved, state))
		}
		settled, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cleanupErr = errors.Join(cleanupErr, e.finishProtection(settled))
		cancel()
		e.cleanupResult <- cleanupErr
		close(e.done)
	}()
	go func() {
		select {
		case <-ctx.Done():
			e.Close()
		case <-e.done:
		}
	}()
	var ready Response
	read := make(chan error, 1)
	go func() { read <- ReadFrame(output, &ready) }()
	startup := time.NewTimer(10 * time.Second)
	defer startup.Stop()
	select {
	case err = <-read:
	case <-ctx.Done():
		e.Close()
		return nil, ctx.Err()
	case <-startup.C:
		e.Close()
		return nil, ErrHostUnavailable
	}
	if err != nil || ready.Version != ProtocolVersion || ready.ID != "startup" || ready.Receipt == nil || ready.Receipt.State != "ready" {
		e.Close()
		return nil, ErrProtocol
	}
	if err = e.acceptProtectionStartup(ctx, ready.Receipt.Result, approved, state); err != nil {
		e.Close()
		return nil, err
	}
	return e, nil
}

func StartManaged(ctx context.Context, directory, version, grant, state string) (*StdioEndpoint, Pack, error) {
	pack, binary, err := LoadPack(directory, version)
	if err != nil {
		return nil, Pack{}, err
	}
	endpoint, err := StartStdio(ctx, binary, pack.SHA256, grant, state)
	if err != nil {
		return nil, Pack{}, err
	}
	if !endpoint.identity.matches(pack) {
		endpoint.Close()
		return nil, Pack{}, ErrProtocol
	}
	return endpoint, pack, nil
}

func (e *StdioEndpoint) Call(ctx context.Context, r Request) (Response, error) {
	select {
	case e.gate <- struct{}{}:
		defer func() { <-e.gate }()
	case <-ctx.Done():
		return Response{}, ctx.Err()
	case <-e.done:
		return Response{}, ErrHostUnavailable
	}
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	r.Token = e.token
	var result Response
	finished := make(chan error, 1)
	go func() {
		err := WriteFrame(e.input, r)
		if err == nil {
			err = ReadFrame(e.output, &result)
		}
		finished <- err
	}()
	select {
	case err := <-finished:
		if err != nil {
			e.Close()
			return Response{}, ErrHostUnavailable
		}
		if result.Version != ProtocolVersion || result.ID != r.ID {
			e.Close()
			return Response{}, ErrProtocol
		}
		return result, nil
	case <-ctx.Done():
		// A timed-out private stream cannot safely be reused: its next reply
		// may belong to the abandoned mutation. Close it, never resend.
		e.Close()
		return Response{}, ctx.Err()
	}
}

func (e *StdioEndpoint) control(ctx context.Context, b Binding, kind string) (Response, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Response{}, err
	}
	value := hex.EncodeToString(id[:])
	response, err := e.Call(ctx, Request{Version: ProtocolVersion, ID: value, OperationID: value, Binding: b, Operation: kind, Arguments: json.RawMessage(`{}`)})
	if err != nil {
		return Response{}, err
	}
	if response.Error != "" || response.Receipt == nil || response.Receipt.State != "complete" {
		return Response{}, ErrAuthority
	}
	return response, nil
}
func (e *StdioEndpoint) Check(ctx context.Context, b Binding) (Context, error) {
	response, err := e.control(ctx, b, "supervisor_info")
	if err != nil {
		return Context{}, err
	}
	var config Context
	if err = decodeStrict(response.Receipt.Result, &config); err != nil {
		return Context{}, err
	}
	return config, nil
}
func (e *StdioEndpoint) Admit(ctx context.Context, b Binding) error {
	_, err := e.control(ctx, b, "supervisor_admit")
	return err
}
func (e *StdioEndpoint) Cancel(ctx context.Context, b Binding) error {
	_, err := e.control(ctx, b, "supervisor_cancel")
	return err
}

func (e *StdioEndpoint) InspectReceipt(ctx context.Context, original Request) (Response, error) {
	if original.Token != "" {
		return Response{}, ErrAuthority
	}
	arguments, err := json.Marshal(original)
	if err != nil {
		return Response{}, err
	}
	request := Request{Version: ProtocolVersion, ID: original.ID, OperationID: original.OperationID, Binding: original.Binding, Operation: "supervisor_receipt", Arguments: arguments}
	return e.Call(ctx, request)
}
func (e *StdioEndpoint) Close() error {
	e.closeOnce.Do(func() {
		_ = e.input.Close()
		// EOF asks the worker to revoke dispatch, stop its independently grouped
		// command children and settle their receipts before exiting. Killing the
		// worker immediately can orphan those groups on Unix.
		grace := time.NewTimer(time.Second)
		defer grace.Stop()
		select {
		case <-e.done:
			_ = e.output.Close()
			e.closeErr = <-e.cleanupResult
			return
		case <-grace.C:
		}
		signalErr := e.stop()
		_ = e.output.Close()
		select {
		case <-e.done:
			e.closeErr = errors.Join(ErrHostUnavailable, signalErr, <-e.cleanupResult)
		case <-time.After(time.Second):
			e.closeErr = errors.Join(ErrHostUnavailable, signalErr)
		}
	})
	settled, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return errors.Join(e.closeErr, e.finishProtection(settled))
}
