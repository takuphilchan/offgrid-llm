package execution

import "golang.org/x/sys/unix"

func observeProcessExit(pid int) error {
	queue, err := unix.Kqueue()
	if err != nil {
		return err
	}
	defer unix.Close(queue)
	change := []unix.Kevent_t{{Ident: uint64(pid), Filter: unix.EVFILT_PROC, Flags: unix.EV_ADD | unix.EV_ONESHOT, Fflags: unix.NOTE_EXIT}}
	events := make([]unix.Kevent_t, 1)
	for {
		n, err := unix.Kevent(queue, change, events, nil)
		if err == unix.EINTR {
			change = nil
			continue
		}
		if err != nil {
			return err
		}
		if n > 0 && events[0].Flags&unix.EV_ERROR != 0 {
			return unix.Errno(events[0].Data)
		}
		if n > 0 {
			return nil
		}
		change = nil
	}
}
