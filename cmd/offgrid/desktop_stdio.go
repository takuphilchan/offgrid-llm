package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

var errDesktopProtocol = errors.New("invalid private desktop protocol")

func desktopReadFrame(reader io.Reader, limit uint32, target any) error {
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > limit {
		return errDesktopProtocol
	}
	body := make([]byte, size)
	defer clear(body)
	if _, err := io.ReadFull(reader, body); err != nil {
		return err
	}
	if jsonwire.Validate(body) != nil {
		return errDesktopProtocol
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		return errDesktopProtocol
	}
	return nil
}

func desktopWriteFrame(writer io.Writer, value any) error {
	body, err := json.Marshal(value)
	if err != nil || len(body) > 32768 {
		return errDesktopProtocol
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(body)))
	if _, err := io.Copy(writer, bytes.NewReader(header[:])); err != nil {
		return err
	}
	_, err = io.Copy(writer, bytes.NewReader(body))
	return err
}

func desktopPrivateInput(ctx context.Context, input io.ReadCloser) io.Reader {
	if file, ok := input.(*os.File); ok {
		return &workspaceSecretFileReader{ctx: ctx, file: file}
	}
	return input
}

// OS handles already use bounded context-aware polling. Closing one while its
// reader obtains Fd races with descriptor destruction/reuse; cancel and join
// that reader before Close. Generic blocking readers need Close to wake up.
func interruptDesktopInput(input io.ReadCloser) {
	if _, polled := input.(*os.File); !polled {
		_ = input.Close()
	}
}

// Explicit private parent lifetime. EOF requests cancellation; existing
// app.Runtime.Run remains the sole application drain and cleanup owner.
func serveDesktopOwned(ctx context.Context, input io.ReadCloser, run func(context.Context) int) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer input.Close()
	stopInput := context.AfterFunc(ctx, func() { interruptDesktopInput(input) })
	defer stopInput()
	observed := make(chan error, 1)
	go func() {
		var extra [1]byte
		n, err := desktopPrivateInput(ctx, input).Read(extra[:])
		if n != 0 || err != io.EOF && ctx.Err() == nil {
			err = errDesktopProtocol
		}
		observed <- err
		cancel()
	}()
	code := run(ctx)
	cancel()
	interruptDesktopInput(input)
	if err := <-observed; errors.Is(err, errDesktopProtocol) && code == 0 {
		return 2
	}
	return code
}

func readDesktopMaintenanceStart(ctx context.Context, input io.ReadCloser, target any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	stopInput := context.AfterFunc(ctx, func() { interruptDesktopInput(input) })
	defer stopInput()
	return desktopReadFrame(desktopPrivateInput(ctx, input), 2<<20, target)
}
