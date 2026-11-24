package execution

import "os/exec"

type ownedProcessStarter func(*exec.Cmd) (func() error, func() error, func() error, error)

// OpenSupervisedSession is exclusively for the pinned stdio worker. Runtime
// pins arrive from its native parent, separately from the serialized grant and
// all HTTP/model input. They authorize no additional task operations.
func OpenSupervisedSession(config Context, privateRuntime string) (*Session, error) {
	start, err := supervisedProcessStarter(privateRuntime)
	if err != nil {
		return nil, err
	}
	session, err := OpenSession(config)
	if err != nil {
		return nil, err
	}
	session.startProcess = start
	return session, nil
}
