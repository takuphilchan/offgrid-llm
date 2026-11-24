package storage

import "context"

// Private fault boundaries let package tests stop/crash an actual filesystem
// operation deterministically. They are not accepted through any client API.
type backupBoundaryKey struct{}
type backupBoundaryHook func(stage, path string) error

func backupBoundary(ctx context.Context, stage, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if hook, ok := ctx.Value(backupBoundaryKey{}).(backupBoundaryHook); ok {
		if err := hook(stage, path); err != nil {
			return err
		}
	}
	return ctx.Err()
}
