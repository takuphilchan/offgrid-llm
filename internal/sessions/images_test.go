package sessions

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestImageTurnRecoveryOwnershipAndBudget(t *testing.T) {
	directory := t.TempDir()
	manager := NewSessionManager(directory)
	session := NewSession("images", "vision")
	session.OwnerID = "alice"
	if err := manager.Save(session); err != nil {
		t.Fatal(err)
	}
	owner := manager.WithAccess(Access{UserID: "alice"})
	images := []api.SessionImage{{Name: "synthetic.png", DataURL: "synthetic-owned-storage-fixture"}}
	turn := &Turn{ID: "one", RequestHash: "hash-one", Model: "vision", Images: images, Status: "running", Output: "partial"}
	if _, _, err := owner.BeginTurn("images", turn); err != nil {
		t.Fatal(err)
	}
	restarted := NewSessionManager(directory)
	if err := restarted.RecoverTurns(); err != nil {
		t.Fatal(err)
	}
	loaded, err := restarted.WithAccess(Access{UserID: "alice"}).Load("images")
	if err != nil || loaded.Turn.Status != "interrupted" || len(loaded.Messages) != 0 || !reflect.DeepEqual(loaded.Turn.Images, images) {
		t.Fatal("lost interrupted input", err)
	}
	if _, err := restarted.WithAccess(Access{UserID: "bob"}).Load("images"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatal("cross-user image access", err)
	}
	large := &Turn{ID: "two", RequestHash: "two", Images: []api.SessionImage{{Name: "large", DataURL: strings.Repeat("x", api.MaxSessionImageRequestBytes+1)}}}
	if _, _, err := owner.BeginTurn("images", large); !errors.Is(err, ErrImageCapacity) {
		t.Fatal("unbounded image admission", err)
	}
	unchanged, _ := owner.Load("images")
	if unchanged.Turn.ID != "one" {
		t.Fatal("rejected turn replaced original")
	}
}
