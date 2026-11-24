package execution

import (
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Notifications are discovery hints, never exit proof. Retained handles and
// lifetime accounting remain authoritative if a notification is lost or late.
type executionJobWatch struct {
	mu       sync.Mutex
	job      windows.Handle
	port     windows.Handle
	members  map[uint32]windows.Handle
	fault    error
	done     chan struct{}
	barrier  chan uint32
	sequence uint32
}

var executionGetCompletion = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetQueuedCompletionStatus")

func newExecutionJobWatch(job windows.Handle, members map[uint32]windows.Handle) (*executionJobWatch, error) {
	port, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 1)
	if err != nil {
		return nil, err
	}
	association := struct{ Key, Port uintptr }{1, uintptr(port)}
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectAssociateCompletionPortInformation, uintptr(unsafe.Pointer(&association)), uint32(unsafe.Sizeof(association))); err != nil {
		windows.CloseHandle(port)
		return nil, err
	}
	w := &executionJobWatch{job: job, port: port, members: members, done: make(chan struct{}), barrier: make(chan uint32, 1)}
	go w.run()
	return w, nil
}

func (w *executionJobWatch) run() {
	defer close(w.done)
	for {
		var message uint32
		var key, value uintptr
		// Job notifications carry a numeric PID in lpOverlapped, not an actual
		// OVERLAPPED allocation. Keep it out of Go's pointer/GC representation.
		ok, _, err := executionGetCompletion.Call(uintptr(w.port), uintptr(unsafe.Pointer(&message)), uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&value)), uintptr(windows.INFINITE))
		if ok == 0 {
			w.mu.Lock()
			if w.fault == nil {
				w.fault = err
			}
			w.mu.Unlock()
			return
		}
		if key == 2 {
			select {
			case w.barrier <- message:
			default:
			}
			continue
		}
		if key == 3 {
			return
		}
		w.mu.Lock()
		if key != 1 {
			if w.fault == nil {
				w.fault = ErrHostUnavailable
			}
		} else if message == 6 { // JOB_OBJECT_MSG_NEW_PROCESS
			if value == 0 || uint64(value) > 1<<32-1 {
				if w.fault == nil {
					w.fault = ErrHostUnavailable
				}
			} else if err := captureExecutionJobMember(w.job, w.members, uint32(value)); err != nil {
				// A late PID cannot be adopted, including after it was reused.
				if w.fault == nil {
					w.fault = err
				}
			}
		}
		w.mu.Unlock()
	}
}

// Called under the owner's lifecycle lock, with no member lock held. One
// barrier at a time prevents callers consuming another operation's response.
func (w *executionJobWatch) drain() error {
	if w.sequence == ^uint32(0) {
		return ErrLimit
	}
	w.sequence++
	if err := windows.PostQueuedCompletionStatus(w.port, w.sequence, 2, nil); err != nil {
		return err
	}
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case sequence := <-w.barrier:
			if sequence == w.sequence {
				return nil
			}
		case <-w.done:
			return ErrHostUnavailable
		case <-timer.C:
			return ErrHostUnavailable
		}
	}
}

func (w *executionJobWatch) close() error {
	select {
	case <-w.done:
	default:
		if err := windows.PostQueuedCompletionStatus(w.port, 0, 3, nil); err != nil {
			return err
		}
		select {
		case <-w.done:
		case <-time.After(2 * time.Second):
			return ErrHostUnavailable
		}
	}
	return windows.CloseHandle(w.port)
}
