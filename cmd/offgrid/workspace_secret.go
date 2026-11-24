package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"github.com/charmbracelet/x/term"
)

const maxWorkspacePassphrase = 4096

func validateWorkspaceSecret(secret []byte) error {
	if len(secret) == 0 || len(secret) > maxWorkspacePassphrase || !utf8.Valid(secret) || bytes.ContainsAny(secret, "\x00\r\n") {
		return usage("passphrase must contain 1–4096 UTF-8 bytes without NUL or embedded line endings")
	}
	return nil
}

func readWorkspaceStdinSecret(ctx context.Context, input io.Reader) ([]byte, error) {
	if file, ok := input.(*os.File); ok {
		if term.IsTerminal(file.Fd()) {
			return nil, usage("--passphrase-stdin requires redirected input; use --passphrase for a protected terminal prompt")
		}
		input = &workspaceSecretFileReader{ctx: ctx, file: file}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	secret, err := io.ReadAll(io.LimitReader(input, maxWorkspacePassphrase+3))
	if err != nil {
		clear(secret)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("could not read backup passphrase")
	}
	if ctx.Err() != nil {
		clear(secret)
		return nil, ctx.Err()
	}
	if bytes.HasSuffix(secret, []byte("\r\n")) {
		secret = secret[:len(secret)-2]
	} else if bytes.HasSuffix(secret, []byte("\n")) {
		secret = secret[:len(secret)-1]
	}
	if err = validateWorkspaceSecret(secret); err != nil {
		clear(secret)
		return nil, err
	}
	return secret, nil
}

type workspaceSecretFileReader struct {
	ctx  context.Context
	file *os.File
}

func (r *workspaceSecretFileReader) Read(p []byte) (int, error) {
	return readWorkspaceSecretFile(r.ctx, r.file, p)
}

func readWorkspaceTerminalSecret(ctx context.Context, input *os.File, output io.Writer, confirm bool) (secret []byte, resultErr error) {
	if !term.IsTerminal(input.Fd()) {
		return nil, usage("--passphrase requires a terminal; use --passphrase-stdin with redirected input")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	state, err := term.MakeRaw(input.Fd())
	if err != nil {
		return nil, errors.New("could not protect terminal passphrase input")
	}
	defer func() {
		// Discard unread paste/typeahead while echo is still disabled. Otherwise
		// an oversized or cancelled secret can become the next shell command.
		if err := flushWorkspaceTerminalInput(input); err != nil {
			clear(secret)
			secret = nil
			resultErr = errors.Join(resultErr, errors.New("could not discard pending protected terminal input"))
		}
		if err := term.Restore(input.Fd(), state); err != nil {
			clear(secret)
			secret = nil
			resultErr = errors.Join(resultErr, errors.New("could not restore terminal input state"))
		}
	}()
	next := workspaceTerminalInput(ctx, input)
	read := func(prompt string) ([]byte, error) {
		if _, err := fmt.Fprint(output, prompt); err != nil {
			return nil, errors.New("could not display passphrase prompt")
		}
		defer fmt.Fprint(output, "\r\n")
		return readWorkspaceSecretLine(ctx, next)
	}
	secret, err = read("Backup passphrase: ")
	if err != nil {
		return nil, err
	}
	if confirm {
		second, err := read("Confirm backup passphrase: ")
		defer clear(second)
		if err != nil || !bytes.Equal(secret, second) {
			clear(secret)
			if err != nil {
				return nil, err
			}
			return nil, usage("backup passphrases did not match")
		}
	}
	return secret, nil
}

// The provider reads at most one UTF-8 character (or byte on Unix) at a time.
// The fixed-capacity buffer is checked before appending; input never grows first.
func readWorkspaceSecretLine(ctx context.Context, next func() ([]byte, error)) (_ []byte, resultErr error) {
	secret := make([]byte, 0, maxWorkspacePassphrase)
	defer func() {
		if resultErr != nil {
			clear(secret)
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := next()
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, errors.New("could not read protected passphrase input")
		}
		for _, c := range data {
			switch c {
			case 3, 4, 27:
				return nil, context.Canceled
			case '\r', '\n':
				if err := validateWorkspaceSecret(secret); err != nil {
					return nil, err
				}
				return secret, nil
			case 8, 127:
				if len(secret) > 0 {
					_, size := utf8.DecodeLastRune(secret)
					clear(secret[len(secret)-size:])
					secret = secret[:len(secret)-size]
				}
			default:
				if c == 0 || len(secret) == maxWorkspacePassphrase {
					return nil, usage("passphrase input is invalid or exceeds 4096 UTF-8 bytes")
				}
				secret = append(secret, c)
			}
		}
	}
}
