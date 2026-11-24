package execution

import (
	"context"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"runtime"
)

func readProtectionGrant(path string) (Context, error) {
	var config Context
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxFrameBytes || CheckPrivatePath(path) != nil {
		return config, ErrAuthority
	}
	file, err := os.Open(path)
	if err != nil {
		return config, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxFrameBytes+1))
	if err != nil || len(data) > MaxFrameBytes || decodeStrict(data, &config) != nil || config.Version != ProtocolVersion || !validBackend(config) || !protectionBindingValid(config.Binding) {
		return Context{}, ErrAuthority
	}
	return config, nil
}

func (e *StdioEndpoint) acceptProtectionStartup(ctx context.Context, data json.RawMessage, config Context, directory string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var startup protectionStartup
	if len(data) > 4096 || decodeStrict(data, &startup) != nil || startup.Protocol != ProtocolVersion || startup.Revision != PackRevision {
		return ErrProtocol
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		return ErrProtocol
	}
	_, present := fields["host_protection"]
	if present && startup.Protection == nil {
		return ErrProtocol
	}
	if present {
		if !validIdentity(startup.Version) || startup.Platform != runtime.GOOS || startup.Architecture != runtime.GOARCH {
			return ErrProtocol
		}
		registry, err := AccountHostProtectionRegistry()
		if err != nil {
			return err
		}
		owner, err := registry.acceptProtectionHandoff(ctx, startup.Protection, config, directory)
		if err != nil {
			return err
		}
		e.protectionMu.Lock()
		e.protection = owner
		e.protectionMu.Unlock()
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = WriteFrame(e.input, protectionStartupAck{Version: 1, Kind: "host_protection_ack", Lease: startup.Protection.Lease, Token: e.token}); err != nil {
			return err
		}
	}
	e.identity = startup.WorkerIdentity
	return nil
}

type protectionStartupAck struct {
	Version int    `json:"version"`
	Kind    string `json:"kind"`
	Lease   string `json:"lease"`
	Token   string `json:"token"`
}

func acceptProtectionAcknowledgment(reader io.Reader, lease, token string) error {
	var ack protectionStartupAck
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return ErrAuthority
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > 1024 {
		return ErrProtocol
	}
	data := make([]byte, int(size))
	if _, err := io.ReadFull(reader, data); err != nil {
		return ErrProtocol
	}
	if decodeStrict(data, &ack) != nil || ack.Version != 1 || ack.Kind != "host_protection_ack" || ack.Lease != lease ||
		!validCredential(ack.Token) || subtle.ConstantTimeCompare([]byte(ack.Token), []byte(token)) != 1 {
		return ErrAuthority
	}
	return nil
}
