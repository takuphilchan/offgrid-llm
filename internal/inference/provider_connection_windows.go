package inference

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var providerGetTCPTable = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")
var providerIsProcessInJob = windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")

func providerConnectionPID(local, remote *net.TCPAddr) (uint32, error) {
	if local == nil || remote == nil || local.IP.To4() == nil || remote.IP.To4() == nil {
		return 0, errors.New("provider connection is not IPv4 TCP")
	}
	// OWNER_PID_CONNECTIONS (4), AF_INET (2). Keep both allocation and retries
	// bounded when other applications continuously add system TCP connections.
	var size uint32
	status, _, _ := providerGetTCPTable.Call(0, uintptr(unsafe.Pointer(&size)), 0, 2, 4, 0)
	if status != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
		return 0, errors.New("provider TCP owner table unavailable")
	}
	for range 3 {
		if size < 4 || size > 4<<20 {
			return 0, errors.New("provider TCP owner table exceeds bounds")
		}
		buffer := make([]byte, size)
		status, _, _ = providerGetTCPTable.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), 0, 2, 4, 0)
		if status == uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
			continue
		}
		if status != 0 || size > uint32(len(buffer)) {
			return 0, errors.New("provider TCP owner query failed")
		}
		count := binary.LittleEndian.Uint32(buffer[:4])
		if uint64(count)*24+4 > uint64(size) {
			return 0, errors.New("provider TCP owner table is malformed")
		}
		var found uint32
		for i := uint32(0); i < count; i++ {
			row := buffer[4+i*24 : 4+(i+1)*24]
			if binary.LittleEndian.Uint32(row[:4]) != 5 || !net.IP(row[4:8]).Equal(local.IP) || int(binary.BigEndian.Uint16(row[8:10])) != local.Port || !net.IP(row[12:16]).Equal(remote.IP) || int(binary.BigEndian.Uint16(row[16:18])) != remote.Port {
				continue
			}
			pid := binary.LittleEndian.Uint32(row[20:24])
			if pid == 0 || found != 0 && found != pid {
				return 0, errors.New("provider TCP ownership is ambiguous")
			}
			found = pid
		}
		return found, nil
	}
	return 0, errors.New("provider TCP owner table changed repeatedly")
}

func (job *providerProcessJob) verifyConnection(ctx context.Context, conn net.Conn) error {
	server, ok := conn.RemoteAddr().(*net.TCPAddr)
	client, clientOK := conn.LocalAddr().(*net.TCPAddr)
	if !ok || !clientOK {
		return errors.New("provider peer identity is unavailable")
	}
	job.mu.RLock()
	defer job.mu.RUnlock()
	if job.closed {
		return errors.New("provider process ownership is closed")
	}
	deadline := time.NewTimer(250 * time.Millisecond)
	defer deadline.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		pid, err := providerConnectionPID(server, client)
		if err != nil {
			return err
		}
		if pid != 0 {
			process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, pid)
			if err != nil {
				return err
			}
			defer windows.CloseHandle(process)
			var belongs int32
			result, _, callErr := providerIsProcessInJob.Call(uintptr(process), uintptr(job.handle), uintptr(unsafe.Pointer(&belongs)))
			if result == 0 {
				return callErr
			}
			// The process handle retains identity while the TCP table is sampled
			// again; a recycled PID cannot turn a different peer into this job.
			current, err := providerConnectionPID(server, client)
			status, waitErr := windows.WaitForSingleObject(process, 0)
			if err != nil || waitErr != nil || current != pid || belongs == 0 || status != uint32(windows.WAIT_TIMEOUT) {
				return errors.New("provider connection does not belong to the live owned runtime")
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("provider established connection has no provable owner")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
