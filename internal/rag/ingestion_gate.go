package rag

import (
	"context"
	"sync"
)

// Serializes index writes without trapping cancelled requests behind another
// import or reindex. The zero value is usable by legacy Engine constructors.
type ingestionGate struct {
	once sync.Once
	slot chan struct{}
}

func (g *ingestionGate) acquire(ctx context.Context) error {
	g.once.Do(func() { g.slot = make(chan struct{}, 1) })
	select {
	case g.slot <- struct{}{}:
		if err := ctx.Err(); err != nil {
			g.release()
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *ingestionGate) release() { <-g.slot }
