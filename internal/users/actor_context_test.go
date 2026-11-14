package users

import (
	"context"
	"testing"
)

func TestDurableActorContextSnapshotsCurrentRoleWithoutCredentials(t *testing.T) {
	store := NewUserStore(t.TempDir())
	actor, _, err := store.CreateUser("durable", "fixture-password", RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	ctx, ok := store.ContextForActor(context.Background(), actor.ID, PermissionChat)
	if !ok {
		t.Fatal("authorized actor missing")
	}
	identity := UserFromContext(ctx)
	if identity.ID != actor.ID || identity.Role != RoleAdmin || identity.PasswordHash != "" || identity.APIKey != "" || identity.APIKeyHash != "" || identity.Metadata != nil {
		t.Fatal("invalid identity snapshot")
	}
	if err = store.UpdateUser(actor.ID, map[string]any{"role": RoleGuest}); err != nil {
		t.Fatal(err)
	}
	if identity.Role != RoleAdmin || store.ActorHasPermission(actor.ID, PermissionAdmin) {
		t.Fatal("mutable role escaped its lock or dispatch missed revocation")
	}
	fresh, ok := store.ContextForActor(context.Background(), actor.ID, PermissionChat)
	if !ok || UserFromContext(fresh).Role != RoleGuest {
		t.Fatal("preparation reused stale role")
	}
	if _, ok = store.ContextForActor(context.Background(), actor.ID, PermissionAdmin); ok {
		t.Fatal("preparation ignored demotion")
	}
	if err = store.UpdateUser(actor.ID, map[string]any{"disabled": true}); err != nil {
		t.Fatal(err)
	}
	if _, ok = store.ContextForActor(context.Background(), actor.ID, PermissionChat); ok {
		t.Fatal("disabled actor admitted")
	}
}
