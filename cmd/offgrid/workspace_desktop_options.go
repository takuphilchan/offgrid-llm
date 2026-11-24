package main

import (
	"context"
	"encoding/hex"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"filippo.io/age"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

type desktopMaintenanceRequest struct {
	Protocol    int    `json:"protocol"`
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	Operation   string `json:"operation"`
	Encryption  string `json:"encryption"`
	DataDir     string `json:"data_dir,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Passphrase  string `json:"passphrase,omitempty"`
	KeyMaterial string `json:"key_material,omitempty"`
}

func validMaintenanceID(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && value == strings.ToLower(value)
}

func maintenancePath(value string) bool {
	return len(value) <= 4096 && utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n") && filepath.IsAbs(value)
}

func (request *desktopMaintenanceRequest) options(ctx context.Context) (storage.BackupWriteOptions, storage.BackupReadOptions, error) {
	var write storage.BackupWriteOptions
	var read storage.BackupReadOptions
	invalid := func() (storage.BackupWriteOptions, storage.BackupReadOptions, error) {
		return write, read, errDesktopProtocol
	}
	if ctx.Err() != nil {
		return write, read, ctx.Err()
	}
	if request.Protocol != 1 || request.Kind != "start" || !validMaintenanceID(request.ID) {
		return invalid()
	}
	switch request.Operation {
	case "backup":
		if !maintenancePath(request.DataDir) || !maintenancePath(request.Destination) || request.Source != "" {
			return invalid()
		}
	case "verify":
		if !maintenancePath(request.Source) || request.DataDir != "" || request.Destination != "" {
			return invalid()
		}
	case "restore":
		if !maintenancePath(request.Source) || !maintenancePath(request.Destination) || request.DataDir != "" {
			return invalid()
		}
	default:
		return invalid()
	}
	if request.Encryption == "legacy" {
		if request.Passphrase != "" || request.KeyMaterial != "" {
			return invalid()
		}
		return write, read, nil
	}
	if request.Encryption == "passphrase" {
		secret := []byte(request.Passphrase)
		defer clear(secret)
		if request.KeyMaterial != "" || validateWorkspaceSecret(secret) != nil {
			return invalid()
		}
		if request.Operation == "backup" {
			recipient, err := age.NewScryptRecipient(request.Passphrase)
			if err != nil {
				return invalid()
			}
			recipient.SetWorkFactor(storage.BackupScryptWorkFactor)
			write.Recipients = []age.Recipient{recipient}
		} else {
			identity, err := age.NewScryptIdentity(request.Passphrase)
			if err != nil {
				return invalid()
			}
			identity.SetMaxWorkFactor(storage.BackupScryptWorkFactor)
			read = storage.BackupReadOptions{Identities: []age.Identity{identity}, RequireEncryption: true}
		}
		return write, read, nil
	}
	if request.Encryption != "recipient" || request.Passphrase != "" || len(request.KeyMaterial) == 0 || len(request.KeyMaterial) > maxWorkspaceKeyFile || !utf8.ValidString(request.KeyMaterial) {
		return invalid()
	}
	var err error
	if request.Operation == "backup" {
		write.Recipients, err = age.ParseRecipients(strings.NewReader(request.KeyMaterial))
		if err != nil || len(write.Recipients) == 0 || len(write.Recipients) > storage.MaxBackupKeys {
			return invalid()
		}
	} else {
		read.Identities, err = age.ParseIdentities(strings.NewReader(request.KeyMaterial))
		if err != nil || len(read.Identities) == 0 || len(read.Identities) > storage.MaxBackupKeys {
			return invalid()
		}
		read.RequireEncryption = true
	}
	return write, read, nil
}
