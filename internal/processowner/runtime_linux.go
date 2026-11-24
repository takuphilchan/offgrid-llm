package processowner

import "context"

// Runtime contains executable identities, not authorization. Only a native
// supervisor may select these pins; never derive them from remote task input.
type Runtime struct {
	Launcher Launcher
	Shell    Launcher
}

func SystemRuntime(ctx context.Context) (Runtime, error) {
	launcher, err := SystemLauncher(ctx)
	if err != nil {
		return Runtime{}, err
	}
	shell, err := systemShell(ctx)
	if err != nil {
		return Runtime{}, err
	}
	return Runtime{Launcher: launcher, Shell: shell}, nil
}

// Verify checks the exact bytes again without interpreting namespace-relative
// owner IDs. StartRuntime repeats this check on the descriptors used for exec.
func (r Runtime) Verify(ctx context.Context) error {
	for _, pin := range []Launcher{r.Launcher, r.Shell} {
		file, err := openLauncher(ctx, pin)
		if err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}
