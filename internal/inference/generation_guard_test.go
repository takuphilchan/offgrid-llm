package inference

import (
	"context"
	"errors"
	"testing"
)

func TestGenerationGuardComposesAndHonorsCancellation(t *testing.T) {
	denied := errors.New("authority revoked")
	allow := true
	ctx := WithGenerationGuard(context.Background(), func() error {
		if !allow {
			return denied
		}
		return nil
	})
	checks := 0
	ctx = WithGenerationGuard(ctx, func() error { checks++; return nil })
	if err := CheckGenerationGuard(ctx); err != nil || checks != 1 {
		t.Fatalf("err=%v checks=%d", err, checks)
	}
	allow = false
	if err := CheckGenerationGuard(ctx); !errors.Is(err, denied) || checks != 1 {
		t.Fatalf("composed guard bypass: %v checks=%d", err, checks)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := CheckGenerationGuard(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := CheckGenerationGuard(WithGenerationGuard(context.Background(), nil)); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyGenerationGuardRejectsLateSuccess(t *testing.T) {
	denied := errors.New("authority revoked")
	revoked := false
	ctx := WithGenerationGuard(context.Background(), func() error {
		if revoked {
			return denied
		}
		return nil
	})
	releases := 0
	lease, err := NewLegacyGenerationLease(func() { releases++ }, ProviderTiming{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	err = lease.Run(ctx, func(context.Context) error { calls++; revoked = true; return nil })
	if !errors.Is(err, denied) {
		t.Fatalf("late success accepted: %v", err)
	}
	err = lease.Run(ctx, func(context.Context) error { calls++; return nil })
	if !errors.Is(err, denied) || calls != 1 {
		t.Fatalf("revoked dispatch: %v calls=%d", err, calls)
	}
	lease.Release()
	lease.Release()
	if releases != 1 {
		t.Fatalf("releases=%d", releases)
	}
}
