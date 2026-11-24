package processowner

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// clone established ownership before executable code ran. A live namespace
// descriptor supports additional connection membership checks. Short commands
// need no procfs discovery: their retained init pidfd still proves lifetime.
func (job *Namespace) captureIdentity() error {
	if exited, err := pidfdExited(job.initFD); err != nil || exited {
		return err
	}
	base := fmt.Sprintf("/proc/%d", job.cmd.Process.Pid)
	namespace, err := os.Open(base + "/ns/pid")
	if err != nil {
		return job.identityOrExit(err)
	}
	accepted := false
	defer func() {
		if !accepted {
			namespace.Close()
		}
	}()
	expected, err := namespace.Stat()
	if err != nil {
		return job.identityOrExit(err)
	}
	self, err := os.Stat("/proc/self/ns/pid")
	if err != nil {
		return err
	}
	if os.SameFile(expected, self) {
		return errors.New("owned process did not enter a separate PID namespace")
	}
	status, err := readProc(base + "/status")
	if err != nil {
		return job.identityOrExit(err)
	}
	valid := false
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "NSpid:") {
			ids := strings.Fields(strings.TrimPrefix(line, "NSpid:"))
			valid = len(ids) > 1 && ids[0] == strconv.Itoa(job.cmd.Process.Pid) && ids[len(ids)-1] == "1"
			break
		}
	}
	if !valid {
		return job.identityOrExit(errors.New("owned process is not its namespace init"))
	}
	for _, entry := range []struct {
		name string
		host int
	}{{"uid_map", os.Geteuid()}, {"gid_map", os.Getegid()}} {
		raw, err := readProc(base + "/" + entry.name)
		if err != nil {
			return job.identityOrExit(err)
		}
		fields := strings.Fields(string(raw))
		if len(fields) != 3 || fields[0] != "0" || fields[1] != strconv.Itoa(entry.host) || fields[2] != "1" {
			return job.identityOrExit(errors.New("owned process user namespace mapping differs from current account"))
		}
	}
	if exited, err := pidfdExited(job.initFD); err != nil || exited {
		return err
	}
	// The pidfd must still refer to a live process after procfs discovery.
	current, err := os.Stat(base + "/ns/pid")
	if err != nil {
		return job.identityOrExit(err)
	}
	if !os.SameFile(expected, current) {
		return errors.New("owned PID namespace changed")
	}
	job.namespace = namespace
	accepted = true
	return nil
}

func (job *Namespace) identityOrExit(cause error) error {
	exited, err := pidfdExited(job.initFD)
	if err != nil {
		return errors.Join(cause, err)
	}
	if exited {
		return nil
	}
	return cause
}
