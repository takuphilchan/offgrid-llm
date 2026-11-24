// Package processowner owns native process lifetimes, not task execution or
// authorization. Callers must supply an approved command and complete environment.
package processowner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var ErrStopUncertain = errors.New("native namespace stop uncertain")
var ErrStarted = errors.New("native namespace command may have started")

type Launcher struct{ Path, SHA256 string }

// Namespace owns the exact child created as PID 1. Its pidfd is returned by
// clone, so even an immediately exiting command cannot outrun owner discovery.
// This boundary contains descendants' lifetimes, not filesystem/network access.
type Namespace struct {
	mu                         sync.RWMutex
	closed                     bool
	cmd                        *exec.Cmd
	launcher, shell, namespace *os.File
	initFD                     int
	done                       chan struct{}
	waitErr                    error
	once                       sync.Once
	err                        error
}

var uncertainNamespaces = struct {
	sync.Mutex
	jobs map[*Namespace]struct{}
}{jobs: map[*Namespace]struct{}{}}

// The activation pipe also closes the pre-PDEATHSIG parent-loss window. Go's
// child-side numeric parent check cannot distinguish an external parent from
// PID 0 inside this namespace. Until Start confirms ownership, EOF prevents the
// gate from executing the approved command, even if parent-death was not armed.
const activationToken = "offgrid-namespace-start-v1\n"
const activationScript = "IFS= read -r offgrid_start <&4 && [ \"$offgrid_start\" = \"offgrid-namespace-start-v1\" ] || exit 125; exec 4<&-; exec \"$@\""

func Start(ctx context.Context, cmd *exec.Cmd, trusted Launcher) (*Namespace, error) {
	shell, err := systemShell(ctx)
	if err != nil {
		return nil, err
	}
	return StartRuntime(ctx, cmd, Runtime{Launcher: trusted, Shell: shell})
}

// StartRuntime uses pins captured by a trusted supervisor before entering a
// user namespace. Unmapped file ownership inside it is not a new trust source.
func StartRuntime(ctx context.Context, cmd *exec.Cmd, trusted Runtime) (*Namespace, error) {
	job, activation, err := prepareNamespaceRuntime(ctx, cmd, trusted, syscall.SIGKILL)
	if err != nil {
		return nil, err
	}
	defer activation.Close()
	if err = ctx.Err(); err == nil {
		var written int
		written, err = activation.Write([]byte(activationToken))
		if err == nil && written != len(activationToken) {
			err = errors.New("namespace activation was incomplete")
		}
	}
	if err != nil {
		return nil, errors.Join(ErrStarted, err, job.Close())
	}
	return job, nil
}

func prepareNamespace(ctx context.Context, cmd *exec.Cmd, trusted Launcher, parentDeath syscall.Signal) (*Namespace, *os.File, error) {
	shell, err := systemShell(ctx)
	if err != nil {
		return nil, nil, err
	}
	return prepareNamespaceRuntime(ctx, cmd, Runtime{Launcher: trusted, Shell: shell}, parentDeath)
}

