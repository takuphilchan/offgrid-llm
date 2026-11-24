package processowner

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func linuxTCPTuple(address *net.TCPAddr) (string, error) {
	if address == nil || address.IP.To4() == nil || !address.IP.IsLoopback() {
		return "", errors.New("owned provider requires an IPv4 loopback peer")
	}
	return fmt.Sprintf("%08X:%04X", binary.LittleEndian.Uint32(address.IP.To4()), address.Port), nil
}

func linuxSocketInode(raw []byte, server, client string, uid int) (string, error) {
	var inode string
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 || fields[1] != server || fields[2] != client || fields[3] != "01" {
			continue
		}
		value, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil || value == 0 || fields[7] != strconv.Itoa(uid) || inode != "" {
			return "", errors.New("provider socket identity is unavailable or ambiguous")
		}
		inode = fields[9]
	}
	return inode, nil
}

func linuxConnectionInode(server, client string) (string, error) {
	raw, err := readProc("/proc/net/tcp")
	if err != nil {
		return "", err
	}
	return linuxSocketInode(raw, server, client, os.Geteuid())
}

func procEntries(path string, limit int) ([]os.DirEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	entries, err := file.ReadDir(limit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > limit {
		return nil, errors.New("provider process identity inventory exceeds bounds")
	}
	return entries, nil
}

func (job *Namespace) socketOwner(ctx context.Context, inode string) (int, error) {
	expected, err := job.namespace.Stat()
	if err != nil {
		return -1, err
	}
	processes, err := procEntries("/proc", 4096)
	if err != nil {
		return -1, err
	}
	for _, process := range processes {
		if err := ctx.Err(); err != nil {
			return -1, err
		}
		pid, err := strconv.Atoi(process.Name())
		if err != nil || pid < 1 {
			continue
		}
		base := "/proc/" + process.Name()
		namespace, err := os.Stat(base + "/ns/pid")
		if err != nil || !os.SameFile(expected, namespace) {
			continue
		}
		fd, err := unix.PidfdOpen(pid, 0)
		if err != nil {
			continue
		}
		owned := false
		check := func() error {
			defer func() {
				if !owned {
					unix.Close(fd)
				}
			}()
			entries, err := procEntries(base+"/fd", 1024)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err := ctx.Err(); err != nil {
					return err
				}
				target, err := os.Readlink(base + "/fd/" + entry.Name())
				if err != nil || target != "socket:["+inode+"]" {
					continue
				}
				current, err := os.Stat(base + "/ns/pid")
				exited, exitErr := pidfdExited(fd)
				if err != nil || exitErr != nil || exited || !os.SameFile(expected, current) {
					return errors.New("provider socket owner changed")
				}
				owned = true
				return nil
			}
			return nil
		}
		if err := check(); err != nil {
			return -1, err
		}
		if owned {
			return fd, nil
		}
	}
	return -1, nil
}

func (job *Namespace) VerifyConnection(ctx context.Context, conn net.Conn) error {
	serverAddress, _ := conn.RemoteAddr().(*net.TCPAddr)
	clientAddress, _ := conn.LocalAddr().(*net.TCPAddr)
	server, err := linuxTCPTuple(serverAddress)
	if err != nil {
		return err
	}
	client, err := linuxTCPTuple(clientAddress)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	job.mu.RLock()
	defer job.mu.RUnlock()
	if job.closed || job.namespace == nil {
		return errors.New("provider namespace ownership is closed")
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		inode, err := linuxConnectionInode(server, client)
		if err != nil {
			return err
		}
		if inode != "" {
			fd, err := job.socketOwner(ctx, inode)
			if err != nil {
				return err
			}
			if fd >= 0 {
				defer unix.Close(fd)
				current, err := linuxConnectionInode(server, client)
				exited, exitErr := pidfdExited(fd)
				if err != nil || exitErr != nil || exited || current != inode {
					return errors.New("provider established connection changed owner")
				}
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return errors.New("provider connection does not belong to the owned namespace")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
