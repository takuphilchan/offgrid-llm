package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"

	"filippo.io/age"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

const maxWorkspaceKeyFile = 1 << 20

type workspaceKeyFlags struct {
	recipients, recipientFiles, identities []string
	passphrase, passphraseStdin            bool
}

func (k *workspaceKeyFlags) register(flags *flag.FlagSet, writing bool) {
	appendKey := func(destination *[]string) func(string) error {
		return func(value string) error {
			if value == "" || len(value) > 8192 || len(*destination) >= storage.MaxBackupKeys {
				return errors.New("invalid or excessive key inputs")
			}
			*destination = append(*destination, value)
			return nil
		}
	}
	if writing {
		flags.Func("recipient", "native age public recipient (repeatable)", appendKey(&k.recipients))
		flags.Func("recipients-file", "native age public recipients file (repeatable)", appendKey(&k.recipientFiles))
	} else {
		flags.Func("identity", "native age private identity file (repeatable)", appendKey(&k.identities))
	}
	flags.BoolVar(&k.passphrase, "passphrase", false, "read a passphrase from the terminal without echo")
	flags.BoolVar(&k.passphraseStdin, "passphrase-stdin", false, "read a bounded passphrase from redirected standard input")
}

func (k workspaceKeyFlags) options(ctx context.Context, streams workspaceStreams, writing bool) (storage.BackupWriteOptions, storage.BackupReadOptions, error) {
	var write storage.BackupWriteOptions
	var read storage.BackupReadOptions
	if err := ctx.Err(); err != nil {
		return write, read, err
	}
	if (k.passphrase && k.passphraseStdin) || ((k.passphrase || k.passphraseStdin) && len(k.recipients)+len(k.recipientFiles)+len(k.identities) > 0) {
		return write, read, usage("choose recipient/identity keys or one passphrase input mode")
	}
	if k.passphrase || k.passphraseStdin {
		var secret []byte
		var err error
		if k.passphraseStdin {
			secret, err = readWorkspaceStdinSecret(ctx, streams.in)
		} else {
			secret, err = streams.terminalSecret(ctx, streams.err, writing)
		}
		if err != nil {
			return write, read, err
		}
		defer clear(secret)
		if err = ctx.Err(); err != nil {
			return write, read, err
		}
		if writing {
			recipient, err := age.NewScryptRecipient(string(secret))
			if err != nil {
				return write, read, errors.New("invalid backup passphrase")
			}
			recipient.SetWorkFactor(storage.BackupScryptWorkFactor)
			write.Recipients = []age.Recipient{recipient}
		} else {
			identity, err := age.NewScryptIdentity(string(secret))
			if err != nil {
				return write, read, errors.New("invalid backup passphrase")
			}
			identity.SetMaxWorkFactor(storage.BackupScryptWorkFactor)
			read = storage.BackupReadOptions{Identities: []age.Identity{identity}, RequireEncryption: true}
		}
		return write, read, nil
	}
	for _, value := range k.recipients {
		parsed, err := age.ParseRecipients(strings.NewReader(value))
		if err != nil || len(parsed) != 1 {
			return write, read, usage("invalid native age recipient")
		}
		write.Recipients = append(write.Recipients, parsed...)
	}
	for _, filename := range k.recipientFiles {
		data, err := readWorkspaceKeyFile(filename)
		if err != nil {
			return write, read, err
		}
		parsed, parseErr := age.ParseRecipients(bytes.NewReader(data))
		clear(data)
		if parseErr != nil || len(parsed) == 0 {
			return write, read, usage("invalid native age recipients file")
		}
		write.Recipients = append(write.Recipients, parsed...)
		if len(write.Recipients) > storage.MaxBackupKeys {
			return write, read, usage("at most 32 backup recipients are supported")
		}
	}
	for _, filename := range k.identities {
		data, err := readWorkspaceKeyFile(filename)
		if err != nil {
			return write, read, err
		}
		parsed, parseErr := age.ParseIdentities(bytes.NewReader(data))
		clear(data)
		if parseErr != nil || len(parsed) == 0 {
			return write, read, usage("invalid native age identity file")
		}
		read.Identities = append(read.Identities, parsed...)
		if len(read.Identities) > storage.MaxBackupKeys {
			return write, read, usage("at most 32 backup identities are supported")
		}
	}
	read.RequireEncryption = len(read.Identities) > 0
	return write, read, ctx.Err()
}

func readWorkspaceKeyFile(filename string) ([]byte, error) {
	before, err := os.Lstat(filename)
	if err != nil || !before.Mode().IsRegular() || before.Size() > maxWorkspaceKeyFile {
		return nil, usage("key file must be a readable regular file of at most 1 MiB")
	}
	file, err := openWorkspaceKeyFile(filename, before)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxWorkspaceKeyFile+1))
	if err != nil || len(data) > maxWorkspaceKeyFile {
		clear(data)
		return nil, errors.New("could not read bounded backup key file")
	}
	return data, nil
}

func openWorkspaceKeyFile(filename string, before os.FileInfo) (*os.File, error) {
	file, err := openWorkspaceKeyHandle(filename)
	if err != nil {
		return nil, errors.New("could not open backup key file")
	}
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		file.Close()
		return nil, errors.New("backup key file changed while opening")
	}
	return file, nil
}