func prepareNamespaceRuntime(ctx context.Context, cmd *exec.Cmd, trusted Runtime, parentDeath syscall.Signal) (*Namespace, *os.File, error) {
	if cmd == nil || !filepath.IsAbs(cmd.Path) || len(cmd.Args) == 0 || cmd.Env == nil || cmd.SysProcAttr != nil || len(cmd.ExtraFiles) != 0 {
		return nil, nil, errors.New("owned namespace requires a complete trusted command without inherited descriptors")
	}
	if info, err := os.Stat(cmd.Path); err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return nil, nil, errors.New("owned namespace runtime executable is unavailable")
	}
	launcher, err := openLauncher(ctx, trusted.Launcher)
	if err != nil {
		return nil, nil, err
	}
	shell, err := openLauncher(ctx, trusted.Shell)
	if err != nil {
		launcher.Close()
		return nil, nil, err
	}
	reader, activation, err := os.Pipe()
	if err != nil {
		launcher.Close()
		shell.Close()
		return nil, nil, err
	}
	defer reader.Close()
	retained := false
	defer func() {
		if !retained {
			activation.Close()
		}
	}()
	if err := ctx.Err(); err != nil {
		launcher.Close()
		shell.Close()
		return nil, nil, err
	}
	job := &Namespace{cmd: cmd, launcher: launcher, shell: shell, initFD: -1, done: make(chan struct{})}
	originalPath, originalArgs := cmd.Path, append([]string(nil), cmd.Args...)
	// Fixed script, literal argv, and pinned root-owned shell: no caller value
	// becomes shell source. Only the read end of the activation pipe is inherited.
	cmd.Path = "/proc/self/fd/3"
	cmd.Args = append([]string{trusted.Launcher.Path, "--mount-proc", "--", "/proc/self/fd/5", "-p", "-c", activationScript, "offgrid-namespace-gate", originalPath}, originalArgs[1:]...)
	cmd.ExtraFiles = []*os.File{launcher, reader, shell}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:                 syscall.CLONE_NEWUSER | syscall.CLONE_NEWPID,
		UidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Geteuid(), Size: 1}},
		GidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getegid(), Size: 1}},
		GidMappingsEnableSetgroups: false, Pdeathsig: parentDeath, PidFD: &job.initFD, Setsid: true,
	}
	started := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		err := cmd.Start()
		started <- err
		if err == nil {
			job.waitErr = cmd.Wait()
		}
		close(job.done)
	}()
	if err = <-started; err != nil {
		launcher.Close()
		shell.Close()
		return nil, nil, err
	}
	if job.initFD < 0 {
		err = errors.New("owned namespace requires a kernel pidfd")
	} else if err = ctx.Err(); err == nil {
		err = job.captureIdentity()
	}
	if err != nil {
		return nil, nil, errors.Join(ErrStarted, err, job.Close())
	}
	retained = true
	return job, activation, nil
}

func (job *Namespace) Done() <-chan struct{} { return job.done }

func (job *Namespace) Wait() error {
	<-job.done
	return job.waitErr
}

func (job *Namespace) stopLocked() error {
	if job.initFD < 0 {
		return ErrStopUncertain
	}
	if err := unix.PidfdSendSignal(job.initFD, syscall.SIGKILL, nil, 0); err != nil && !errors.Is(err, unix.ESRCH) {
		return errors.Join(ErrStopUncertain, err)
	}
	return nil
}

func (job *Namespace) Stop() error {
	job.mu.RLock()
	defer job.mu.RUnlock()
	if job.closed {
		return job.err
	}
	return job.stopLocked()
}

func (job *Namespace) Close() error {
	job.once.Do(func() {
		job.mu.Lock()
		defer job.mu.Unlock()
		job.closed = true
		defer func() {
			if job.err != nil {
				uncertainNamespaces.Lock()
				uncertainNamespaces.jobs[job] = struct{}{}
				uncertainNamespaces.Unlock()
			}
		}()
		if job.err = job.stopLocked(); job.err != nil {
			// The owned os.Process can still request termination after a kernel
			// pidfd failure, but that request does not establish cleanup proof.
			_ = job.cmd.Process.Kill()
			return
		}
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		select {
		case <-job.done:
		case <-timer.C:
			job.err = errors.Join(ErrStopUncertain, errors.New("namespace init reap exceeded deadline"))
			return
		}
		exited, err := pidfdExited(job.initFD)
		var exit *exec.ExitError
		if err != nil || !exited || job.waitErr != nil && !errors.As(job.waitErr, &exit) {
			job.err = errors.Join(ErrStopUncertain, err, job.waitErr)
			return
		}
		// The exact namespace init has exited and been reaped. Linux tears
		// down its descendants before publishing that exit; no PID scan or
		// zero process-group count is used as proof.
		job.err = unix.Close(job.initFD)
		if job.namespace != nil {
			job.err = errors.Join(job.err, job.namespace.Close())
		}
		job.err = errors.Join(job.err, job.launcher.Close())
		job.err = errors.Join(job.err, job.shell.Close())
		if job.err != nil {
			job.err = errors.Join(ErrStopUncertain, job.err)
		}
	})
	return job.err
}
