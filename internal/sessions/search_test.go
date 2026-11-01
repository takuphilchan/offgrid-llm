package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSearchNamesOwnershipBoundsAndNoMessageSearch(t *testing.T) {
	manager := NewSessionManager(t.TempDir())
	for i := 0; i < 45; i++ {
		session := NewSession(fmt.Sprintf("Demo café %02d", i), "model")
		session.OwnerID = "alice"
		if i%2 == 0 {
			session.OwnerID = "bob"
		}
		session.UpdatedAt = time.Unix(int64(i), 0)
		session.Messages = []Message{{Role: "user", Content: "private-message-marker"}}
		if err := manager.Save(session); err != nil {
			t.Fatal(err)
		}
	}
	scoped := manager.WithAccess(Access{UserID: "alice"})
	found, partial, err := scoped.SearchNames(context.Background(), "CAFÉ", 20)
	if err != nil || !partial || len(found) != 20 {
		t.Fatalf("%d %v %v", len(found), partial, err)
	}
	for _, result := range found {
		session, _ := manager.Load(result.ID)
		if session.OwnerID != "alice" {
			t.Fatal("cross-user result")
		}
	}
	encoded, _ := json.Marshal(found)
	if strings.Contains(string(encoded), "private-message-marker") || strings.Contains(string(encoded), "alice") {
		t.Fatal("private metadata escaped")
	}
	found, partial, err = scoped.SearchNames(context.Background(), "private-message-marker", 20)
	if err != nil || partial || len(found) != 0 {
		t.Fatal("searched messages despite declared scope")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = scoped.SearchNames(ctx, "demo", 20); err != context.Canceled {
		t.Fatal(err)
	}
	if _, _, err = manager.WithAccess(Access{}).SearchNames(context.Background(), "demo", 20); err != ErrAccessDenied {
		t.Fatal(err)
	}
}

func TestSearchNamesReportsUnreadableAndOversizedHistory(t *testing.T) {
	dir := t.TempDir()
	manager := NewSessionManager(dir)
	for name, data := range map[string]string{"demo-broken.json": "{", "demo-large.json": strings.Repeat(" ", 4<<20+1)} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	found, partial, err := manager.WithAccess(Access{All: true}).SearchNames(context.Background(), "demo", 20)
	if err != nil || !partial || len(found) != 0 {
		t.Fatalf("%v %v %v", found, partial, err)
	}
}

func TestSearchNamesExactReadBudgetStillReportsPartial(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("demo-%d", i)
		data, _ := json.Marshal(NewSession(name, "model"))
		data = append(data, []byte(strings.Repeat(" ", (4<<20)-len(data)))...)
		if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	found, partial, err := NewSessionManager(dir).WithAccess(Access{All: true}).SearchNames(context.Background(), "demo", 20)
	if err != nil || !partial || len(found) != 8 {
		t.Fatalf("%d %v %v", len(found), partial, err)
	}
}
